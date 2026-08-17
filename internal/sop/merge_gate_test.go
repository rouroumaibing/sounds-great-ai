package sop

import "testing"

func TestMergeGateAllPass(t *testing.T) {
	rc := NewReviewCycle()
	rc.ReceiveReview(ReviewProvenance{
		ReviewerDogID: "xigou-dog",
		ReviewSHA:     "abc123",
	})

	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"internal/sop/review.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{LocalPeerReviewSHA: "abc123", CurrentHead: "abc123"},
		AuthorDogID:         "bianmu",
		QualityGatePassed:   true,
		HasUncommittedChanges: false,
		TestsPass:           true,
		ReviewCycle:         rc,
	})
	if !result.Passed {
		for _, c := range result.Conditions {
			if !c.Passed {
				t.Errorf("condition %s failed: %s", c.ID, c.Message)
			}
		}
	}
}

func TestMergeGateNoReview(t *testing.T) {
	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"somefile.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{},
		QualityGatePassed:   true,
		HasUncommittedChanges: false,
		TestsPass:           true,
	})
	if result.Passed {
		t.Error("expected fail: no review")
	}
	if result.Conditions[0].Passed {
		t.Error("E1 should fail")
	}
}

func TestMergeGateQualityGateFailed(t *testing.T) {
	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"somefile.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{LocalPeerReviewSHA: "abc123"},
		QualityGatePassed:   false,
		HasUncommittedChanges: false,
		TestsPass:           true,
	})
	if result.Passed {
		t.Error("expected fail: quality gate not passed")
	}
}

func TestMergeGateUncommittedChanges(t *testing.T) {
	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"somefile.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{LocalPeerReviewSHA: "abc123"},
		QualityGatePassed:   true,
		HasUncommittedChanges: true,
		TestsPass:           true,
	})
	if result.Passed {
		t.Error("expected fail: uncommitted changes")
	}
}

func TestMergeGateTestsFail(t *testing.T) {
	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"somefile.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{LocalPeerReviewSHA: "abc123"},
		QualityGatePassed:   true,
		HasUncommittedChanges: false,
		TestsPass:           false,
	})
	if result.Passed {
		t.Error("expected fail: tests not passing")
	}
}

func TestMergeGateRiskTrack(t *testing.T) {
	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"docs/VISION.md"},
		ReviewMatrix:        ReviewProvenanceMatrix{LocalPeerReviewSHA: "abc123"},
		QualityGatePassed:   true,
		HasUncommittedChanges: false,
		TestsPass:           true,
	})
	if result.Track != TrackFullGate {
		t.Error("VISION.md change should be full_gate")
	}
}

func TestMergeGateCrossBreedReview(t *testing.T) {
	rc := NewReviewCycle()
	rc.ReceiveReview(ReviewProvenance{
		ReviewerDogID: "xigou-dog",
		ReviewSHA:     "abc123",
	})

	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"somefile.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{LocalPeerReviewSHA: "abc123"},
		AuthorDogID:         "bianmu",
		QualityGatePassed:   true,
		HasUncommittedChanges: false,
		TestsPass:           true,
		ReviewCycle:         rc,
	})
	if !result.Conditions[1].Passed {
		t.Error("E2 cross-breed should pass")
	}
}

func TestMergeGateSelfReviewFails(t *testing.T) {
	rc := NewReviewCycle()
	rc.ReceiveReview(ReviewProvenance{
		ReviewerDogID: "bianmu",
		ReviewSHA:     "abc123",
	})

	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"somefile.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{LocalPeerReviewSHA: "abc123"},
		AuthorDogID:         "bianmu",
		QualityGatePassed:   true,
		HasUncommittedChanges: false,
		TestsPass:           true,
		ReviewCycle:         rc,
	})
	if result.Passed {
		t.Error("expected fail: self-review must not pass the merge gate")
	}
	if result.Conditions[1].Passed {
		t.Error("E2 should fail on self-review")
	}
}

func TestMergeGateExternalReviewBindsTarget(t *testing.T) {
	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"somefile.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{CloudReviewSHA: "ext-1", CurrentHead: "ext-1"},
		QualityGatePassed:   true,
		HasUncommittedChanges: false,
		TestsPass:           true,
	})
	if !result.Passed {
		t.Fatalf("external same-target review should pass: %+v", result.Conditions[1])
	}
}

func TestMergeGateExternalReviewWrongTarget(t *testing.T) {
	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"somefile.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{CloudReviewSHA: "ext-1", CurrentHead: "head-other"},
		QualityGatePassed:   true,
		HasUncommittedChanges: false,
		TestsPass:           true,
	})
	if result.Passed {
		t.Fatal("external review not binding the exact target revision must fail")
	}
}

func TestMergeGateAmbiguousIntentFailsClosed(t *testing.T) {
	g := NewMergeGate("")
	result := g.Run(MergeGateInput{
		ChangedFiles:        []string{"somefile.go"},
		ReviewMatrix:        ReviewProvenanceMatrix{LocalPeerReviewSHA: "", CloudReviewSHA: ""},
		QualityGatePassed:   true,
		HasUncommittedChanges: false,
		TestsPass:           true,
	})
	if result.Passed {
		t.Fatal("ambiguous review intent must fail closed")
	}
}
