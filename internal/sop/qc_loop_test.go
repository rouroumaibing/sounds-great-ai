package sop

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestQCLoopStepStructure(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		AuthorBreed:   "bianmu",
		ReviewerBreed: "xigou",
		ChangedFiles:  []string{"somefile.go"},
	})
	if len(result.Steps) != 7 {
		t.Errorf("expected 7 steps, got %d", len(result.Steps))
	}
	expectedNames := []string{"hygiene", "fresh_context", "cross_breed_review", "evidence_manifest", "ci_repair", "verdict", "sign_off"}
	for i, name := range expectedNames {
		if result.Steps[i].Name != name {
			t.Errorf("step %d: expected %s, got %s", i+1, name, result.Steps[i].Name)
		}
		if result.Steps[i].Step != i+1 {
			t.Errorf("step %d: expected step number %d, got %d", i+1, i+1, result.Steps[i].Step)
		}
	}
}

func TestQCLoopCrossBreedReview(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		AuthorBreed:   "bianmu",
		ReviewerBreed: "xigou",
	})
	if !result.Steps[2].Passed {
		t.Error("cross-breed review should pass for different breeds")
	}
}

func TestQCLoopSameBreedReview(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		AuthorBreed:   "bianmu",
		ReviewerBreed: "bianmu",
	})
	if result.Steps[2].Passed {
		t.Error("cross-breed review should fail for same breed")
	}
}

func TestQCLoopFreshContextSameBreed(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		AuthorBreed:   "bianmu",
		ReviewerBreed: "bianmu",
	})
	if result.Steps[1].Passed {
		t.Error("fresh-context should fail for same breed")
	}
}

func TestQCLoopEvidenceManifest(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		ChangedFiles: []string{"a.go", "b.go", "c.go"},
	})
	if !result.Steps[3].Passed {
		t.Error("evidence manifest should pass with changed files")
	}
}

func TestQCLoopNoChangedFiles(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{})
	if !result.Steps[3].Passed {
		t.Error("evidence manifest should be advisory with no files")
	}
}

func TestQCLoopAdvisoryNoBreeds(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{})
	if !result.Steps[1].Passed {
		t.Error("fresh_context should be advisory with no breeds")
	}
	if !result.Steps[2].Passed {
		t.Error("cross_breed_review should be advisory with no breeds")
	}
}

// Three-layer panel (clowder F253): reviewer must differ from author, and the
// Layer-3 final approver must be independent of both.
func TestQCLoopFinalApproverSameAsAuthor(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		AuthorBreed:        "bianmu",
		ReviewerBreed:      "xigou",
		FinalApproverBreed: "bianmu", // same as author
	})
	if result.Steps[2].Passed {
		t.Error("cross_breed_review should fail when final approver == author")
	}
}

func TestQCLoopFinalApproverSameAsReviewer(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		AuthorBreed:        "bianmu",
		ReviewerBreed:      "xigou",
		FinalApproverBreed: "xigou", // same as reviewer
	})
	if result.Steps[2].Passed {
		t.Error("cross_breed_review should fail when final approver == reviewer")
	}
}

func TestQCLoopThreeLayerPanelOK(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		AuthorBreed:        "bianmu",
		ReviewerBreed:      "xigou",
		FinalApproverBreed: "demu", // distinct from both
	})
	if !result.Steps[2].Passed {
		t.Error("cross_breed_review should pass for a valid 3-layer panel")
	}
}

// Risk-tiering (clowder F253 trigger strategy): doc-only changes skip the
// shared-capability steps (cross_breed_review / evidence_manifest / ci_repair)
// to avoid alarm fatigue, while still running hygiene + fresh_context. The
// overall verdict still depends on the guardian sign-off (which needs a real
// spec/feature context), so this test asserts the tiering + skip behaviour
// rather than the aggregate Passed flag.
func TestQCLoopLightRiskTier(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		AuthorBreed:   "bianmu",
		ReviewerBreed: "xigou",
		ChangedFiles:  []string{"docs/guide.md", "README.md"},
	})
	if result.RiskTier != "light" {
		t.Errorf("expected light risk tier, got %q", result.RiskTier)
	}
	if len(result.Steps) != 7 {
		t.Errorf("light tier keeps the 7-step slots, got %d", len(result.Steps))
	}
	for _, s := range result.Steps {
		if s.Name == "ci_repair" && !strings.Contains(s.Message, "low-risk") {
			t.Errorf("ci_repair should be skipped in light tier, got %q", s.Message)
		}
		if s.Name == "cross_breed_review" && !strings.Contains(s.Message, "low-risk") {
			t.Errorf("cross_breed_review should be skipped in light tier, got %q", s.Message)
		}
	}
}

// Full risk tier is the default for unknown (empty) or Go-touching changes.
func TestQCLoopFullRiskTier(t *testing.T) {
	q := NewQCLoop(t.TempDir())
	result := q.Run(QCLoopInput{
		AuthorBreed:   "bianmu",
		ReviewerBreed: "xigou",
		ChangedFiles:  []string{"internal/foo.go"},
	})
	if result.RiskTier != "full" {
		t.Errorf("expected full risk tier for Go change, got %q", result.RiskTier)
	}
	if len(result.Steps) != 7 {
		t.Errorf("full tier should run 7 steps, got %d", len(result.Steps))
	}
}

// ComputeStale: a moved HEAD invalidates the prior verdict; a matching HEAD or
// an unknown prior state does not.
func TestComputeStale(t *testing.T) {
	base := QCState{ReviewedSha: "abc123"}
	if !ComputeStale(base, "def456") {
		t.Error("moved HEAD should be stale")
	}
	if ComputeStale(base, "abc123") {
		t.Error("matching HEAD should not be stale")
	}
	if ComputeStale(base, "") {
		t.Error("unknown sha (non-git) should not be stale")
	}
	if ComputeStale(QCState{}, "abc123") {
		t.Error("no prior reviewed sha should not be stale")
	}
}

// QCState load/save round-trips through a temp file.
func TestQCStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qc-state.json")
	in := QCState{Phase: "qc.archived", ReviewedSha: "deadbeef", IdempotencyKey: "deadbeef-1", StaleFlag: true}
	if err := SaveQCState(path, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out := LoadQCState(path)
	if out.Phase != in.Phase || out.ReviewedSha != in.ReviewedSha || out.IdempotencyKey != in.IdempotencyKey || out.StaleFlag != in.StaleFlag {
		t.Errorf("round-trip mismatch: %+v != %+v", out, in)
	}
	// Missing file yields a fresh idle state (non-fatal).
	idle := LoadQCState(filepath.Join(t.TempDir(), "missing.json"))
	if idle.Phase != "qc.idle" {
		t.Errorf("expected idle phase for missing file, got %q", idle.Phase)
	}
}
