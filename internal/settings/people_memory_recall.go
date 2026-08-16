package settings

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// This file holds the F276 read/derive surface (RelationshipCard recall,
// alias resolution, listings) and the dual-path deferred-receipt machinery
// (content-free receipts that a daily clerk claims back into proposals).

// RelationshipCardFact is one bounded fact on a recall card.
type RelationshipCardFact struct {
	ClaimID        string      `json:"claim_id"`
	Text           string      `json:"text"`
	Kind           string      `json:"kind"`
	ProvenanceRefs []SourceRef `json:"provenance_refs"`
}

// RelationshipCardInteraction is the latest interaction headline on a card.
type RelationshipCardInteraction struct {
	EventID    string         `json:"event_id"`
	OccurredAt *TemporalValue `json:"occurred_at,omitempty"`
	Headline   string         `json:"headline"`
}

// RelationshipCard is the derived, bounded, non-storable recall projection
// (object 6). storable/indexable are always false (KD-7).
type RelationshipCard struct {
	PersonID          string                     `json:"person_id"`
	RelationshipID    string                     `json:"relationship_id"`
	DisplayName       string                     `json:"display_name"`
	Facts             []RelationshipCardFact     `json:"facts"`
	RelationshipLine  string                     `json:"relationship_line,omitempty"`
	LatestInteraction *RelationshipCardInteraction `json:"latest_interaction,omitempty"`
	Uncertainty       []string                   `json:"uncertainty"`
	ProvenanceRefs    []SourceRef                `json:"provenance_refs"`
	Storable          bool                       `json:"storable"`  // always false
	Indexable         bool                       `json:"indexable"` // always false
}

// recallCard builds the bounded relationship card for an active person.
func (d *peopleMemoryDocument) recallCard(personID string) (*RelationshipCard, bool, error) {
	p, ok := d.People[personID]
	if !ok || p.Status != PersonStatusActive {
		return nil, false, nil
	}
	card := &RelationshipCard{
		PersonID:      personID,
		DisplayName:   p.DisplayName,
		Facts:         make([]RelationshipCardFact, 0),
		Uncertainty:   make([]string, 0),
		Storable:      false,
		Indexable:     false,
	}
	for _, r := range d.Relationships[personID] {
		if r.Status == RelStatusCurrent {
			card.RelationshipID = r.RelationshipID
			card.RelationshipLine = "关系状态: " + r.Status
			for _, sr := range r.SourceRefs {
				card.ProvenanceRefs = append(card.ProvenanceRefs, sr)
			}
		}
	}
	claims := d.Claims[personID]
	sortedClaims := make([]*PersonClaimVersion, 0, len(claims))
	for _, c := range claims {
		if c.Status == ClaimStatusCurrent {
			sortedClaims = append(sortedClaims, c)
		}
	}
	sort.Slice(sortedClaims, func(i, j int) bool { return sortedClaims[i].RecordedAt > sortedClaims[j].RecordedAt })
	for i, c := range sortedClaims {
		if i >= 3 {
			break
		}
		text := c.Payload.Predicate
		if c.Payload.Kind == ClaimKindUserAssessment {
			text = c.Payload.Statement
		}
		card.Facts = append(card.Facts, RelationshipCardFact{
			ClaimID:        c.ClaimID,
			Text:           text,
			Kind:           c.Payload.Kind,
			ProvenanceRefs: c.SourceRefs,
		})
	}
	events := d.Events[personID]
	sortedEvents := make([]*InteractionEvent, 0, len(events))
	for _, e := range events {
		if e.Status == EventStatusActive {
			sortedEvents = append(sortedEvents, e)
		}
	}
	sort.Slice(sortedEvents, func(i, j int) bool { return sortedEvents[i].RecordedAt > sortedEvents[j].RecordedAt })
	if len(sortedEvents) > 0 {
		e := sortedEvents[0]
		card.LatestInteraction = &RelationshipCardInteraction{
			EventID:    e.EventID,
			OccurredAt: e.OccurredAt,
			Headline:   e.Headline,
		}
	}
	return card, true, nil
}

// resolveActivePersonByAlias finds the active person whose display name or alias
// matches. Ambiguous matches fail closed.
func (d *peopleMemoryDocument) resolveActivePersonByAlias(alias string) (string, error) {
	needle := strings.ToLower(strings.TrimSpace(alias))
	if needle == "" {
		return "", fmt.Errorf("empty alias")
	}
	var hit string
	matches := 0
	for id, p := range d.People {
		if p.Status != PersonStatusActive {
			continue
		}
		if strings.ToLower(p.DisplayName) == needle {
			hit = id
			matches++
			continue
		}
		for _, a := range p.PrivateAliases {
			if strings.ToLower(a) == needle {
				hit = id
				matches++
				break
			}
		}
		if matches > 1 {
			return "", fmt.Errorf("ambiguous alias %q matches multiple active people", alias)
		}
	}
	if matches == 0 {
		return "", nil
	}
	return hit, nil
}

func (d *peopleMemoryDocument) listPeople() []*PersonIdentity {
	out := make([]*PersonIdentity, 0, len(d.People))
	for _, p := range d.People {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out
}

func (d *peopleMemoryDocument) listClaims(personID string) []*PersonClaimVersion {
	out := append([]*PersonClaimVersion(nil), d.Claims[personID]...)
	sort.Slice(out, func(i, j int) bool { return out[i].RecordedAt > out[j].RecordedAt })
	return out
}

func (d *peopleMemoryDocument) listRelationships(personID string) []*PersonRelationship {
	out := append([]*PersonRelationship(nil), d.Relationships[personID]...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (d *peopleMemoryDocument) listEvents(personID string) []*InteractionEvent {
	out := append([]*InteractionEvent(nil), d.Events[personID]...)
	sort.Slice(out, func(i, j int) bool { return out[i].RecordedAt > out[j].RecordedAt })
	return out
}

// deferReceipt creates a content-free, exact-source-bound deferred receipt (AC-A20).
func (d *peopleMemoryDocument) deferReceipt(ownerUserID, requesterCat, subject, personID string, coords []SourceRef) (*DeferredPersonMemoryReceipt, error) {
	r := &DeferredPersonMemoryReceipt{
		ReceiptID:    "rcpt-" + uuid.NewString()[:12],
		OwnerUserID:  ownerUserID,
		RequesterCat: requesterCat,
		Subject:      subject,
		PersonID:     personID,
		SourceCoords: coords,
		CreatedAt:    time.Now().UnixMilli(),
	}
	d.Receipts[r.ReceiptID] = r
	return r, nil
}

// listReadyDeferred returns unclaimed, non-withdrawn receipts (the clerk's queue).
// Bounded to at most 8 (AC-A22).
func (d *peopleMemoryDocument) listReadyDeferred() []*DeferredPersonMemoryReceipt {
	out := make([]*DeferredPersonMemoryReceipt, 0)
	for _, r := range d.Receipts {
		if !r.Withdrawn && r.ClaimedAt == 0 {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

// claimDeferredReceipt converts a deferred receipt into a staged candidate.
func (d *peopleMemoryDocument) claimDeferredReceipt(receiptID, requesterCat string) (*CaptureCandidate, error) {
	r, ok := d.Receipts[receiptID]
	if !ok {
		return nil, fmt.Errorf("receipt %q not found", receiptID)
	}
	if r.Withdrawn {
		return nil, fmt.Errorf("receipt %q was withdrawn", receiptID)
	}
	if r.ClaimedAt != 0 {
		return nil, fmt.Errorf("receipt %q already claimed", receiptID)
	}
	now := time.Now().UnixMilli()
	c := &CaptureCandidate{
		CandidateID:       "cand-" + uuid.NewString()[:12],
		RequesterCat:      requesterCat,
		SourceMessageRef:  firstOrEmptySource(r.SourceCoords),
		TargetPersonID:    r.PersonID,
		State:             CandPendingApproval,
		PresentedAt:       now,
		CreatedAt:         now,
		DeferredReceiptID: r.ReceiptID,
	}
	d.Candidates[c.CandidateID] = c
	r.ClaimedAt = now
	r.ClaimID = c.CandidateID
	d.reindexPending()
	return c, nil
}

// reserveDeferredReceipt marks a receipt as claimed WITHOUT creating a staged
// candidate. Used by the daily clerk before re-invoking the original dog: the
// receipt is reserved so it is not double-processed, and the actual candidate
// is created later from the dog's re-derived proposal (RunPeopleMemoryClerkOnce
// calls Propose after the dog returns). by identifies the claimant (e.g. "clerk").
func (d *peopleMemoryDocument) reserveDeferredReceipt(receiptID, by string) error {
	r, ok := d.Receipts[receiptID]
	if !ok {
		return fmt.Errorf("receipt %q not found", receiptID)
	}
	if r.Withdrawn {
		return fmt.Errorf("receipt %q was withdrawn", receiptID)
	}
	if r.ClaimedAt != 0 {
		return fmt.Errorf("receipt %q already claimed", receiptID)
	}
	now := time.Now().UnixMilli()
	r.ClaimedAt = now
	r.ClaimID = "reserved:" + by
	d.reindexPending()
	return nil
}

// releaseDeferredReceipt clears a reservation so the receipt becomes ready
// again. Used when the re-invoked dog fails or returns insufficient evidence
// and the clerk wants to retry the receipt on the next daily run.
func (d *peopleMemoryDocument) releaseDeferredReceipt(receiptID string) error {
	r, ok := d.Receipts[receiptID]
	if !ok {
		return fmt.Errorf("receipt %q not found", receiptID)
	}
	r.ClaimedAt = 0
	r.ClaimID = ""
	d.reindexPending()
	return nil
}

func (d *peopleMemoryDocument) withdrawReceipt(receiptID string) error {
	r, ok := d.Receipts[receiptID]
	if !ok {
		return fmt.Errorf("receipt %q not found", receiptID)
	}
	if r.ClaimedAt != 0 {
		return fmt.Errorf("receipt %q already claimed into a candidate; withdraw the candidate instead", receiptID)
	}
	r.Withdrawn = true
	d.reindexPending()
	return nil
}

func (d *peopleMemoryDocument) forgetReceipt(receiptID string) error {
	r, ok := d.Receipts[receiptID]
	if !ok {
		return fmt.Errorf("receipt %q not found", receiptID)
	}
	if r.ClaimedAt != 0 {
		return fmt.Errorf("receipt %q already claimed; forget the candidate instead", receiptID)
	}
	delete(d.Receipts, receiptID)
	d.reindexPending()
	return nil
}

func firstOrEmptySource(coords []SourceRef) SourceRef {
	if len(coords) > 0 {
		return coords[0]
	}
	return SourceRef{}
}

// PeopleMemoryRecallCardTokenCap mirrors the upstream maxRelationshipCardTokens:
// a single injected relationship card must stay within this token budget so the
// chat-card injection never bloats the dog's context.
const PeopleMemoryRecallCardTokenCap = 160

// PeopleMemoryRecallInjectionBudgetTokens is the per-turn budget for the whole
// "关系记忆" injection block (homologous aggregate token budget).
const PeopleMemoryRecallInjectionBudgetTokens = 600

// F276 drill budget constants — homologous PERSON_MEMORY_LIMITS.
// A drill is an on-demand, source-tied projection the dog asks for when it wants
// the verbatim backing of a claim/relationship/event it only saw as a bounded
// card. The per-call cap stops a single drill from bloating the context; the
// per-person-per-turn and aggregate-per-turn caps stop a chatty turn from
// draining the whole budget (the upstream drill-discipline, AC-A20 family).
const (
	// PeopleMemoryDrillMaxTokensPerCall is the hard cap on one projected text
	// (maxDrillTokensPerCall=500).
	PeopleMemoryDrillMaxTokensPerCall = 500
	// PeopleMemoryDrillMaxPerPersonPerTurn caps drills for one person per turn
	// (maxDrillsPerPersonPerTurn=3).
	PeopleMemoryDrillMaxPerPersonPerTurn = 3
	// PeopleMemoryDrillMaxAggregatePerTurn caps all drill tokens per turn
	// (maxPersonMemoryTokensPerTurn=1200).
	PeopleMemoryDrillMaxAggregatePerTurn = 1200
)

// PeopleMemoryDrillItemKind enumerates the three recall items a drill can target.
const (
	DrillItemClaim       = "claim"
	DrillItemRelationship = "relationship"
	DrillItemEvent       = "event"
)

// PeopleMemoryDrillInput requests the verbatim backing of one recall item,
// scoped to a (owner, turn) budget and a time window (PersonMemoryDrillInput).
type PeopleMemoryDrillInput struct {
	TurnID         string `json:"turn_id"`
	PersonID       string `json:"person_id"`
	ItemKind       string `json:"item_kind"` // claim | relationship | event
	ItemID         string `json:"item_id"`
	TimeWindowFrom int64  `json:"time_window_from"`
	TimeWindowTo   int64  `json:"time_window_to"`
}

// PeopleMemoryDrillResult is the outcome of a drill (PersonMemoryDrillResult).
// Status is one of "ok" | "not_available" | "budget_exceeded".
type PeopleMemoryDrillResult struct {
	Status         string    `json:"status"`
	Kind           string    `json:"kind,omitempty"`
	ItemID         string    `json:"item_id,omitempty"`
	Text           string    `json:"text,omitempty"`
	SourceRef      SourceRef `json:"source_ref,omitempty"`
	EstimatedTokens int      `json:"estimated_tokens,omitempty"`
}

// drillTurnBudget tracks the per-turn drill spend for one (operator, turn). It is
// ephemeral process state (never persisted) — exactly homologous to the
// in-memory PersonMemoryRecallService.budgets map.
type drillTurnBudget struct {
	aggregateTokens int
	callsByPerson   map[string]int
}

// peopleMemoryDrillBudgetCap guards the in-memory budget map from unbounded
// growth in a long-lived server: when exceeded, the whole map is reset (a
// best-effort memory guard; per-turn budgets are short-lived, so the loss is
// negligible).
const peopleMemoryDrillBudgetCap = 4096

// boundedProjectionText truncates value so its estimated tokens stay within
// PeopleMemoryDrillMaxTokensPerCall, mirroring the upstream 0.8x shrink loop. It
// returns the truncated text and its estimated token count.
func boundedProjectionText(value string) (string, int) {
	text := value
	tk := estimateTokens(text)
	runes := []rune(text)
	for tk > PeopleMemoryDrillMaxTokensPerCall && len(runes) > 1 {
		keep := int(float64(len(runes)) * 0.8)
		if keep < 1 {
			keep = 1
		}
		runes = runes[:keep]
		text = string(runes) + "…"
		tk = estimateTokens(text)
	}
	return text, tk
}

// drillFindItem is the pure, budget-free data lookup for a drill: it resolves a
// claim/relationship/event by id within a person and returns its verbatim text,
// first source ref, and recorded-at time (findItem). Returns ok=false
// when the item is absent or tombstoned.
func (d *peopleMemoryDocument) drillFindItem(input PeopleMemoryDrillInput) (text string, src SourceRef, recordedAt int64, ok bool) {
	switch input.ItemKind {
	case DrillItemClaim:
		for _, c := range d.Claims[input.PersonID] {
			if c.ClaimID != input.ItemID || c.Status == ClaimStatusRedacted {
				continue
			}
			t := c.Payload.Predicate
			if c.Payload.Kind == ClaimKindUserAssessment {
				t = c.Payload.Statement
			}
			if len(c.SourceRefs) > 0 {
				src = c.SourceRefs[0]
			}
			return t, src, c.RecordedAt, true
		}
	case DrillItemRelationship:
		for _, r := range d.Relationships[input.PersonID] {
			if r.RelationshipID != input.ItemID {
				continue
			}
			t := "关系状态: " + r.Status
			if len(r.SourceRefs) > 0 {
				src = r.SourceRefs[0]
			}
			rt := r.CreatedAt
			if len(r.Transitions) > 0 {
				rt = r.Transitions[len(r.Transitions)-1].RecordedAt
			}
			return t, src, rt, true
		}
	case DrillItemEvent:
		for _, e := range d.Events[input.PersonID] {
			if e.EventID != input.ItemID || e.Status == EventStatusRedacted {
				continue
			}
			t := e.Headline
			if len(e.SourceRefs) > 0 {
				src = e.SourceRefs[0]
			}
			return t, src, e.RecordedAt, true
		}
	}
	return "", SourceRef{}, 0, false
}

// recallContextForQuery returns a homologous "anchor-first" recall block
// (F236): when the user's message references a known third-party person (by
// display name or alias), it injects a token-bounded RelationshipCard so the dog
// "remembers" them. Returns ("", false) when nothing matches or the query is
// empty. Cards are trimmed to PeopleMemoryRecallCardTokenCap and the whole
// block is capped at PeopleMemoryRecallInjectionBudgetTokens.
func (d *peopleMemoryDocument) recallContextForQuery(query string) (string, bool) {
	if strings.TrimSpace(query) == "" {
		return "", false
	}
	q := strings.ToLower(query)
	matched := make([]string, 0)
	for pid, p := range d.People {
		if p.Status != PersonStatusActive {
			continue
		}
		if strings.Contains(q, strings.ToLower(strings.TrimSpace(p.DisplayName))) {
			matched = append(matched, pid)
			continue
		}
		for _, a := range p.PrivateAliases {
			if a != "" && strings.Contains(q, strings.ToLower(a)) {
				matched = append(matched, pid)
				break
			}
		}
	}
	if len(matched) == 0 {
		return "", false
	}
	var sb strings.Builder
	sb.WriteString("## 关系记忆\n\n")
	sb.WriteString("> 以下为你长期记忆中的第三方人物（独立于本次会话历史）。如相关请据此理解对方；不确定时勿臆断。\n\n")
	used := 0
	for _, pid := range matched {
		card, ok, _ := d.recallCard(pid)
		if !ok {
			continue
		}
		txt := formatRecallCard(card)
		tk := estimateTokens(txt)
		if tk > PeopleMemoryRecallCardTokenCap {
			txt = trimRecallCard(card, PeopleMemoryRecallCardTokenCap)
			tk = estimateTokens(txt)
		}
		if tk <= 0 {
			continue
		}
		if used+tk > PeopleMemoryRecallInjectionBudgetTokens {
			// try to fit a trimmed version within remaining budget
			room := PeopleMemoryRecallInjectionBudgetTokens - used
			if room > 40 {
				txt = trimRecallCard(card, room)
				tk = estimateTokens(txt)
			}
		}
		if used+tk > PeopleMemoryRecallInjectionBudgetTokens || tk <= 0 {
			continue
		}
		sb.WriteString(txt)
		sb.WriteString("\n")
		used += tk
	}
	if used == 0 {
		return "", false
	}
	return sb.String(), true
}

// formatRecallCard renders a RelationshipCard as a compact markdown block.
func formatRecallCard(card *RelationshipCard) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n", card.DisplayName)
	if card.RelationshipLine != "" {
		fmt.Fprintf(&b, "- 关系: %s\n", card.RelationshipLine)
	}
	if len(card.Facts) > 0 {
		b.WriteString("- 事实:\n")
		for _, f := range card.Facts {
			fmt.Fprintf(&b, "  - %s\n", f.Text)
		}
	}
	if card.LatestInteraction != nil && card.LatestInteraction.Headline != "" {
		when := ""
		if card.LatestInteraction.OccurredAt != nil && card.LatestInteraction.OccurredAt.Value != "" {
			when = " (" + card.LatestInteraction.OccurredAt.Value + ")"
		}
		fmt.Fprintf(&b, "- 最近互动: %s%s\n", card.LatestInteraction.Headline, when)
	}
	return b.String()
}

// trimRecallCard renders the card but drops facts (oldest-first) until it fits
// within tokenBudget runes/4; if still over, it drops the interaction and
// relationship line (homologous degradation).
func trimRecallCard(card *RelationshipCard, tokenBudget int) string {
	trimmed := &RelationshipCard{
		PersonID:         card.PersonID,
		DisplayName:      card.DisplayName,
		RelationshipLine: card.RelationshipLine,
		LatestInteraction: card.LatestInteraction,
		Facts:            append([]RelationshipCardFact(nil), card.Facts...),
	}
	for len(trimmed.Facts) > 0 && estimateTokens(formatRecallCard(trimmed)) > tokenBudget {
		trimmed.Facts = trimmed.Facts[:len(trimmed.Facts)-1]
	}
	if estimateTokens(formatRecallCard(trimmed)) > tokenBudget {
		trimmed.LatestInteraction = nil
	}
	if estimateTokens(formatRecallCard(trimmed)) > tokenBudget {
		trimmed.RelationshipLine = ""
	}
	return formatRecallCard(trimmed)
}

// estimateTokens is a lightweight CJK-friendly token estimate (~4 runes/token).
func estimateTokens(s string) int {
	return len([]rune(s)) / 4
}
