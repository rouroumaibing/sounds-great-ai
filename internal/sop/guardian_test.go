package sop

import (
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/internal/a2a"
)

func TestCheckA2ADepthContinue(t *testing.T) {
	g := NewGuardian(nil, 3)
	thread := &a2a.Thread{ID: "test", ReviewRoundCount: 1}
	if action := g.CheckA2ADepth(thread); action != Continue {
		t.Errorf("expected Continue, got %v", action)
	}
}

func TestCheckA2ADepthEscalate(t *testing.T) {
	g := NewGuardian(nil, 3)
	thread := &a2a.Thread{ID: "test", ReviewRoundCount: 3}
	if action := g.CheckA2ADepth(thread); action != EscalateToCVO {
		t.Errorf("expected EscalateToCVO, got %v", action)
	}
}

func TestCheckA2ADepthBoundary(t *testing.T) {
	g := NewGuardian(nil, 3)
	thread := &a2a.Thread{ID: "test", ReviewRoundCount: 2}
	if action := g.CheckA2ADepth(thread); action != Continue {
		t.Errorf("expected Continue at round 2 (limit 3), got %v", action)
	}
}

func TestGuardianSignOffAllPass(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "feature-x.md"), []byte("# Feature X\n## VISION Compatibility\n- [x] ok"), 0644)

	g := NewGuardian(nil, 3)
	result := g.SignOff(GuardianInput{
		FeatureName: "feature-x",
		SpecDir:     specDir,
		WorkDir:     dir,
	})
	// checkHasTests will fail since we're not in a real project, but spec + vision should pass
	if !result.Questions[0].Passed {
		t.Errorf("spec check should pass: %s", result.Questions[0].Message)
	}
	if !result.Questions[2].Passed {
		t.Errorf("vision check should pass: %s", result.Questions[2].Message)
	}
}

func TestGuardianSignOffNoSpec(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs")
	os.MkdirAll(specDir, 0755)

	g := NewGuardian(nil, 3)
	result := g.SignOff(GuardianInput{
		FeatureName: "nonexistent",
		SpecDir:     specDir,
		WorkDir:     dir,
	})
	if result.SignedOff {
		t.Error("expected sign-off to fail with no spec")
	}
	if result.Questions[0].Passed {
		t.Error("spec check should fail")
	}
}

func TestGuardianSignOffNoVision(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "feature-y.md"), []byte("# Feature Y\nNo vision section"), 0644)

	g := NewGuardian(nil, 3)
	result := g.SignOff(GuardianInput{
		FeatureName: "feature-y",
		SpecDir:     specDir,
		WorkDir:     dir,
	})
	if result.Questions[2].Passed {
		t.Error("vision check should fail")
	}
}

func TestRequiresGuardianSignOff(t *testing.T) {
	if !RequiresGuardianSignOff("completion") {
		t.Error("completion should require guardian sign-off")
	}
	if RequiresGuardianSignOff("impl") {
		t.Error("impl should not require guardian sign-off")
	}
	if RequiresGuardianSignOff("review") {
		t.Error("review should not require guardian sign-off")
	}
}

func TestGuardianSignOffQuestions(t *testing.T) {
	dir := t.TempDir()
	g := NewGuardian(nil, 3)
	result := g.SignOff(GuardianInput{
		SpecDir: "/nonexistent",
		WorkDir: dir,
	})
	if len(result.Questions) != 3 {
		t.Errorf("expected 3 questions, got %d", len(result.Questions))
	}
	expectedQuestions := []string{
		"Does feature have spec?",
		"Does feature have tests?",
		"Does feature have VISION compatibility?",
	}
	for i, eq := range expectedQuestions {
		if result.Questions[i].Question != eq {
			t.Errorf("question %d: expected %q, got %q", i, eq, result.Questions[i].Question)
		}
	}
}
