package settings

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// This file holds the lifecycle mutations on materialized truth: correction
// (versioned supersede), retirement, interaction amendment, redaction, and
// hard-forget. Every mutation is owner-authorized and recorded with provenance;
// nothing is silently overwritten (KD-7 / AC-B3..B6).

// CorrectClaim supersedes the expected current claim with a new version carrying
// the corrected payload. agent_inference payloads are rejected (AC-A3).
func (d *peopleMemoryDocument) correctClaim(personID, expectedCurrentClaimID string, payload PersonClaimPayload, src SourceRef) (*PersonClaimVersion, error) {
	if payload.Kind == ClaimKindAgentInference {
		return nil, fmt.Errorf("agent_inference cannot be materialized as truth")
	}
	var target *PersonClaimVersion
	for _, cl := range d.Claims[personID] {
		if cl.ClaimID == expectedCurrentClaimID {
			target = cl
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("claim %q not found for person %q", expectedCurrentClaimID, personID)
	}
	if target.Status != ClaimStatusCurrent {
		return nil, fmt.Errorf("claim %q is not the current claim (status=%s)", expectedCurrentClaimID, target.Status)
	}
	target.Status = ClaimStatusSuperseded
	now := time.Now().UnixMilli()
	srcs := []SourceRef{src}
	if src.SourceKind == "" {
		srcs = target.SourceRefs
	}
	cv := &PersonClaimVersion{
		ClaimID:           "claim-" + newShortID(),
		PersonID:          personID,
		Payload:           payload,
		Status:            ClaimStatusCurrent,
		RecordedAt:        now,
		SourceRefs:        srcs,
		SupersedesClaimID: target.ClaimID,
	}
	d.Claims[personID] = append(d.Claims[personID], cv)
	d.reindexPending()
	return cv, nil
}

// RetireClaim moves a current claim to retired.
func (d *peopleMemoryDocument) retireClaim(personID, expectedCurrentClaimID string, src SourceRef) error {
	for _, cl := range d.Claims[personID] {
		if cl.ClaimID == expectedCurrentClaimID {
			if cl.Status != ClaimStatusCurrent {
				return fmt.Errorf("claim %q is not current (status=%s)", expectedCurrentClaimID, cl.Status)
			}
			cl.Status = ClaimStatusRetired
			if src.SourceKind != "" {
				cl.SourceRefs = append(cl.SourceRefs, src)
			}
			d.reindexPending()
			return nil
		}
	}
	return fmt.Errorf("claim %q not found for person %q", expectedCurrentClaimID, personID)
}

// AmendInteraction creates a new event that amends a prior one.
func (d *peopleMemoryDocument) amendInteraction(personID, expectedEventID string, payload CandidateInteractionDraft, src SourceRef) (*InteractionEvent, error) {
	var relID string
	for _, ev := range d.Events[personID] {
		if ev.EventID == expectedEventID {
			relID = ev.RelationshipID
			break
		}
	}
	if relID == "" {
		return nil, fmt.Errorf("event %q not found for person %q", expectedEventID, personID)
	}
	now := time.Now().UnixMilli()
	srcs := []SourceRef{src}
	if src.SourceKind == "" {
		srcs = nil
	}
	ev := &InteractionEvent{
		EventID:           "ev-" + newShortID(),
		RelationshipID:    relID,
		OccurredAt:        payload.OccurredAt,
		Duration:          payload.Duration,
		EventKind:         payload.EventKind,
		Headline:          payload.Headline,
		ImportanceOrTopic: payload.ImportanceOrTopic,
		UncertaintyNotes:  payload.UncertaintyNotes,
		SourceRefs:        srcs,
		AmendsEventID:     expectedEventID,
		Status:            EventStatusActive,
		RecordedAt:        now,
	}
	d.Events[personID] = append(d.Events[personID], ev)
	d.reindexPending()
	return ev, nil
}

// RedactItem purges the payload and source refs of a claim or event (AC-B6).
func (d *peopleMemoryDocument) redactItem(personID string, item RedactTarget) error {
	switch item.Kind {
	case "claim":
		for _, cl := range d.Claims[personID] {
			if cl.ClaimID == item.ID {
				cl.Status = ClaimStatusRedacted
				cl.Payload = PersonClaimPayload{Kind: ClaimKindRedacted}
				cl.SourceRefs = nil
				d.reindexPending()
				return nil
			}
		}
	case "event":
		for _, ev := range d.Events[personID] {
			if ev.EventID == item.ID {
				ev.Status = EventStatusRedacted
				ev.SourceRefs = nil
				ev.Headline = ""
				ev.ImportanceOrTopic = ""
				ev.UncertaintyNotes = nil
				d.reindexPending()
				return nil
			}
		}
	default:
		return fmt.Errorf("unknown redact item kind %q", item.Kind)
	}
	return fmt.Errorf("%s %q not found for person %q", item.Kind, item.ID, personID)
}

// HardForget purges a person entirely: identity, claims, relationships, events,
// and any bound candidates/receipts. Returns a content-free deletion receipt.
func (d *peopleMemoryDocument) hardForget(personID string) (*PersonMemoryDeletionReceipt, error) {
	counts := map[string]int{
		"people":       0,
		"claims":       0,
		"relationships":0,
		"events":       0,
		"candidates":   0,
		"receipts":     0,
	}
	if _, ok := d.People[personID]; !ok {
		return &PersonMemoryDeletionReceipt{
			RequestID: "forget-" + newShortID(),
			PersonID:  personID,
			Verdict:   "already_absent",
			Counts:    counts,
		}, nil
	}
	counts["people"] = 1
	counts["claims"] = len(d.Claims[personID])
	counts["events"] = len(d.Events[personID])
	if rels := d.Relationships[personID]; rels != nil {
		counts["relationships"] = len(rels)
	}
	delete(d.People, personID)
	delete(d.Claims, personID)
	delete(d.Events, personID)
	delete(d.Relationships, personID)
	for id, c := range d.Candidates {
		bound := c.TargetPersonID == personID ||
			(c.PersonDraft != nil && c.PersonDraft.DisplayName != "" && personDisplayNameMatches(d.People, personID, c))
		if bound {
			delete(d.Candidates, id)
			counts["candidates"]++
		}
	}
	for id, r := range d.Receipts {
		if r.PersonID == personID {
			delete(d.Receipts, id)
			counts["receipts"]++
		}
	}
	d.reindexPending()
	return &PersonMemoryDeletionReceipt{
		RequestID: "forget-" + newShortID(),
		PersonID:  personID,
		Verdict:   "purged",
		Counts:    counts,
	}, nil
}

// personDisplayNameMatches is a fail-closed helper: a candidate is only "bound"
// to a person by its explicit TargetPersonID. Pre-materialization candidates
// carry only a PersonDraft and are NOT hard-forgotten via a person id.
func personDisplayNameMatches(_ map[string]*PersonIdentity, _ string, _ *CaptureCandidate) bool {
	return false
}

// HardForgetProposal purges an unbound terminal candidate by its exact id.
func (d *peopleMemoryDocument) hardForgetProposal(proposalID string) (*PersonMemoryDeletionReceipt, error) {
	c, ok := d.Candidates[proposalID]
	if !ok {
		return &PersonMemoryDeletionReceipt{
			RequestID: "forget-" + newShortID(),
			Verdict:   "already_absent",
			Counts:    map[string]int{},
		}, nil
	}
	if c.TargetPersonID != "" || (c.PersonDraft != nil && c.PersonDraft.DisplayName != "") {
		return nil, fmt.Errorf("proposal %q is person-bound; forget the whole person relationship instead", proposalID)
	}
	delete(d.Candidates, proposalID)
	d.reindexPending()
	return &PersonMemoryDeletionReceipt{
		RequestID:  "forget-" + newShortID(),
		ProposalID: proposalID,
		Verdict:    "purged",
		Counts:     map[string]int{"candidates": 1},
	}, nil
}

// PersonMemoryDeletionReceipt is the content-free proof of a forget (no payload).
type PersonMemoryDeletionReceipt struct {
	RequestID  string         `json:"request_id"`
	PersonID   string         `json:"person_id,omitempty"`
	ProposalID string         `json:"proposal_id,omitempty"`
	Verdict    string         `json:"verdict"` // purged | already_absent
	Counts     map[string]int `json:"counts"`
}

// newShortID returns a short random id suffix.
func newShortID() string {
	return uuid.NewString()[:12]
}
