package dossier

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func testStores(t *testing.T) (ObservationStore, ProposalStore, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	obs, err := NewSQLiteObservationStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	proposals, err := NewSQLiteProposalStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	return obs, proposals, dbPath
}

func TestObservationStoreRoundTrip(t *testing.T) {
	obs, _, _ := testStores(t)

	o1, err := obs.Add(AddObservationInput{DogID: "bianmu", Content: "拆解又快又准", Author: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if o1.ID == "" || o1.Provenance.Date == "" {
		t.Errorf("observation not fully populated: %+v", o1)
	}
	if _, err := obs.Add(AddObservationInput{DogID: "jinmao", Content: "检索稳", Author: "operator"}); err != nil {
		t.Fatal(err)
	}

	list, err := obs.List("bianmu", 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v", list, err)
	}
	grouped, err := obs.ListAll(10)
	if err != nil || len(grouped) != 2 {
		t.Fatalf("ListAll = %v, %v", grouped, err)
	}
	got, ok, err := obs.Get(o1.ID)
	if !ok || err != nil || got.Content != "拆解又快又准" {
		t.Fatalf("Get = %+v, %v, %v", got, ok, err)
	}
}

func validProposalInput(sourceID string) CreateProposalInput {
	return CreateProposalInput{
		SourceEvent:    SourceEventReviewComplete,
		SourceID:       sourceID,
		TargetDogID:    "bianmu",
		TargetFields:   []string{"badIntuitions"},
		BeforeSnapshot: "- **③ 坏直觉**：爱替队友做主",
		AfterDraft:     "- **③ 坏直觉**：已校准",
		Rationale:      "review 实证",
		EvidenceRefs:   []EvidenceRef{{Type: "review", ID: "t1"}},
		BaseHash:       strings.Repeat("a", 64),
		CreatedBy:      "xigou",
	}
}

func TestProposalValidationFailClosed(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*CreateProposalInput)
	}{
		{"empty evidence", func(in *CreateProposalInput) { in.EvidenceRefs = nil }},
		{"bad evidence type", func(in *CreateProposalInput) { in.EvidenceRefs = []EvidenceRef{{Type: "vibe", ID: "x"}} }},
		{"empty evidence id", func(in *CreateProposalInput) { in.EvidenceRefs = []EvidenceRef{{Type: "review"}} }},
		{"bad source event", func(in *CreateProposalInput) { in.SourceEvent = "whim" }},
		{"empty source id", func(in *CreateProposalInput) { in.SourceID = "" }},
		{"empty target", func(in *CreateProposalInput) { in.TargetDogID = "" }},
		{"empty fields", func(in *CreateProposalInput) { in.TargetFields = nil }},
		{"empty before", func(in *CreateProposalInput) { in.BeforeSnapshot = "" }},
		{"empty after", func(in *CreateProposalInput) { in.AfterDraft = "" }},
		{"empty rationale", func(in *CreateProposalInput) { in.Rationale = "" }},
		{"empty baseHash", func(in *CreateProposalInput) { in.BaseHash = "" }},
	}
	for _, tc := range cases {
		in := validProposalInput("s-" + tc.name)
		tc.mut(&in)
		if err := ValidateCreateProposal(in); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: expected ErrValidation, got %v", tc.name, err)
		}
	}
	if err := ValidateCreateProposal(validProposalInput("ok")); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
}

func TestProposalStoreIdempotencyAndStateMachine(t *testing.T) {
	_, proposals, _ := testStores(t)

	p1, created, err := proposals.Create(validProposalInput("src-1"))
	if err != nil || !created {
		t.Fatalf("create: %v %v", created, err)
	}
	p2, created, err := proposals.Create(validProposalInput("src-1"))
	if err != nil {
		t.Fatal(err)
	}
	if created || p2.ProposalID != p1.ProposalID {
		t.Errorf("idempotent create must return existing: %+v vs %+v", p1, p2)
	}

	// approved before applied; rejected can't be approved.
	if _, err := proposals.MarkApplied(p1.ProposalID, "bianmu", "sha"); !errors.Is(err, ErrProposalState) {
		t.Errorf("apply before approve must fail with state error, got %v", err)
	}
	approved, err := proposals.MarkApproved(p1.ProposalID, "operator")
	if err != nil || approved.Status != ProposalApproved || approved.ApprovedBy != "operator" {
		t.Fatalf("approve: %+v %v", approved, err)
	}
	if _, err := proposals.MarkApproved(p1.ProposalID, "operator2"); !errors.Is(err, ErrProposalState) {
		t.Errorf("double approve must fail, got %v", err)
	}
	applied, err := proposals.MarkApplied(p1.ProposalID, "bianmu", "abc123")
	if err != nil || applied.Status != ProposalApplied || applied.AppliedCommitSHA != "abc123" {
		t.Fatalf("apply: %+v %v", applied, err)
	}

	// A second proposal gets rejected with a reason.
	p3, _, err := proposals.Create(validProposalInput("src-2"))
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := proposals.MarkRejected(p3.ProposalID, "operator", "证据不足")
	if err != nil || rejected.Status != ProposalRejected || rejected.RejectReason != "证据不足" {
		t.Fatalf("reject: %+v %v", rejected, err)
	}

	pending, err := proposals.ListPending(10)
	if err != nil || len(pending) != 0 {
		t.Errorf("no pending should remain: %v %v", pending, err)
	}
	byDog, err := proposals.ListByDog("bianmu", 10)
	if err != nil || len(byDog) != 2 {
		t.Errorf("ListByDog = %v, %v", byDog, err)
	}
}

func TestCheckpointIdempotent(t *testing.T) {
	store := NewInMemoryOpportunityStore()
	cp := NewCheckpoint(store, nil)

	_, fired1 := cp.OnReviewComplete(ReviewCompleteContext{ThreadID: "t1", ReviewerDogID: "xigou", AuthorDogID: "bianmu", CommitSHA: "sha1"})
	_, fired2 := cp.OnReviewComplete(ReviewCompleteContext{ThreadID: "t1", ReviewerDogID: "xigou", AuthorDogID: "bianmu", CommitSHA: "sha1"})
	if !fired1 || fired2 {
		t.Errorf("first should fire, duplicate should not: %v %v", fired1, fired2)
	}
	// Different reviewer = new opportunity.
	_, fired3 := cp.OnReviewComplete(ReviewCompleteContext{ThreadID: "t1", ReviewerDogID: "demu", AuthorDogID: "bianmu", CommitSHA: "sha1"})
	if !fired3 {
		t.Error("different reviewer must fire a new opportunity")
	}

	pending := store.ListPending()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}
	if pending[0].TargetDogID != "bianmu" {
		t.Errorf("target must be the reviewed author, got %s", pending[0].TargetDogID)
	}

	if !store.Dismiss(pending[0].OpportunityID) {
		t.Error("dismiss should succeed")
	}
	if store.Dismiss(pending[0].OpportunityID) {
		t.Error("double dismiss should fail")
	}
	if !store.MarkConverted(pending[1].OpportunityID, "dsp_x") {
		t.Error("convert should succeed")
	}
}
