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
		ReviewSHA:     "abc123",
		Timestamp:     time.Now(),
	})
	if !rc.IsCrossBreedReview("abc123", "bianmu") {
		t.Error("expected cross-breed review")
	}
	if rc.IsCrossBreedReview("abc123", "xigou") {
		t.Error("same breed should not be cross-breed")
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
