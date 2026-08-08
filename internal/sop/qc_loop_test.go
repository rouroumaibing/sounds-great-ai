package sop

import "testing"

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
