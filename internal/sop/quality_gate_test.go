package sop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQualityGateSpecAlignment(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	os.WriteFile(specPath, []byte("# Spec\n## VISION Compatibility\n- [x] ok"), 0644)

	q := NewQualityGate(dir)
	result := q.Run(QualityGateInput{
		SpecFile:   "spec.md",
		HasCommits: true,
		HasTests:   true,
	})
	if !result.Passed {
		for _, c := range result.Checks {
			if !c.Passed {
				t.Errorf("check %s failed: %s", c.Name, c.Message)
			}
		}
	}
}

func TestQualityGateSpecNotFound(t *testing.T) {
	q := NewQualityGate(t.TempDir())
	result := q.Run(QualityGateInput{
		SpecFile:   "nonexistent.md",
		HasCommits: true,
		HasTests:   true,
	})
	if result.Passed {
		t.Error("expected fail for missing spec")
	}
	if result.Checks[0].Passed {
		t.Error("spec_alignment should fail")
	}
}

func TestQualityGateNoEvidence(t *testing.T) {
	q := NewQualityGate(t.TempDir())
	result := q.Run(QualityGateInput{
		SpecFile:   "",
		HasCommits: false,
		HasTests:   false,
	})
	if result.Passed {
		t.Error("expected fail for no evidence")
	}
}

func TestQualityGateVisionCoverage(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	os.WriteFile(specPath, []byte("# Spec\nNo vision section here"), 0644)

	q := NewQualityGate(dir)
	result := q.Run(QualityGateInput{
		SpecFile:   "spec.md",
		HasCommits: true,
		HasTests:   true,
	})
	// spec_alignment passes, evidence passes, but vision_coverage fails
	if result.Passed {
		t.Error("expected fail for missing VISION section")
	}
}

func TestQualityGateNoSpecAdvisory(t *testing.T) {
	q := NewQualityGate(t.TempDir())
	result := q.Run(QualityGateInput{
		SpecFile:   "",
		HasCommits: true,
		HasTests:   true,
	})
	if !result.Passed {
		t.Error("no spec should be advisory (pass)")
	}
}

func TestQualityGateAllChecks(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "feature-spec.md")
	os.WriteFile(specPath, []byte("# Feature\n## VISION Compatibility\n- [x] §3 compliant"), 0644)

	q := NewQualityGate(dir)
	result := q.Run(QualityGateInput{
		SpecFile:   "feature-spec.md",
		HasCommits: true,
		HasTests:   true,
	})
	if !result.Passed {
		t.Error("expected all checks to pass")
	}
	if len(result.Checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(result.Checks))
	}
}
