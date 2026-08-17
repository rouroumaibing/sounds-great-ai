package settings

// BroadcastingPeopleMemoryStore wraps any PeopleMemoryStore and publishes a
// PeopleMemoryEvent after every mutating call, so other clients (e.g. other
// browser tabs) can refresh in real time over SSE. Read methods and the
// embedded store's other behaviour are delegated unchanged. The hub is shared
// with the SSE handler so both ends use the same instance.
//
// Wrapping at this layer means EVERY mutation path broadcasts — including the
// daily deferred-receipt clerk (RunPeopleMemoryClerkOnce promotes receipts to
// candidates) and both the file- and Redis-backed stores.
type BroadcastingPeopleMemoryStore struct {
	PeopleMemoryStore
	hub *PeopleMemoryEventHub
}

// NewBroadcastingPeopleMemoryStore decorates inner with an event hub. Pass a
// nil hub to disable broadcasting (the store still works, it just stays silent).
func NewBroadcastingPeopleMemoryStore(inner PeopleMemoryStore, hub *PeopleMemoryEventHub) PeopleMemoryStore {
	return &BroadcastingPeopleMemoryStore{PeopleMemoryStore: inner, hub: hub}
}

func (b *BroadcastingPeopleMemoryStore) publish(ev PeopleMemoryEvent) {
	if b.hub != nil {
		b.hub.Publish(ev)
	}
}

// ---- Proposal lifecycle (object 5): candidate-scoped events ----

func (b *BroadcastingPeopleMemoryStore) Propose(operatorID string, c *CaptureCandidate) (*CaptureCandidate, error) {
	stored, err := b.PeopleMemoryStore.Propose(operatorID, c)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, CandidateID: stored.CandidateID, Type: "proposed"})
	}
	return stored, err
}

func (b *BroadcastingPeopleMemoryStore) ApproveDrafts(operatorID, candidateID string, draftIDs []string) (*PersonMemoryDecisionReceipt, error) {
	rec, err := b.PeopleMemoryStore.ApproveDrafts(operatorID, candidateID, draftIDs)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, CandidateID: candidateID, Type: "candidate_updated"})
	}
	return rec, err
}

func (b *BroadcastingPeopleMemoryStore) RejectDrafts(operatorID, candidateID string, draftIDs []string) (*CaptureCandidate, error) {
	c, err := b.PeopleMemoryStore.RejectDrafts(operatorID, candidateID, draftIDs)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, CandidateID: candidateID, Type: "candidate_updated"})
	}
	return c, err
}

func (b *BroadcastingPeopleMemoryStore) RejectCandidate(operatorID, candidateID string) (*CaptureCandidate, error) {
	c, err := b.PeopleMemoryStore.RejectCandidate(operatorID, candidateID)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, CandidateID: candidateID, Type: "candidate_updated"})
	}
	return c, err
}

func (b *BroadcastingPeopleMemoryStore) MarkNotNow(operatorID, candidateID string) (*CaptureCandidate, error) {
	c, err := b.PeopleMemoryStore.MarkNotNow(operatorID, candidateID)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, CandidateID: candidateID, Type: "candidate_updated"})
	}
	return c, err
}

func (b *BroadcastingPeopleMemoryStore) WithdrawCandidate(operatorID, candidateID string) (*CaptureCandidate, error) {
	c, err := b.PeopleMemoryStore.WithdrawCandidate(operatorID, candidateID)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, CandidateID: candidateID, Type: "candidate_updated"})
	}
	return c, err
}

func (b *BroadcastingPeopleMemoryStore) UndoDecision(operatorID, decisionID string) (*CaptureCandidate, error) {
	c, err := b.PeopleMemoryStore.UndoDecision(operatorID, decisionID)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, CandidateID: c.CandidateID, Type: "candidate_updated"})
	}
	return c, err
}

func (b *BroadcastingPeopleMemoryStore) HardForgetProposal(operatorID, proposalID string) (*PersonMemoryDeletionReceipt, error) {
	rec, err := b.PeopleMemoryStore.HardForgetProposal(operatorID, proposalID)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, Type: "candidate_updated"})
	}
	return rec, err
}

// ---- Lifecycle mutations on materialized truth: person-scoped events ----

func (b *BroadcastingPeopleMemoryStore) CorrectClaim(operatorID, personID, expectedCurrentClaimID string, payload PersonClaimPayload, src SourceRef) (*PersonClaimVersion, error) {
	cv, err := b.PeopleMemoryStore.CorrectClaim(operatorID, personID, expectedCurrentClaimID, payload, src)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, PersonID: personID, Type: "person_updated"})
	}
	return cv, err
}

func (b *BroadcastingPeopleMemoryStore) RetireClaim(operatorID, personID, expectedCurrentClaimID string, src SourceRef) error {
	err := b.PeopleMemoryStore.RetireClaim(operatorID, personID, expectedCurrentClaimID, src)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, PersonID: personID, Type: "person_updated"})
	}
	return err
}

func (b *BroadcastingPeopleMemoryStore) AmendInteraction(operatorID, personID, expectedEventID string, payload CandidateInteractionDraft, src SourceRef) (*InteractionEvent, error) {
	ev, err := b.PeopleMemoryStore.AmendInteraction(operatorID, personID, expectedEventID, payload, src)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, PersonID: personID, Type: "person_updated"})
	}
	return ev, err
}

func (b *BroadcastingPeopleMemoryStore) RedactItem(operatorID, personID string, item RedactTarget) error {
	err := b.PeopleMemoryStore.RedactItem(operatorID, personID, item)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, PersonID: personID, Type: "person_updated"})
	}
	return err
}

func (b *BroadcastingPeopleMemoryStore) HardForget(operatorID, personID string) (*PersonMemoryDeletionReceipt, error) {
	rec, err := b.PeopleMemoryStore.HardForget(operatorID, personID)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, PersonID: personID, Type: "person_updated"})
	}
	return rec, err
}

// ---- Dual-path deferred receipts ----

func (b *BroadcastingPeopleMemoryStore) DeferReceipt(operatorID, requesterDog, subject, personID string, coords []SourceRef) (*DeferredPersonMemoryReceipt, error) {
	r, err := b.PeopleMemoryStore.DeferReceipt(operatorID, requesterDog, subject, personID, coords)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, Type: "deferred_updated"})
	}
	return r, err
}

func (b *BroadcastingPeopleMemoryStore) ClaimDeferredReceipt(operatorID, receiptID, requesterDog string) (*CaptureCandidate, error) {
	c, err := b.PeopleMemoryStore.ClaimDeferredReceipt(operatorID, receiptID, requesterDog)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, CandidateID: c.CandidateID, Type: "deferred_updated"})
	}
	return c, err
}

func (b *BroadcastingPeopleMemoryStore) ReserveDeferredReceipt(operatorID, receiptID, by string) error {
	err := b.PeopleMemoryStore.ReserveDeferredReceipt(operatorID, receiptID, by)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, Type: "deferred_updated"})
	}
	return err
}

func (b *BroadcastingPeopleMemoryStore) ReleaseDeferredReceipt(operatorID, receiptID string) error {
	err := b.PeopleMemoryStore.ReleaseDeferredReceipt(operatorID, receiptID)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, Type: "deferred_updated"})
	}
	return err
}

func (b *BroadcastingPeopleMemoryStore) WithdrawReceipt(operatorID, receiptID string) error {
	err := b.PeopleMemoryStore.WithdrawReceipt(operatorID, receiptID)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, Type: "deferred_updated"})
	}
	return err
}

func (b *BroadcastingPeopleMemoryStore) ForgetReceipt(operatorID, receiptID string) error {
	err := b.PeopleMemoryStore.ForgetReceipt(operatorID, receiptID)
	if err == nil {
		b.publish(PeopleMemoryEvent{OperatorID: operatorID, Type: "deferred_updated"})
	}
	return err
}
