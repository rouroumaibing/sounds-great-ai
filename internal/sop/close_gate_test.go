package sop

import (
	"testing"
)

func TestCloseGateGenerateShip(t *testing.T) {
	cg := NewCloseGate(t.TempDir())
	report, err := cg.Generate(CloseGateInput{
		FeatureName: "feature-ship",
		ACMatrix: []AcceptanceCriterion{
			{ID: "ac1", Text: "AC 1", Passed: true},
			{ID: "ac2", Text: "AC 2", Passed: true},
		},
		QualityGate: QualityGateResult{Passed: true},
		MergeGate:   MergeGateResult{Passed: true},
		Guardian:    GuardianResult{SignedOff: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Resolution != ResolutionShip {
		t.Errorf("expected ship, got %s", report.Resolution)
	}
}

func TestCloseGateGenerateIterate(t *testing.T) {
	cg := NewCloseGate(t.TempDir())
	report, _ := cg.Generate(CloseGateInput{
		FeatureName: "feature-iterate",
		ACMatrix: []AcceptanceCriterion{
			{ID: "ac1", Text: "AC 1", Passed: true},
			{ID: "ac2", Text: "AC 2", Passed: false},
		},
		QualityGate: QualityGateResult{Passed: true},
		MergeGate:   MergeGateResult{Passed: true},
		Guardian:    GuardianResult{SignedOff: true},
	})
	if report.Resolution != ResolutionIterate {
		t.Errorf("expected iterate, got %s", report.Resolution)
	}
}

func TestCloseGateGenerateSunset(t *testing.T) {
	cg := NewCloseGate(t.TempDir())
	report, _ := cg.Generate(CloseGateInput{
		FeatureName: "feature-sunset",
		ACMatrix: []AcceptanceCriterion{
			{ID: "ac1", Text: "AC 1", Passed: false},
		},
		QualityGate: QualityGateResult{Passed: false},
		MergeGate:   MergeGateResult{Passed: false},
		Guardian:    GuardianResult{SignedOff: false},
	})
	if report.Resolution != ResolutionSunset {
		t.Errorf("expected sunset, got %s", report.Resolution)
	}
}

func TestCloseGatePersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	cg := NewCloseGate(dir)

	report, err := cg.GenerateAndPersist(CloseGateInput{
		FeatureName: "feature-test",
		ACMatrix: []AcceptanceCriterion{
			{ID: "ac1", Text: "AC 1", Passed: true},
		},
		Evidence: CloseEvidence{
			Commits: []string{"abc123", "def456"},
		},
		QualityGate: QualityGateResult{Passed: true},
		MergeGate:   MergeGateResult{Passed: true},
		Guardian:    GuardianResult{SignedOff: true},
	})
	if err != nil {
		t.Fatalf("GenerateAndPersist failed: %v", err)
	}
	if report.Resolution != ResolutionShip {
		t.Errorf("expected ship, got %s", report.Resolution)
	}

	// Load it back
	loaded, err := cg.Load("feature-test")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.FeatureName != "feature-test" {
		t.Errorf("expected feature-test, got %s", loaded.FeatureName)
	}
	if len(loaded.ACMatrix) != 1 {
		t.Errorf("expected 1 AC, got %d", len(loaded.ACMatrix))
	}
	if len(loaded.Evidence.Commits) != 2 {
		t.Errorf("expected 2 commits, got %d", len(loaded.Evidence.Commits))
	}
}

func TestCloseGateLoadNotFound(t *testing.T) {
	cg := NewCloseGate(t.TempDir())
	_, err := cg.Load("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestCloseGateReportStructure(t *testing.T) {
	cg := NewCloseGate(t.TempDir())
	report, _ := cg.Generate(CloseGateInput{
		FeatureName: "feature-struct",
		ACMatrix:    []AcceptanceCriterion{{ID: "ac1", Text: "test", Passed: true}},
		Evidence: CloseEvidence{
			Commits:          []string{"sha1"},
			TestResults:      []CheckResult{{Name: "go_test", Passed: true}},
			ReviewProvenance: []ReviewProvenance{{ReviewerBreed: "xigou", ReviewSHA: "sha1"}},
		},
		QualityGate: QualityGateResult{Passed: true, Checks: []CheckResult{{Name: "spec", Passed: true}}},
		MergeGate:   MergeGateResult{Passed: true, Conditions: []ConditionResult{{ID: "E1", Passed: true}}},
		Guardian:    GuardianResult{SignedOff: true, Questions: []QuestionResult{{Question: "spec?", Passed: true}}},
	})
	if report.FeatureName != "feature-struct" {
		t.Error("wrong feature name")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("generated_at should not be zero")
	}
	if len(report.Evidence.Commits) != 1 {
		t.Error("wrong commits count")
	}
	if len(report.Evidence.TestResults) != 1 {
		t.Error("wrong test results count")
	}
	if len(report.Evidence.ReviewProvenance) != 1 {
		t.Error("wrong provenance count")
	}
}
