package settings

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// This file holds the document-level mutations on ONE operator's
// peopleMemoryDocument. Both the file store and the Redis store reuse these
// exact methods; only the persistence wrapper differs.

// reindexPending rebuilds the per-person pending candidate index from scratch.
// A candidate is "pending" (still actionable in the owner-visible list) when its
// state is pending_approval / not_now / partially_materialized.
func (d *peopleMemoryDocument) reindexPending() {
	idx := map[string][]string{}
	for id, c := range d.Candidates {
		switch c.State {
		case CandPendingApproval, CandNotNow, CandPartiallyMaterialized:
			pid := c.TargetPersonID
			if pid == "" && c.PersonDraft != nil {
				pid = id // pre-materialization placeholder key
			}
			if pid == "" {
				pid = "unbound"
			}
			idx[pid] = append(idx[pid], id)
		}
	}
	_ = idx
}

// propose stages a capture candidate. It does NOT materialize anything — only an
// explicit approveDrafts promotes drafts to canonical truth.
func (d *peopleMemoryDocument) propose(c *CaptureCandidate) (*CaptureCandidate, error) {
	if len(c.ClaimDrafts) == 0 && c.RelationshipDraft == nil && c.InteractionDraft == nil && c.PersonDraft == nil {
		return nil, fmt.Errorf("a candidate must carry at least one draft or a person draft")
	}
	if len(c.ClaimDrafts) > 3 {
		return nil, fmt.Errorf("a candidate may present at most 3 claim drafts")
	}
	now := peopleNowMs()
	if c.CandidateID == "" {
		c.CandidateID = "cand-" + uuid.NewString()[:12]
	}
	c.State = CandPendingApproval
	c.PresentedAt = now
	c.CreatedAt = now
	for i := range c.ClaimDrafts {
		if c.ClaimDrafts[i].Decision == "" {
			c.ClaimDrafts[i].Decision = "pending"
		}
	}
	if c.RelationshipDraft != nil && c.RelationshipDraft.Decision == "" {
		c.RelationshipDraft.Decision = "pending"
	}
	if c.InteractionDraft != nil && c.InteractionDraft.Decision == "" {
		c.InteractionDraft.Decision = "pending"
	}
	d.Candidates[c.CandidateID] = c
	d.reindexPending()
	return c, nil
}

// listPending returns actionable candidates, newest first, capped at limit (0 = all).
func (d *peopleMemoryDocument) listPending(limit int) []*CaptureCandidate {
	out := make([]*CaptureCandidate, 0)
	for _, c := range d.Candidates {
		switch c.State {
		case CandPendingApproval, CandNotNow, CandPartiallyMaterialized:
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// approveDrafts materializes the selected draft ids of a candidate.
func (d *peopleMemoryDocument) approveDrafts(candidateID string, draftIDs []string) (*PersonMemoryDecisionReceipt, error) {
	c, ok := d.Candidates[candidateID]
	if !ok {
		return nil, fmt.Errorf("candidate %q not found", candidateID)
	}
	known := map[string]bool{}
	for _, dr := range c.ClaimDrafts {
		known[dr.DraftID] = true
	}
	if c.RelationshipDraft != nil {
		known[c.RelationshipDraft.DraftID] = true
	}
	if c.InteractionDraft != nil {
		known[c.InteractionDraft.DraftID] = true
	}
	approveSet := map[string]bool{}
	for _, id := range draftIDs {
		if !known[id] {
			return nil, fmt.Errorf("draft %q does not belong to candidate %q", id, candidateID)
		}
		approveSet[id] = true
	}
	if len(approveSet) == 0 {
		return nil, fmt.Errorf("no draft ids selected for approval")
	}

	personID := c.TargetPersonID
	if personID == "" && c.PersonDraft != nil {
		personID = "person-" + uuid.NewString()[:12]
		d.People[personID] = &PersonIdentity{
			PersonID:       personID,
			DisplayName:    c.PersonDraft.DisplayName,
			PrivateAliases: c.PersonDraft.PrivateAliases,
			Status:         PersonStatusActive,
			CreatedAt:      peopleNowMs(),
			SourceRefs:     []SourceRef{c.SourceMessageRef},
		}
		if c.PersonDraft.WorkspaceEntityLink != nil {
			link := *c.PersonDraft.WorkspaceEntityLink
			link.CheckedAt = peopleNowMs()
			d.People[personID].WorkspaceEntityLink = &link
		}
	}
	if personID == "" {
		return nil, fmt.Errorf("candidate has no target person and no person draft")
	}
	if _, ok := d.People[personID]; !ok {
		return nil, fmt.Errorf("target person %q does not exist", personID)
	}
	relID := "rel-" + personID

	receipt := &PersonMemoryDecisionReceipt{
		DecisionID:       "dec-" + uuid.NewString()[:12],
		CandidateID:      candidateID,
		PersonID:         personID,
		SelectedDraftIDs: draftIDs,
		DecidedAt:        peopleNowMs(),
	}
	src := []SourceRef{c.SourceMessageRef}

	for i := range c.ClaimDrafts {
		dr := &c.ClaimDrafts[i]
		if !approveSet[dr.DraftID] {
			continue
		}
		if dr.Payload.Kind == ClaimKindAgentInference {
			return nil, fmt.Errorf("agent_inference cannot be materialized as truth (draft %q)", dr.DraftID)
		}
		cv := &PersonClaimVersion{
			ClaimID:    "claim-" + uuid.NewString()[:12],
			PersonID:   personID,
			Payload:    dr.Payload,
			Status:     ClaimStatusCurrent,
			RecordedAt: peopleNowMs(),
			SourceRefs: src,
		}
		key := claimKey(dr.Payload)
		for _, old := range d.Claims[personID] {
			if old.Status == ClaimStatusCurrent && claimKey(old.Payload) == key {
				old.Status = ClaimStatusSuperseded
				cv.SupersedesClaimID = old.ClaimID
				receipt.RestoredClaimIDs = append(receipt.RestoredClaimIDs, old.ClaimID)
			}
		}
		d.Claims[personID] = append(d.Claims[personID], cv)
		receipt.MaterializedClaimIDs = append(receipt.MaterializedClaimIDs, cv.ClaimID)
		dr.Decision = "approved"
	}

	if c.RelationshipDraft != nil && approveSet[c.RelationshipDraft.DraftID] {
		rel := d.upsertRelationship(personID, relID, c.RelationshipDraft.Status, src)
		receipt.CreatedRelationship = rel
		c.RelationshipDraft.Decision = "approved"
	}

	if c.InteractionDraft != nil && approveSet[c.InteractionDraft.DraftID] {
		ev := &InteractionEvent{
			EventID:           "ev-" + uuid.NewString()[:12],
			RelationshipID:    relID,
			OccurredAt:        c.InteractionDraft.OccurredAt,
			Duration:          c.InteractionDraft.Duration,
			EventKind:         c.InteractionDraft.EventKind,
			Headline:          c.InteractionDraft.Headline,
			ImportanceOrTopic: c.InteractionDraft.ImportanceOrTopic,
			UncertaintyNotes:  c.InteractionDraft.UncertaintyNotes,
			SourceRefs:        src,
			Status:            EventStatusActive,
			RecordedAt:        peopleNowMs(),
		}
		d.Events[personID] = append(d.Events[personID], ev)
		receipt.MaterializedEventIDs = append(receipt.MaterializedEventIDs, ev.EventID)
		c.InteractionDraft.Decision = "approved"
	}

	remaining := make([]string, 0)
	for _, dr := range c.ClaimDrafts {
		if dr.Decision != "approved" {
			remaining = append(remaining, dr.DraftID)
		}
	}
	if c.RelationshipDraft != nil && c.RelationshipDraft.Decision != "approved" {
		remaining = append(remaining, c.RelationshipDraft.DraftID)
	}
	if c.InteractionDraft != nil && c.InteractionDraft.Decision != "approved" {
		remaining = append(remaining, c.InteractionDraft.DraftID)
	}
	c.RemainingDraftIDs = remaining
	if len(remaining) == 0 {
		c.State = CandMaterialized
	} else {
		c.State = CandPartiallyMaterialized
	}
	receipt.RemainingDraftIDs = remaining
	c.DecisionRefs = append(c.DecisionRefs, receipt.DecisionID)
	d.Decisions[receipt.DecisionID] = receipt
	d.reindexPending()
	return receipt, nil
}

// rejectDrafts marks the given drafts as rejected — dropped, never
// materialized — individually. This is the per-card "reject"
// of a profile-update action: each draft decides on its own. When every draft
// is decided (approved or rejected) the candidate resolves. Nothing is
// materialized here; only the per-draft decision flag and the candidate state
// change (fail-closed: rejected drafts never become truth).
func (d *peopleMemoryDocument) rejectDrafts(candidateID string, draftIDs []string) (*CaptureCandidate, error) {
	c, ok := d.Candidates[candidateID]
	if !ok {
		return nil, fmt.Errorf("candidate %q not found", candidateID)
	}
	known := map[string]bool{}
	for _, dr := range c.ClaimDrafts {
		known[dr.DraftID] = true
	}
	if c.RelationshipDraft != nil {
		known[c.RelationshipDraft.DraftID] = true
	}
	if c.InteractionDraft != nil {
		known[c.InteractionDraft.DraftID] = true
	}
	rejectSet := map[string]bool{}
	for _, id := range draftIDs {
		if !known[id] {
			return nil, fmt.Errorf("draft %q does not belong to candidate %q", id, candidateID)
		}
		rejectSet[id] = true
	}
	if len(rejectSet) == 0 {
		return nil, fmt.Errorf("no draft ids selected for rejection")
	}
	for i := range c.ClaimDrafts {
		if rejectSet[c.ClaimDrafts[i].DraftID] {
			c.ClaimDrafts[i].Decision = "rejected"
		}
	}
	if c.RelationshipDraft != nil && rejectSet[c.RelationshipDraft.DraftID] {
		c.RelationshipDraft.Decision = "rejected"
	}
	if c.InteractionDraft != nil && rejectSet[c.InteractionDraft.DraftID] {
		c.InteractionDraft.Decision = "rejected"
	}

	remaining := make([]string, 0)
	anyApproved := false
	for _, dr := range c.ClaimDrafts {
		if dr.Decision == "approved" {
			anyApproved = true
		}
		if dr.Decision != "approved" && dr.Decision != "rejected" {
			remaining = append(remaining, dr.DraftID)
		}
	}
	if c.RelationshipDraft != nil {
		if c.RelationshipDraft.Decision == "approved" {
			anyApproved = true
		}
		if c.RelationshipDraft.Decision != "approved" && c.RelationshipDraft.Decision != "rejected" {
			remaining = append(remaining, c.RelationshipDraft.DraftID)
		}
	}
	if c.InteractionDraft != nil {
		if c.InteractionDraft.Decision == "approved" {
			anyApproved = true
		}
		if c.InteractionDraft.Decision != "approved" && c.InteractionDraft.Decision != "rejected" {
			remaining = append(remaining, c.InteractionDraft.DraftID)
		}
	}
	c.RemainingDraftIDs = remaining
	switch {
	case len(remaining) == 0 && anyApproved:
		c.State = CandMaterialized
	case len(remaining) == 0:
		c.State = CandRejected
	default:
		c.State = CandPartiallyMaterialized
	}
	d.reindexPending()
	return c, nil
}

// upsertRelationship ensures exactly one relationship per person; on a new
// status it appends a transition and updates the current status.
func (d *peopleMemoryDocument) upsertRelationship(personID, relID, status string, src []SourceRef) bool {
	rels := d.Relationships[personID]
	for _, r := range rels {
		if r.RelationshipID == relID {
			r.Transitions = append(r.Transitions, RelationshipTransition{
				Status: status, RecordedAt: peopleNowMs(), SourceRefs: src,
			})
			r.Status = status
			return false
		}
	}
	now := peopleNowMs()
	rel := &PersonRelationship{
		RelationshipID: relID,
		PersonID:       personID,
		Status:         status,
		CreatedAt:      now,
		SourceRefs:     src,
		Transitions:    []RelationshipTransition{{Status: status, RecordedAt: now, SourceRefs: src}},
	}
	d.Relationships[personID] = append(rels, rel)
	return true
}

// claimKey returns a stable equality key for supersede detection.
func claimKey(p PersonClaimPayload) string {
	switch p.Kind {
	case ClaimKindReportedFact:
		return "fact:" + p.Predicate
	case ClaimKindUserAssessment:
		return "assess:" + p.Statement
	default:
		return p.Kind + ":" + p.Statement
	}
}

func (d *peopleMemoryDocument) rejectCandidate(candidateID string) (*CaptureCandidate, error) {
	c, ok := d.Candidates[candidateID]
	if !ok {
		return nil, fmt.Errorf("candidate %q not found", candidateID)
	}
	c.State = CandRejected
	d.reindexPending()
	return c, nil
}

func (d *peopleMemoryDocument) markNotNow(candidateID string) (*CaptureCandidate, error) {
	c, ok := d.Candidates[candidateID]
	if !ok {
		return nil, fmt.Errorf("candidate %q not found", candidateID)
	}
	c.State = CandNotNow
	c.NotNowAt = peopleNowMs()
	d.reindexPending()
	return c, nil
}

func (d *peopleMemoryDocument) withdrawCandidate(candidateID string) (*CaptureCandidate, error) {
	c, ok := d.Candidates[candidateID]
	if !ok {
		return nil, fmt.Errorf("candidate %q not found", candidateID)
	}
	if c.State == CandMaterialized {
		return nil, fmt.Errorf("cannot withdraw a materialized candidate (use correct/forget)")
	}
	c.State = CandWithdrawn
	d.reindexPending()
	return c, nil
}

// undoDecision reverts a prior approval.
func (d *peopleMemoryDocument) undoDecision(decisionID string) (*CaptureCandidate, error) {
	rec, ok := d.Decisions[decisionID]
	if !ok {
		return nil, fmt.Errorf("decision %q not found", decisionID)
	}
	c, ok := d.Candidates[rec.CandidateID]
	if !ok {
		return nil, fmt.Errorf("candidate %q not found", rec.CandidateID)
	}
	personID := rec.PersonID
	if len(rec.MaterializedClaimIDs) > 0 {
		kept := make([]*PersonClaimVersion, 0, len(d.Claims[personID]))
		for _, cl := range d.Claims[personID] {
			drop := false
			for _, id := range rec.MaterializedClaimIDs {
				if cl.ClaimID == id {
					drop = true
					break
				}
			}
			if !drop {
				kept = append(kept, cl)
			}
		}
		d.Claims[personID] = kept
	}
	for _, id := range rec.RestoredClaimIDs {
		for _, cl := range d.Claims[personID] {
			if cl.ClaimID == id && cl.Status == ClaimStatusSuperseded {
				cl.Status = ClaimStatusCurrent
				cl.SupersedesClaimID = ""
			}
		}
	}
	if len(rec.MaterializedEventIDs) > 0 {
		kept := make([]*InteractionEvent, 0, len(d.Events[personID]))
		for _, ev := range d.Events[personID] {
			drop := false
			for _, id := range rec.MaterializedEventIDs {
				if ev.EventID == id {
					drop = true
					break
				}
			}
			if !drop {
				kept = append(kept, ev)
			}
		}
		d.Events[personID] = kept
	}
	if rec.CreatedRelationship {
		delete(d.Relationships, personID)
	} else if rec.RelationshipStatus != "" {
		for _, r := range d.Relationships[personID] {
			if len(r.Transitions) > 1 {
				r.Transitions = r.Transitions[:len(r.Transitions)-1]
				r.Status = r.Transitions[len(r.Transitions)-1].Status
			}
		}
	}
	approved := map[string]bool{}
	for _, id := range rec.SelectedDraftIDs {
		approved[id] = true
	}
	for i := range c.ClaimDrafts {
		if approved[c.ClaimDrafts[i].DraftID] {
			c.ClaimDrafts[i].Decision = "pending"
		}
	}
	if c.RelationshipDraft != nil && approved[c.RelationshipDraft.DraftID] {
		c.RelationshipDraft.Decision = "pending"
	}
	if c.InteractionDraft != nil && approved[c.InteractionDraft.DraftID] {
		c.InteractionDraft.Decision = "pending"
	}
	c.State = CandPendingApproval
	delete(d.Decisions, decisionID)
	d.reindexPending()
	return c, nil
}
