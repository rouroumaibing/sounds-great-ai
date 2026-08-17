package sop

import (
	"testing"
	"time"
)

func TestSelectReviewerDifferentBreed(t *testing.T) {
	policy := ReviewPolicy{RequireDifferentBreed: true}
	reviewer := SelectReviewer("bianmu", []string{"xigou", "jinmao"}, policy)
	if reviewer == "bianmu" {
		t.Fatal("reviewer must be different breed")
	}
	if reviewer == "" {
		t.Fatal("expected non-empty reviewer")
	}
}

func TestSelectReviewerNoCandidates(t *testing.T) {
	policy := ReviewPolicy{RequireDifferentBreed: true}
	reviewer := SelectReviewer("bianmu", []string{}, policy)
	if reviewer != "" {
		t.Fatalf("expected empty, got %s", reviewer)
	}
}

func TestSelectReviewerFromBreedsCrossBreed(t *testing.T) {
	author := BreedInfo{ID: "bianmu", CLI: "claude", CannotReviewSelf: true, CrossBreedPreferred: true, CanReview: []string{"xigou", "demu"}}
	candidates := []BreedInfo{
		{ID: "xigou", CLI: "codex", Available: true, Roles: []string{"reviewer"}},
		{ID: "demu", CLI: "gemini", Available: true, Roles: []string{"diagnostic"}},
	}
	policy := ReviewPolicy{RequireDifferentBreed: true}
	reviewer, err := SelectReviewerFromBreeds(author, candidates, policy)
	if err != nil {
		t.Fatalf("expected reviewer, got error: %v", err)
	}
	if reviewer.ID == "bianmu" {
		t.Error("reviewer must be different breed")
	}
}

func TestSelectReviewerFromBreedsExcludeUnavailable(t *testing.T) {
	author := BreedInfo{ID: "bianmu", CLI: "claude", CannotReviewSelf: true}
	candidates := []BreedInfo{
		{ID: "xigou", CLI: "codex", Available: false},
		{ID: "demu", CLI: "gemini", Available: true},
	}
	policy := ReviewPolicy{RequireDifferentBreed: true, ExcludeUnavailable: true}
	reviewer, err := SelectReviewerFromBreeds(author, candidates, policy)
	if err != nil {
		t.Fatalf("expected reviewer, got error: %v", err)
	}
	if reviewer.ID != "demu" {
		t.Errorf("expected demu (only available), got %s", reviewer.ID)
	}
}

func TestSelectReviewerFromBreedsDifferentCLI(t *testing.T) {
	author := BreedInfo{ID: "bianmu", CLI: "claude", CannotReviewSelf: true}
	candidates := []BreedInfo{
		{ID: "bianmu", CLI: "codex", Available: true}, // same breed, different CLI
		{ID: "xigou", CLI: "claude", Available: true}, // different breed, same CLI
		{ID: "demu", CLI: "gemini", Available: true},  // different breed, different CLI
	}
	policy := ReviewPolicy{RequireDifferentBreed: true, RequireDifferentCLI: true}
	reviewer, err := SelectReviewerFromBreeds(author, candidates, policy)
	if err != nil {
		t.Fatalf("expected reviewer, got error: %v", err)
	}
	if reviewer.CLI == "claude" {
		t.Error("reviewer must have different CLI")
	}
	if reviewer.ID == "bianmu" {
		t.Error("reviewer must be different breed")
	}
}

func TestSelectReviewerFromBreedsCanReview(t *testing.T) {
	author := BreedInfo{ID: "bianmu", CLI: "claude", CannotReviewSelf: true, CanReview: []string{"xigou"}}
	candidates := []BreedInfo{
		{ID: "jinmao", CLI: "gemini", Available: true},
		{ID: "xigou", CLI: "codex", Available: true},
	}
	policy := ReviewPolicy{RequireDifferentBreed: true}
	reviewer, err := SelectReviewerFromBreeds(author, candidates, policy)
	if err != nil {
		t.Fatalf("expected reviewer, got error: %v", err)
	}
	if reviewer.ID != "xigou" {
		t.Errorf("expected xigou (in can_review), got %s", reviewer.ID)
	}
}

func TestSelectReviewerFromBreedsNoneAvailable(t *testing.T) {
	author := BreedInfo{ID: "bianmu", CLI: "claude", CannotReviewSelf: true}
	candidates := []BreedInfo{
		{ID: "bianmu", CLI: "claude", Available: true},
	}
	policy := ReviewPolicy{RequireDifferentBreed: true}
	_, err := SelectReviewerFromBreeds(author, candidates, policy)
	if err == nil {
		t.Error("expected error when no reviewer available")
	}
}

func TestReviewCycleRequestAndReceive(t *testing.T) {
	rc := NewReviewCycle()
	rc.RequestReview(ReviewRequest{
		FromBreed:   "bianmu",
		ToBreed:     "xigou",
		ArtifactSHA: "abc123",
		Message:     "please review",
	})
	rc.ReceiveReview(ReviewProvenance{
		ReviewerID:    "xigou",
		ReviewerBreed: "xigou",
		ReviewSHA:     "abc123",
		Status:        "approved",
	})
	if !rc.HasReviewForSHA("abc123") {
		t.Error("expected review for SHA abc123")
	}
}

func TestReviewCycleCrossBreedCheck(t *testing.T) {
	rc := NewReviewCycle()
	rc.ReceiveReview(ReviewProvenance{
		ReviewerID:    "xigou",
		ReviewerBreed: "xigou",
		ReviewerDogID: "xigou-dog",
		ReviewSHA:     "abc123",
		Timestamp:     time.Now(),
	})
	if !rc.IsCrossBreedReview("abc123", "bianmu") {
		t.Error("expected cross-breed review")
	}
	if rc.IsCrossBreedReview("abc123", "xigou-dog") {
		t.Error("same dog identity should not be cross-breed")
	}
}

func TestReviewCycleSelfReviewCheck(t *testing.T) {
	rc := NewReviewCycle()
	rc.ReceiveReview(ReviewProvenance{
		ReviewerID:    "bianmu",
		ReviewerBreed: "bianmu",
		ReviewerDogID: "bianmu",
		ReviewSHA:     "abc123",
		Timestamp:     time.Now(),
	})
	if !rc.IsSelfReview("abc123", "bianmu") {
		t.Error("expected self-review detection")
	}
	if rc.IsSelfReview("abc123", "xigou-dog") {
		t.Error("different dog identity should not be self-review")
	}
	if rc.IsSelfReview("missing", "bianmu") {
		t.Error("unknown SHA should not be self-review")
	}
}

func TestRecordReviewFailClosed(t *testing.T) {
	rc := NewReviewCycle()
	rc.AssignReview("bianmu", "xigou-dog", "thread-1")

	// Author may not self-review.
	if err := rc.RecordReview(ReviewProvenance{ReviewerDogID: "bianmu", ReviewSHA: "abc"}); err != ErrSelfReview {
		t.Fatalf("expected ErrSelfReview, got %v", err)
	}
	// A non-assigned dog may not write the verdict.
	if err := rc.RecordReview(ReviewProvenance{ReviewerDogID: "demu-dog", ReviewSHA: "abc"}); err != ErrWrongPrincipal {
		t.Fatalf("expected ErrWrongPrincipal, got %v", err)
	}
	// Missing identity is rejected.
	if err := rc.RecordReview(ReviewProvenance{ReviewSHA: "abc"}); err != ErrReviewNoIdentity {
		t.Fatalf("expected ErrReviewNoIdentity, got %v", err)
	}
	// The assigned reviewer succeeds (with the matching review thread).
	if err := rc.RecordReview(ReviewProvenance{ReviewerDogID: "xigou-dog", AuthorDogID: "bianmu", ReviewerThreadID: "thread-1", ReviewSHA: "abc"}); err != nil {
		t.Fatalf("assigned reviewer should succeed, got %v", err)
	}
	if !rc.IsCrossBreedReview("abc", "bianmu") {
		t.Error("expected recorded cross-breed review")
	}
}

func TestComputeReviewerDelta(t *testing.T) {
	comments := "P1-1: race condition [delta:new]\nP2-1: edge case [delta:covered]\nP3-1: refactor [delta:N/A]"
	d := ComputeReviewerDelta(comments)
	if d.New != 1 || d.Covered != 1 || d.NA != 1 {
		t.Fatalf("unexpected delta counts: %+v", d)
	}
	if d.Ratio < 0.49 || d.Ratio > 0.51 {
		t.Fatalf("expected ratio ~0.5, got %v", d.Ratio)
	}
	// [FC:new] is accepted as the equivalent annotation.
	if got := ComputeReviewerDelta("P1: bug [FC:new]"); got.New != 1 {
		t.Error("expected [FC:new] to count as new")
	}
}

func TestReviewProvenance(t *testing.T) {
	rc := NewReviewCycle()
	rc.ReceiveReview(ReviewProvenance{
		ReviewerID:    "demu",
		ReviewerBreed: "demu",
		ReviewSHA:     "def456",
		Status:        "changes_requested",
	})
	prov := rc.Provenance()
	if len(prov) != 1 {
		t.Fatalf("expected 1 provenance, got %d", len(prov))
	}
	if prov[0].ReviewerBreed != "demu" {
		t.Error("expected demu as reviewer breed")
	}
}

func TestSelectReviewPanelThreeRoles(t *testing.T) {
	author := BreedInfo{ID: "bianmu", CLI: "claude", CannotReviewSelf: true}
	candidates := []BreedInfo{
		{ID: "xigou", CLI: "codex", Available: true, Roles: []string{"reviewer"}},
		{ID: "demu", CLI: "gemini", Available: true, Roles: []string{"approver"}},
	}
	policy := ReviewPolicy{RequireDifferentBreed: true}
	panel, err := SelectReviewPanel(author, candidates, policy)
	if err != nil {
		t.Fatalf("expected panel, got error: %v", err)
	}
	if panel.Reviewer == nil || panel.FinalApprover == nil {
		t.Fatal("both reviewer and final approver must be set")
	}
	if panel.Reviewer.ID == "bianmu" || panel.FinalApprover.ID == "bianmu" {
		t.Error("neither role may be the author")
	}
	if panel.Reviewer.ID == panel.FinalApprover.ID {
		t.Error("reviewer and final approver must be distinct identities")
	}
}

func TestSelectReviewPanelNoIndependentApprover(t *testing.T) {
	author := BreedInfo{ID: "bianmu", CLI: "claude", CannotReviewSelf: true}
	candidates := []BreedInfo{
		{ID: "xigou", CLI: "codex", Available: true, Roles: []string{"reviewer"}},
	}
	policy := ReviewPolicy{RequireDifferentBreed: true}
	_, err := SelectReviewPanel(author, candidates, policy)
	if err != ErrNoFinalApproverAvailable {
		t.Fatalf("expected ErrNoFinalApproverAvailable, got %v", err)
	}
}
