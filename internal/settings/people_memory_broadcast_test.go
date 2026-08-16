package settings

import (
	"testing"
	"time"
)

// TestBroadcastingStorePublishesOnMutation verifies the decorator emits a
// PeopleMemoryEvent after a mutating call, scoped to the right operator and
// carrying the candidate id. Reads must NOT broadcast (delegated, no override).
func TestBroadcastingStorePublishesOnMutation(t *testing.T) {
	inner := NewFilePeopleMemoryStore(t.TempDir())
	hub := NewPeopleMemoryEventHub()
	store := NewBroadcastingPeopleMemoryStore(inner, hub)

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	c := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "Alice"},
		ClaimDrafts: []CandidateClaimDraft{
			{DraftID: "d1", Payload: PersonClaimPayload{Kind: "reported_fact", Predicate: "role", Value: "pm"}, Decision: "pending"},
		},
	}
	stored, err := store.Propose("op1", c)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	select {
	case ev := <-ch:
		if ev.OperatorID != "op1" || ev.CandidateID != stored.CandidateID || ev.Type != "proposed" {
			t.Fatalf("unexpected propose event: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event published for Propose")
	}

	if _, err := store.ApproveDrafts("op1", stored.CandidateID, []string{"d1"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	select {
	case ev := <-ch:
		if ev.Type != "candidate_updated" || ev.CandidateID != stored.CandidateID {
			t.Fatalf("unexpected approve event: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event published for ApproveDrafts")
	}

	// A read must not broadcast.
	if _, _, err := store.GetCandidate("op1", stored.CandidateID); err != nil {
		t.Fatalf("get candidate: %v", err)
	}
	select {
	case ev := <-ch:
		t.Fatalf("reads must not broadcast, got: %+v", ev)
	case <-time.After(200 * time.Millisecond):
		// expected: no event
	}
}

// TestBroadcastingStoreNilHubIsSilent verifies a nil hub never panics and the
// wrapped store still behaves normally.
func TestBroadcastingStoreNilHubIsSilent(t *testing.T) {
	store := NewBroadcastingPeopleMemoryStore(NewFilePeopleMemoryStore(t.TempDir()), nil)
	c := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "Bob"},
		ClaimDrafts: []CandidateClaimDraft{{DraftID: "d1", Decision: "pending"}},
	}
	if _, err := store.Propose("opX", c); err != nil {
		t.Fatalf("propose with nil hub: %v", err)
	}
}
