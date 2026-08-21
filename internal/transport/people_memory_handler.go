package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/telemetry"
)

// PeopleMemoryHandler exposes the "People & Relationship Memory" store over
// HTTP. Like the capsule handler, NO reasoning runs inside the platform: a
// candidate is submitted as a proposal and only an explicit approval
// materializes it (不可逆决策 §4.1). The content submitted is treated as operator/
// CLI-authored truth; the platform stores and projects it only.
//
// Multi-operator: every request resolves its owner scope (operatorID) from the
// X-Operator-Id header, falling back to the server-derived defaultOperator
// (the platform leader). All store calls are operator-scoped so operators are
// isolated (KD-1). Cross-thread source authorization is fail-closed.
type PeopleMemoryHandler struct {
	store          settings.PeopleMemoryStore
	defaultOperator string
	authorizer     settings.SourceAuthorizer
	hub            *settings.PeopleMemoryEventHub
}

// NewPeopleMemoryHandler creates the handler. defaultOperator is the leader
// name (used when no X-Operator-Id header is present). authorizer enforces
// cross-thread source checks; a nil authorizer degrades to AllowAll. hub is the
// in-process event bus used by the SSE endpoint for cross-tab live sync; it may
// be nil (the endpoint then reports unavailable, but all other routes work).
func NewPeopleMemoryHandler(store settings.PeopleMemoryStore, defaultOperator string, authorizer settings.SourceAuthorizer, hub *settings.PeopleMemoryEventHub) *PeopleMemoryHandler {
	if defaultOperator == "" {
		defaultOperator = "operator"
	}
	if authorizer == nil {
		authorizer = settings.AllowAllAuthorizer{}
	}
	return &PeopleMemoryHandler{store: store, defaultOperator: defaultOperator, authorizer: authorizer, hub: hub}
}

// resolveOperator derives the owner scope for a request: X-Operator-Id header,
// else the server-derived default (leader). ResolveStrictUserId is
// stricter (401 when neither cookie nor header); SG is single-leader today, so
// the header is optional and falls back to the leader.
func (h *PeopleMemoryHandler) resolveOperator(r *http.Request) string {
	if op := strings.TrimSpace(r.Header.Get("X-Operator-Id")); op != "" {
		return op
	}
	return h.defaultOperator
}

// resolveOperatorQuery derives the owner scope for an SSE subscription from the
// ?operator= query parameter. EventSource (browser SSE) cannot send custom
// headers, so the operator is passed in the URL; the fallback to defaultOperator
// mirrors resolveOperator's X-Operator-Id behaviour.
func (h *PeopleMemoryHandler) resolveOperatorQuery(r *http.Request) string {
	if op := strings.TrimSpace(r.URL.Query().Get("operator")); op != "" {
		return op
	}
	return h.defaultOperator
}

// StreamEvents is an SSE endpoint that pushes PeopleMemoryEvent to subscribers
// viewing a given operator, so multiple browser tabs stay in sync: when one tab
// approves/rejects/forgets a proposal (or edits a person), every other tab
// viewing that operator refreshes. The stream is filtered server-side by
// operator so a tab only receives events for its own scope.
func (h *PeopleMemoryHandler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	if h.hub == nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "event hub unavailable"})
		return
	}
	operator := h.resolveOperatorQuery(r)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprintf(w, ": connected operator=%s\n\n", operator)
	flusher.Flush()

	ch := h.hub.Subscribe()
	defer h.hub.Unsubscribe(ch)

	// Heartbeat keeps idle proxies from closing the connection.
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case ev, open := <-ch:
			if !open {
				return
			}
			if ev.OperatorID != operator {
				continue
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// Routes mounts the endpoints under /api/people-memory. Persons are
// namespaced under /person/ so the {personID} wildcard never collides with the
// static /candidates and /deferred siblings (Go 1.22 ServeMux rule).
func (h *PeopleMemoryHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/people-memory", h.ListPeople)
	mux.HandleFunc("GET /api/people-memory/operators", h.ListOperators)
	mux.HandleFunc("GET /api/people-memory/events", h.StreamEvents)
	mux.HandleFunc("GET /api/people-memory/candidates", h.ListCandidates)
	mux.HandleFunc("GET /api/people-memory/deferred", h.ListDeferred)
	mux.HandleFunc("GET /api/people-memory/person/{personID}", h.GetPerson)
	mux.HandleFunc("GET /api/people-memory/person/{personID}/card", h.RecallCard)
	mux.HandleFunc("POST /api/people-memory/recall/drill", h.Drill)
	mux.HandleFunc("POST /api/people-memory/propose", h.Propose)
	mux.HandleFunc("POST /api/people-memory/defer", h.Defer)
	mux.HandleFunc("GET /api/people-memory/candidates/{candidateID}", h.GetCandidate)
	mux.HandleFunc("POST /api/people-memory/candidates/{candidateID}/approve", h.Approve)
	mux.HandleFunc("POST /api/people-memory/candidates/{candidateID}/reject-drafts", h.RejectDrafts)
	mux.HandleFunc("POST /api/people-memory/candidates/{candidateID}/reject", h.Reject)
	mux.HandleFunc("POST /api/people-memory/candidates/{candidateID}/not-now", h.NotNow)
	mux.HandleFunc("POST /api/people-memory/candidates/{candidateID}/withdraw", h.Withdraw)
	mux.HandleFunc("POST /api/people-memory/candidates/{candidateID}/undo", h.Undo)
	mux.HandleFunc("POST /api/people-memory/candidates/{candidateID}/forget", h.ForgetProposal)
	mux.HandleFunc("POST /api/people-memory/person/{personID}/claims/{claimID}/correct", h.CorrectClaim)
	mux.HandleFunc("POST /api/people-memory/person/{personID}/claims/{claimID}/retire", h.RetireClaim)
	mux.HandleFunc("POST /api/people-memory/person/{personID}/events/{eventID}/amend", h.AmendEvent)
	mux.HandleFunc("POST /api/people-memory/person/{personID}/items/redact", h.RedactItem)
	mux.HandleFunc("POST /api/people-memory/person/{personID}/forget", h.ForgetPerson)
	mux.HandleFunc("POST /api/people-memory/deferred/{receiptID}/claim", h.ClaimDeferred)
	mux.HandleFunc("POST /api/people-memory/deferred/{receiptID}/withdraw", h.WithdrawReceipt)
	mux.HandleFunc("POST /api/people-memory/deferred/{receiptID}/forget", h.ForgetReceipt)
	return mux
}

// ListPeople returns active people as compact summaries.
func (h *PeopleMemoryHandler) ListPeople(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	people, err := h.store.ListPeople(op)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(people))
	for _, p := range people {
		claims, _ := h.store.ListClaims(op, p.PersonID)
		events, _ := h.store.ListEvents(op, p.PersonID)
		cur := 0
		for _, c := range claims {
			if c.Status == settings.ClaimStatusCurrent {
				cur++
			}
		}
		out = append(out, map[string]any{
			"person_id":     p.PersonID,
			"display_name":  p.DisplayName,
			"aliases":       p.PrivateAliases,
			"status":        p.Status,
			"current_claims": cur,
			"events":        len(events),
		})
	}
	respondJSON(w, http.StatusOK, out)
}

// ListOperators returns the operators that currently hold people-memory data.
// Used by the frontend operator switcher so a user can scope the whole panel to
// a particular owner (KD-1 owner-partitioned memory).
func (h *PeopleMemoryHandler) ListOperators(w http.ResponseWriter, r *http.Request) {
	ops, err := h.store.ListOperators()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, ops)
}

// GetPerson returns a person with all claims, relationships, events, and card.
func (h *PeopleMemoryHandler) GetPerson(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	pid := r.PathValue("personID")
	p, ok, err := h.store.GetPerson(op, pid)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
		return
	}
	claims, _ := h.store.ListClaims(op, pid)
	rels, _ := h.store.ListRelationships(op, pid)
	events, _ := h.store.ListEvents(op, pid)
	card, hasCard, _ := h.store.RecallCard(op, pid)
	respondJSON(w, http.StatusOK, map[string]any{
		"person":       p,
		"claims":       claims,
		"relationships": rels,
		"events":       events,
		"card":         card,
		"has_card":     hasCard,
	})
}

// RecallCard returns the bounded relationship card. Supports ?alias= resolution.
func (h *PeopleMemoryHandler) RecallCard(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	pid := r.PathValue("personID")
	if alias := r.URL.Query().Get("alias"); alias != "" {
		resolved, err := h.store.ResolveActivePersonByAlias(op, alias)
		if err != nil {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if resolved == "" {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "no active person matches alias"})
			return
		}
		pid = resolved
	}
	card, ok, err := h.store.RecallCard(op, pid)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "no active card"})
		return
	}
	respondJSON(w, http.StatusOK, card)
}

// Drill returns the verbatim backing of one recall item (claim / relationship /
// event) on demand, enforcing per-turn drill budgets. It is
// read-only: the store only consults the ephemeral (operator, turn) budget map,
// never the persisted document. Response is PeopleMemoryDrillResult
// (status: ok | not_available | budget_exceeded).
func (h *PeopleMemoryHandler) Drill(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	var input settings.PeopleMemoryDrillInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if strings.TrimSpace(input.PersonID) == "" || strings.TrimSpace(input.ItemID) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "person_id and item_id are required"})
		return
	}
	if input.TurnID == "" {
		input.TurnID = "default"
	}
	res, err := h.store.RecallDrill(op, input)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if telemetry.IsInitialized() && telemetry.PeopleMemoryDrillInvoked != nil {
		telemetry.PeopleMemoryDrillInvoked.Add(context.Background(), 1)
	}
	respondJSON(w, http.StatusOK, res)
}

// Propose stages a capture candidate. Body mirrors CaptureCandidate (drafts etc).
func (h *PeopleMemoryHandler) Propose(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	var c settings.CaptureCandidate
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if c.RequesterDog == "" {
		c.RequesterDog = op
	}
	// Cross-thread source authorization (fail-closed).
	if ok, _ := h.authorizer.AuthorizeSource(r.Context(), op, c.SourceMessageRef); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "source not authorized for this operator"})
		return
	}
	stored, err := h.store.Propose(op, &c)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if telemetry.IsInitialized() && telemetry.ProfileUpdateProposed != nil {
		telemetry.PeopleMemoryProposed.Add(context.Background(), 1)
	}
	respondJSON(w, http.StatusCreated, stored)
}

// Defer creates a content-free deferred receipt (the dual-path "capture/defer").
func (h *PeopleMemoryHandler) Defer(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	var body struct {
		Subject   string               `json:"subject"`
		PersonID  string               `json:"person_id"`
		Requester string               `json:"requester_dog"`
		Coords    []settings.SourceRef `json:"source_coords"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if strings.TrimSpace(body.Subject) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "subject is required"})
		return
	}
	// Authorize every supplied source coordinate (fail-closed).
	for _, co := range body.Coords {
		if ok, _ := h.authorizer.AuthorizeSource(r.Context(), op, co); !ok {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "source coordinate not authorized for this operator"})
			return
		}
	}
	receipt, err := h.store.DeferReceipt(op, body.Requester, body.Subject, body.PersonID, body.Coords)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, receipt)
}

// ListCandidates returns actionable (pending) candidates.
func (h *PeopleMemoryHandler) ListCandidates(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	list, err := h.store.ListPending(op, 0)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// ListDeferred returns ready (unclaimed) deferred receipts.
func (h *PeopleMemoryHandler) ListDeferred(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	list, err := h.store.ListReadyDeferred(op)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// GetCandidate returns a single candidate.
func (h *PeopleMemoryHandler) GetCandidate(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("candidateID")
	c, ok, err := h.store.GetCandidate(op, id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "candidate not found"})
		return
	}
	respondJSON(w, http.StatusOK, c)
}

// Approve approves the selected draft ids of a candidate (materializes truth).
func (h *PeopleMemoryHandler) Approve(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("candidateID")
	var body struct {
		DraftIDs []string `json:"draft_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	receipt, err := h.store.ApproveDrafts(op, id, body.DraftIDs)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if telemetry.IsInitialized() && telemetry.ProfileUpdateApproved != nil {
		telemetry.PeopleMemoryApproved.Add(context.Background(), 1)
	}
	respondJSON(w, http.StatusOK, receipt)
}

// RejectDrafts rejects the selected draft ids of a candidate individually
// (per-card reject) — the drafts are dropped and never
// materialized. The candidate resolves once every draft is decided.
func (h *PeopleMemoryHandler) RejectDrafts(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("candidateID")
	var body struct {
		DraftIDs []string `json:"draft_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	c, err := h.store.RejectDrafts(op, id, body.DraftIDs)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if telemetry.IsInitialized() && telemetry.ProfileUpdateRejected != nil {
		telemetry.PeopleMemoryRejected.Add(context.Background(), 1)
	}
	respondJSON(w, http.StatusOK, c)
}

// Reject discards a candidate.
func (h *PeopleMemoryHandler) Reject(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("candidateID")
	c, err := h.store.RejectCandidate(op, id)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if telemetry.IsInitialized() && telemetry.ProfileUpdateRejected != nil {
		telemetry.PeopleMemoryRejected.Add(context.Background(), 1)
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "rejected", "candidate_id": c.CandidateID})
}

// NotNow keeps the candidate in the owner-visible pending list (TTL=0).
func (h *PeopleMemoryHandler) NotNow(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("candidateID")
	c, err := h.store.MarkNotNow(op, id)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, c)
}

// Withdraw retracts a not-yet-materialized candidate.
func (h *PeopleMemoryHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("candidateID")
	c, err := h.store.WithdrawCandidate(op, id)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, c)
}

// Undo reverts a prior approval decision.
func (h *PeopleMemoryHandler) Undo(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	var body struct {
		DecisionID string `json:"decision_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DecisionID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "decision_id required"})
		return
	}
	c, err := h.store.UndoDecision(op, body.DecisionID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, c)
}

// CorrectClaim supersedes a current claim with a corrected version.
func (h *PeopleMemoryHandler) CorrectClaim(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	pid := r.PathValue("personID")
	cid := r.PathValue("claimID")
	var body struct {
		Payload   settings.PersonClaimPayload `json:"payload"`
		SourceRef settings.SourceRef          `json:"source_ref"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	cv, err := h.store.CorrectClaim(op, pid, cid, body.Payload, body.SourceRef)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, cv)
}

// RetireClaim retires (preserves history of) a current claim.
func (h *PeopleMemoryHandler) RetireClaim(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	pid := r.PathValue("personID")
	cid := r.PathValue("claimID")
	var body struct {
		SourceRef settings.SourceRef `json:"source_ref"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.store.RetireClaim(op, pid, cid, body.SourceRef); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "retired", "claim_id": cid})
}

// AmendEvent creates a new event amending a prior one.
func (h *PeopleMemoryHandler) AmendEvent(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	pid := r.PathValue("personID")
	eid := r.PathValue("eventID")
	var body struct {
		Payload   settings.CandidateInteractionDraft `json:"payload"`
		SourceRef settings.SourceRef                 `json:"source_ref"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	ev, err := h.store.AmendInteraction(op, pid, eid, body.Payload, body.SourceRef)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, ev)
}

// RedactItem purges payload + source refs of a claim or event (tombstone).
func (h *PeopleMemoryHandler) RedactItem(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	pid := r.PathValue("personID")
	var body struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.store.RedactItem(op, pid, settings.RedactTarget{Kind: body.Kind, ID: body.ID}); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "redacted", "kind": body.Kind, "id": body.ID})
}

// ForgetPerson hard-forgets a whole person relationship.
func (h *PeopleMemoryHandler) ForgetPerson(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	pid := r.PathValue("personID")
	receipt, err := h.store.HardForget(op, pid)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, receipt)
}

// ForgetProposal hard-forgets an unbound terminal candidate.
func (h *PeopleMemoryHandler) ForgetProposal(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("candidateID")
	receipt, err := h.store.HardForgetProposal(op, id)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, receipt)
}

// ClaimDeferred converts a deferred receipt into a staged candidate.
func (h *PeopleMemoryHandler) ClaimDeferred(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("receiptID")
	var body struct {
		Requester string `json:"requester_dog"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Requester == "" {
		body.Requester = op
	}
	c, err := h.store.ClaimDeferredReceipt(op, id, body.Requester)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, c)
}

// WithdrawReceipt withdraws a deferred receipt before it is claimed.
func (h *PeopleMemoryHandler) WithdrawReceipt(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("receiptID")
	if err := h.store.WithdrawReceipt(op, id); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "withdrawn", "receipt_id": id})
}

// ForgetReceipt hard-forgets a deferred receipt.
func (h *PeopleMemoryHandler) ForgetReceipt(w http.ResponseWriter, r *http.Request) {
	op := h.resolveOperator(r)
	id := r.PathValue("receiptID")
	if err := h.store.ForgetReceipt(op, id); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "forgotten", "receipt_id": id})
}
