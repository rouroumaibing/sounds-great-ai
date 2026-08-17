package settings

import (
	"context"
	"strings"
	"testing"
)

// TestRunPeopleMemoryClerkReinvokeProposes verifies the faithful F276 clerk:
// with an Invoke func it re-invokes the original dog, parses the returned
// proposal JSON, and persists a rejectable candidate linked to the receipt.
// The receipt is reserved (not re-listed) afterwards.
func TestRunPeopleMemoryClerkReinvokeProposes(t *testing.T) {
	store := NewFilePeopleMemoryStore(t.TempDir())
	op := "operator"
	rcpt, err := store.DeferReceipt(op, "claude", "Alice likes tea", "", []SourceRef{
		{ThreadID: "t1", MessageID: "m1", Excerpt: "Alice likes tea"},
	})
	if err != nil {
		t.Fatalf("defer: %v", err)
	}

	invoke := func(_ context.Context, clientID, prompt, _ string) (string, error) {
		if !strings.Contains(prompt, "Alice likes tea") {
			t.Errorf("prompt did not carry resolved source text: %q", prompt)
		}
		return `{"display_name":"Alice","aliases":["Al"],"claim_drafts":[{"kind":"reported_fact","predicate":"likes tea","confidence":"high"}],"relationship_draft":{"status":"current","line":"friend"}}`, nil
	}
	resolve := func(_ string, ref SourceRef) (string, bool) {
		return ref.Excerpt, true
	}
	deps := PeopleMemoryClerkDeps{Invoke: invoke, ResolveSource: resolve, DefaultClientID: "claude", WorkDir: "/tmp"}

	RunPeopleMemoryClerkOnce(context.Background(), store, deps)

	if ready, _ := store.ListReadyDeferred(op); len(ready) != 0 {
		t.Fatalf("receipt should be reserved, %d still ready", len(ready))
	}
	pending, _ := store.ListPending(op, 10)
	if len(pending) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(pending))
	}
	c := pending[0]
	if c.DeferredReceiptID != rcpt.ReceiptID {
		t.Errorf("candidate not linked to receipt: %q", c.DeferredReceiptID)
	}
	if c.State != CandPendingApproval {
		t.Errorf("candidate state = %q, want pending_approval", c.State)
	}
	if c.PersonDraft == nil || c.PersonDraft.DisplayName != "Alice" {
		t.Errorf("person draft missing/incorrect: %+v", c.PersonDraft)
	}
	if len(c.ClaimDrafts) != 1 || c.ClaimDrafts[0].Payload.Kind != "reported_fact" {
		t.Errorf("claim draft not mapped: %+v", c.ClaimDrafts)
	}
	if c.RelationshipDraft == nil || c.RelationshipDraft.Status != "current" {
		t.Errorf("relationship draft not mapped: %+v", c.RelationshipDraft)
	}
}

// TestRunPeopleMemoryClerkReinvokeDefers verifies that when the dog returns
// {"defer":true} (insufficient evidence) the receipt is released and remains
// ready for a future run — no candidate is created.
func TestRunPeopleMemoryClerkReinvokeDefers(t *testing.T) {
	store := NewFilePeopleMemoryStore(t.TempDir())
	op := "operator"
	if _, err := store.DeferReceipt(op, "claude", "subj", "", []SourceRef{{ThreadID: "t1", MessageID: "m1"}}); err != nil {
		t.Fatal(err)
	}
	deps := PeopleMemoryClerkDeps{
		Invoke:          func(_ context.Context, _, _, _ string) (string, error) { return `{"defer":true}`, nil },
		ResolveSource:   func(_ string, _ SourceRef) (string, bool) { return "x", true },
		DefaultClientID: "claude",
	}
	RunPeopleMemoryClerkOnce(context.Background(), store, deps)

	if ready, _ := store.ListReadyDeferred(op); len(ready) != 1 {
		t.Fatalf("receipt should still be ready after defer, got %d", len(ready))
	}
	if pending, _ := store.ListPending(op, 10); len(pending) != 0 {
		t.Fatalf("defer must not create a candidate, got %d", len(pending))
	}
}

// TestRunPeopleMemoryClerkReinvokeFallback verifies that when Invoke is nil the
// clerk degrades to the original behaviour: promote the receipt to a staged
// (empty) candidate so the operator still gets a reviewable item.
func TestRunPeopleMemoryClerkReinvokeFallback(t *testing.T) {
	store := NewFilePeopleMemoryStore(t.TempDir())
	op := "operator"
	if _, err := store.DeferReceipt(op, "claude", "subj", "", []SourceRef{{ThreadID: "t1", MessageID: "m1"}}); err != nil {
		t.Fatal(err)
	}
	RunPeopleMemoryClerkOnce(context.Background(), store, PeopleMemoryClerkDeps{})

	if ready, _ := store.ListReadyDeferred(op); len(ready) != 0 {
		t.Fatalf("receipt should be promoted (not ready), %d ready", len(ready))
	}
	pending, _ := store.ListPending(op, 10)
	if len(pending) != 1 {
		t.Fatalf("expected 1 promoted candidate, got %d", len(pending))
	}
	if len(pending[0].ClaimDrafts) != 0 {
		t.Errorf("fallback candidate should be empty, got %d drafts", len(pending[0].ClaimDrafts))
	}
}

// TestRecallContextForQuery verifies the homologous anchor-first recall
// injection: a user message referencing a known person yields a bounded block,
// while an unrelated message yields nothing.
func TestRecallContextForQuery(t *testing.T) {
	store := NewFilePeopleMemoryStore(t.TempDir())
	op := "operator"
	c := &CaptureCandidate{
		CandidateID: "c1", RequesterDog: op, State: CandPendingApproval,
		PersonDraft: &PersonIdentityDraft{DisplayName: "Alice", PrivateAliases: []string{"Al"}},
		ClaimDrafts: []CandidateClaimDraft{{
			DraftID: "d1", Payload: PersonClaimPayload{Kind: "reported_fact", Predicate: "likes tea"}, Decision: "pending",
		}},
		RemainingDraftIDs: []string{"d1"}, CreatedAt: 1, PresentedAt: 1,
	}
	if _, err := store.Propose(op, c); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApproveDrafts(op, "c1", []string{"d1"}); err != nil {
		t.Fatal(err)
	}
	if people, _ := store.ListPeople(op); len(people) != 1 {
		t.Fatalf("expected 1 active person, got %d", len(people))
	}

	block, err := store.RecallContextForQuery(op, "Alice told me about the meeting")
	if err != nil {
		t.Fatal(err)
	}
	if block == "" || !strings.Contains(block, "Alice") {
		t.Fatalf("expected block referencing Alice, got %q", block)
	}
	if !strings.Contains(block, "## 关系记忆") {
		t.Errorf("block missing header: %q", block)
	}

	empty, err := store.RecallContextForQuery(op, "the weather is nice today")
	if err != nil {
		t.Fatal(err)
	}
	if empty != "" {
		t.Fatalf("unrelated query should yield no block, got %q", empty)
	}
}
