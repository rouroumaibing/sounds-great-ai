package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SourceAuthorizer verifies that a memory item's typed provenance (SourceRef)
// actually references content the operator is allowed to see. This is the
// SG homologue of the cross-thread source authorization
// (PersonMemorySourceBundleResolver): a capture is only materialized when its
// source thread/message is confirmed accessible. Unverifiable sources are
// rejected (fail-closed), zero writes.
type SourceAuthorizer interface {
	AuthorizeSource(ctx context.Context, operatorID string, ref SourceRef) (bool, error)
}

// AllowAllAuthorizer is the default no-op authorizer (single-operator,
// owner-private store where every thread belongs to the operator). It is used
// when no threadstore is wired in.
type AllowAllAuthorizer struct{}

// AuthorizeSource always allows.
func (AllowAllAuthorizer) AuthorizeSource(_ context.Context, _ string, _ SourceRef) (bool, error) {
	return true, nil
}

// PeopleMemoryClerkBatchLimit mirrors AC-A22 (daily batch of 8).
const PeopleMemoryClerkBatchLimit = 8

// PeopleMemoryClerkCronSpec mirrors the DeferredPersonMemoryDailyTaskSpec
// cron expression "30 4 * * *" — i.e. daily at 04:30 local time. The SG clerk is
// in-process (no external cron daemon dependency); it computes the next 04:30
// tick and sleeps until then so it is aligned to the same wall-clock time as
// the reference schedule and does NOT fire immediately on process restart.
const PeopleMemoryClerkCronSpec = "30 4 * * *"

const (
	peopleMemoryClerkHour = 4
	peopleMemoryClerkMin  = 30
)

// ClerkInvokeFunc re-invokes a dog (CLI client) one-shot and returns its final
// text. Used so the daily clerk lets the ORIGINAL dog re-derive the F276
// proposal from the exact sources — homologous: the cat (not the
// platform) does the reasoning. clientID selects the CLI adapter
// (claude/codex/gemini/opencode/kimi); workDir is the dog's working directory.
type ClerkInvokeFunc func(ctx context.Context, clientID, prompt, workDir string) (string, error)

// ClerkSourceResolver resolves a typed source coordinate to its owner-visible
// text so the re-invoked dog can reason over the actual content, not just refs.
type ClerkSourceResolver func(operatorID string, ref SourceRef) (string, bool)

// PeopleMemoryClerkDeps are the optional capabilities the daily clerk needs to
// re-invoke the original dog. When Invoke is nil the clerk degrades to the
// simpler "promote receipt to a staged (empty) candidate" behaviour (the
// original SG F276 dual path). DefaultClientID is used when a receipt carries
// no usable requester client id.
type PeopleMemoryClerkDeps struct {
	Invoke          ClerkInvokeFunc
	ResolveSource   ClerkSourceResolver
	DefaultClientID string
	WorkDir         string
}

// nextDailyRun returns the duration until the next occurrence of (hour, min) in
// the local timezone relative to now. If that time already passed today, the
// next occurrence is tomorrow. The result is always positive and strictly less
// than 24h. now is a parameter (not time.Now) so the schedule is testable.
func nextDailyRun(now time.Time, hour, min int) time.Duration {
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if !target.After(now) {
		target = target.AddDate(0, 0, 1)
	}
	return target.Sub(now)
}

// StartPeopleMemoryClerk launches the daily deferred-receipt clerk aligned to
// PeopleMemoryClerkCronSpec (04:30 local), the SG homologue of the
// DeferredPersonMemoryDailyTaskSpec. Each tick promotes ready (unclaimed,
// non-withdrawn) deferred receipts into rejectable capture candidates. When
// deps.Invoke is set it re-invokes the ORIGINAL dog (RequesterCat) so the dog —
// not the platform — re-derives the proposal from the exact sources; the
// platform only persists the dog's returned, rejectable proposal. It never
// silently materializes truth and never reads message bodies directly (the dog
// receives resolved source text). The goroutine exits when ctx is cancelled.
func StartPeopleMemoryClerk(ctx context.Context, store PeopleMemoryStore, deps PeopleMemoryClerkDeps) {
	go func() {
		for {
			timer := time.NewTimer(nextDailyRun(time.Now(), peopleMemoryClerkHour, peopleMemoryClerkMin))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				RunPeopleMemoryClerkOnce(ctx, store, deps)
			}
		}
	}()
}

// RunPeopleMemoryClerkOnce promotes ready deferred receipts for every operator.
// Exported so it can be triggered manually / from tests.
func RunPeopleMemoryClerkOnce(ctx context.Context, store PeopleMemoryStore, deps PeopleMemoryClerkDeps) {
	ops, err := store.ListOperators()
	if err != nil {
		log.Printf("[people-memory-clerk] list operators failed: %v", err)
		return
	}
	for _, op := range ops {
		ready, err := store.ListReadyDeferred(op)
		if err != nil {
			log.Printf("[people-memory-clerk] operator=%s list ready failed: %v", op, err)
			continue
		}
		processed := 0
		for _, r := range ready {
			if processed >= PeopleMemoryClerkBatchLimit {
				break
			}
			processed++
			if deps.Invoke == nil {
				// Degraded mode: promote to a staged (empty) candidate, exactly
				// like the original SG F276 dual path.
				if _, err := store.ClaimDeferredReceipt(op, r.ReceiptID, "clerk"); err != nil {
					log.Printf("[people-memory-clerk] operator=%s receipt=%s claim failed: %v", op, r.ReceiptID, err)
				}
				continue
			}
			if err := store.ReserveDeferredReceipt(op, r.ReceiptID, "clerk"); err != nil {
				log.Printf("[people-memory-clerk] operator=%s receipt=%s reserve failed: %v", op, r.ReceiptID, err)
				continue
			}
			clientID := r.RequesterCat
			if strings.TrimSpace(clientID) == "" {
				clientID = deps.DefaultClientID
			}
			prompt := buildClerkReinvokePrompt(op, r, deps)
			resp, invErr := deps.Invoke(ctx, clientID, prompt, deps.WorkDir)
			if invErr != nil {
				log.Printf("[people-memory-clerk] operator=%s receipt=%s invoke(%s) failed: %v", op, r.ReceiptID, clientID, invErr)
				// Fall back to promoting an empty staged candidate so the
				// operator still gets a reviewable item (no retry loop).
				_ = store.ReleaseDeferredReceipt(op, r.ReceiptID)
				if _, cerr := store.ClaimDeferredReceipt(op, r.ReceiptID, "clerk"); cerr != nil {
					log.Printf("[people-memory-clerk] operator=%s receipt=%s fallback claim failed: %v", op, r.ReceiptID, cerr)
				}
				continue
			}
			prop, ok := parseClerkProposal(resp)
			if !ok {
				// Insufficient evidence / defer: release so the receipt is ready
				// again and retried on the next daily run.
				log.Printf("[people-memory-clerk] operator=%s receipt=%s: dog returned no usable proposal (deferred/insufficient)", op, r.ReceiptID)
				_ = store.ReleaseDeferredReceipt(op, r.ReceiptID)
				continue
			}
			cand := buildCandidateFromProposal(op, r, prop)
			if _, perr := store.Propose(op, cand); perr != nil {
				log.Printf("[people-memory-clerk] operator=%s receipt=%s propose failed: %v", op, r.ReceiptID, perr)
				_ = store.ReleaseDeferredReceipt(op, r.ReceiptID)
				continue
			}
			log.Printf("[people-memory-clerk] operator=%s receipt=%s re-invoked %s -> candidate %s", op, r.ReceiptID, clientID, cand.CandidateID)
		}
	}
}

// clerkClaimDraft / clerkRelationshipDraft / clerkInteractionDraft / clerkProposal
// are the JSON contract the re-invoked dog must return. The dog outputs ONLY a
// single JSON object (no prose, no fences); on insufficient evidence it returns
// {"defer":true}.
type clerkClaimDraft struct {
	Kind       string `json:"kind"`       // reported_fact | user_assessment
	Predicate  string `json:"predicate"`  // for reported_fact
	Statement  string `json:"statement"`  // for user_assessment (or free note)
	Confidence string `json:"confidence"` // high | medium | low
}

type clerkRelationshipDraft struct {
	Status string `json:"status"` // current | former
	Line   string `json:"line"`
}

type clerkInteractionDraft struct {
	Headline   string `json:"headline"`
	OccurredAt string `json:"occurred_at"`
}

type clerkProposal struct {
	TargetPersonID   string                  `json:"target_person_id"`
	DisplayName      string                  `json:"display_name"`
	Aliases          []string                `json:"aliases"`
	ClaimDrafts      []clerkClaimDraft       `json:"claim_drafts"`
	RelationshipDraft *clerkRelationshipDraft `json:"relationship_draft"`
	InteractionDraft  *clerkInteractionDraft  `json:"interaction_draft"`
	Defer            bool                    `json:"defer"`
}

// buildClerkReinvokePrompt builds the hidden re-derivation prompt for the
// original dog, faithfully mirroring the triggerContent: exact sources
// only, no thread-history scanning, output a single rejectable F276 proposal as
// JSON, never materialize directly.
func buildClerkReinvokePrompt(operatorID string, r *DeferredPersonMemoryReceipt, deps PeopleMemoryClerkDeps) string {
	var b strings.Builder
	b.WriteString("[F276 deferred person-memory daily clerk]\n")
	fmt.Fprintf(&b, "receiptId=%s\n", r.ReceiptID)
	fmt.Fprintf(&b, "claimId=%s\n", "clerk")
	fmt.Fprintf(&b, "subject=%s\n", jsonString(r.Subject))
	personID := r.PersonID
	if personID == "" {
		personID = "(new person)"
	}
	fmt.Fprintf(&b, "personId=%s\n", personID)
	b.WriteString("registry=owner-private\n")
	b.WriteString("exact sources (read ONLY these; do not scan thread history or other conversations):\n")
	for _, ref := range r.SourceCoords {
		coord := fmt.Sprintf("%s#%s", ref.ThreadID, ref.MessageID)
		text := "(source body not resolvable in this context)"
		if deps.ResolveSource != nil {
			if resolved, ok := deps.ResolveSource(operatorID, ref); ok && resolved != "" {
				text = resolved
			}
		}
		fmt.Fprintf(&b, "- message %s: %s\n", coord, text)
	}
	b.WriteString("\nIf these exact sources support a useful, owner-confirmed known-person delta, output ONLY a single JSON object (no prose, no ``` fences) of this exact shape:\n")
	b.WriteString(`{"target_person_id":"","display_name":"","aliases":[],"claim_drafts":[{"kind":"reported_fact|user_assessment","predicate":"","statement":"","confidence":"high|medium|low"}],"relationship_draft":{"status":"current|former","line":""},"interaction_draft":{"headline":"","occurred_at":""}}` + "\n")
	if r.PersonID != "" {
		fmt.Fprintf(&b, "Set target_person_id to %q to attach to the existing person.\n", r.PersonID)
	}
	b.WriteString("Use null for relationship_draft / interaction_draft when not applicable.\n")
	fmt.Fprintf(&b, "deferredReceipt={\"receiptId\":\"%s\",\"claimId\":\"clerk\"}, clientRequestId=\"%s\".\n", r.ReceiptID, r.ReceiptID)
	b.WriteString("Never materialize memory yourself. Never fabricate sources. Never turn the correction/capture into an interaction event. If evidence is insufficient or lacks explicit owner confirmation, output exactly: {\"defer\":true}.\n")
	return b.String()
}

// parseClerkProposal extracts the first JSON object from the dog's response and
// validates it carries at least one draft. Returns ok=false on parse failure,
// an explicit {"defer":true}, or an empty proposal.
func parseClerkProposal(resp string) (*clerkProposal, bool) {
	start := strings.Index(resp, "{")
	end := strings.LastIndex(resp, "}")
	if start < 0 || end <= start {
		return nil, false
	}
	var p clerkProposal
	if err := json.Unmarshal([]byte(resp[start:end+1]), &p); err != nil {
		return nil, false
	}
	if p.Defer {
		return nil, false
	}
	if len(p.ClaimDrafts) == 0 && p.RelationshipDraft == nil && p.InteractionDraft == nil {
		return nil, false
	}
	return &p, true
}

// buildCandidateFromProposal maps the dog's re-derived proposal into a rejectable
// CaptureCandidate linked to the originating deferred receipt.
func buildCandidateFromProposal(operatorID string, r *DeferredPersonMemoryReceipt, p *clerkProposal) *CaptureCandidate {
	now := time.Now().UnixMilli()
	c := &CaptureCandidate{
		CandidateID:       "cand-" + uuid.NewString()[:12],
		RequesterCat:      r.RequesterCat,
		SourceMessageRef:  firstOrEmptySource(r.SourceCoords),
		TargetPersonID:    p.TargetPersonID,
		State:             CandPendingApproval,
		PresentedAt:       now,
		CreatedAt:         now,
		DeferredReceiptID: r.ReceiptID,
	}
	if strings.TrimSpace(p.DisplayName) != "" {
		c.PersonDraft = &PersonIdentityDraft{DisplayName: strings.TrimSpace(p.DisplayName), PrivateAliases: p.Aliases}
	}
	drafts := make([]CandidateClaimDraft, 0, len(p.ClaimDrafts))
	remaining := make([]string, 0, len(p.ClaimDrafts))
	for i, cd := range p.ClaimDrafts {
		did := fmt.Sprintf("d%d", i+1)
		payload := PersonClaimPayload{Kind: "reported_fact"}
		if strings.EqualFold(cd.Kind, "user_assessment") {
			payload.Kind = "user_assessment"
			payload.Statement = cd.Statement
		} else {
			payload.Kind = "reported_fact"
			payload.Predicate = cd.Predicate
			if cd.Statement != "" {
				payload.Value = cd.Statement
			}
		}
		drafts = append(drafts, CandidateClaimDraft{
			DraftID:         did,
			Payload:         payload,
			Decision:        "pending",
			SourceRole:      "quoted_third_party",
			EvidenceExcerpt: firstExcerpt(r.SourceCoords),
		})
		remaining = append(remaining, did)
	}
	c.ClaimDrafts = drafts
	c.RemainingDraftIDs = remaining
	if p.RelationshipDraft != nil {
		c.RelationshipDraft = &CandidateRelationshipDraft{Status: p.RelationshipDraft.Status, Decision: "pending"}
	}
	if p.InteractionDraft != nil {
		id := &CandidateInteractionDraft{Headline: p.InteractionDraft.Headline, Decision: "pending"}
		if p.InteractionDraft.OccurredAt != "" {
			id.OccurredAt = &TemporalValue{Kind: "exact", Value: p.InteractionDraft.OccurredAt}
		}
		c.InteractionDraft = id
	}
	return c
}

// firstExcerpt returns a bounded verbatim excerpt from the first source coord,
// if present (KD-7 privacy discipline: excerpt only, never the full body).
func firstExcerpt(coords []SourceRef) string {
	if len(coords) > 0 {
		return coords[0].Excerpt
	}
	return ""
}

// jsonString quotes s for safe inline embedding in the prompt.
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return "\"\""
	}
	return string(b)
}
