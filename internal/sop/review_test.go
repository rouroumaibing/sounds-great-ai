package sop

import "testing"

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
