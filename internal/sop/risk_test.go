package sop

import "testing"

func TestRiskAssessmentLowRisk(t *testing.T) {
	a := RiskAssessment{Behavior: true}
	if a.IsHighRisk() {
		t.Error("single behavior axis should be low risk")
	}
	if a.Track() != TrackTargeted {
		t.Error("expected targeted track")
	}
}

func TestRiskAssessmentSecurityHighRisk(t *testing.T) {
	a := RiskAssessment{Security: true}
	if !a.IsHighRisk() {
		t.Error("security axis should be high risk")
	}
	if a.Track() != TrackFullGate {
		t.Error("expected full_gate track")
	}
}

func TestRiskAssessmentIrreversibleHighRisk(t *testing.T) {
	a := RiskAssessment{Irreversible: true}
	if !a.IsHighRisk() {
		t.Error("irreversible axis should be high risk")
	}
	if a.Track() != TrackFullGate {
		t.Error("expected full_gate track")
	}
}

func TestRiskAssessmentTwoAxesHighRisk(t *testing.T) {
	a := RiskAssessment{Behavior: true, Data: true}
	if !a.IsHighRisk() {
		t.Error("2+ axes should be high risk")
	}
	if a.Track() != TrackFullGate {
		t.Error("expected full_gate track")
	}
}

func TestRiskAssessmentContractAloneLowRisk(t *testing.T) {
	a := RiskAssessment{Contract: true}
	if a.IsHighRisk() {
		t.Error("single contract axis should be low risk")
	}
}

func TestRiskAssessmentFlaggedAxes(t *testing.T) {
	a := RiskAssessment{Behavior: true, Security: true, Data: true}
	axes := a.FlaggedAxes()
	if len(axes) != 3 {
		t.Fatalf("expected 3 axes, got %d", len(axes))
	}
}

func TestRiskRouterRoute(t *testing.T) {
	r := NewRiskRouter()
	if r.Route(RiskAssessment{}) != TrackTargeted {
		t.Error("empty assessment should route to targeted")
	}
	if r.Route(RiskAssessment{Security: true}) != TrackFullGate {
		t.Error("security should route to full_gate")
	}
}

func TestAssessRiskFromFiles(t *testing.T) {
	files := []string{
		"internal/sop/guardian.go",
		"internal/memory/store.go",
	}
	a := AssessRiskFromFiles(files)
	if !a.Behavior {
		t.Error("expected behavior flag from sop change")
	}
	if !a.Data {
		t.Error("expected data flag from memory change")
	}
	if !a.Contract {
		t.Error("expected contract flag from sop change")
	}
	if a.Track() != TrackFullGate {
		t.Error("expected full_gate for behavior+data+contract")
	}
}

func TestAssessRiskFromFilesSecurity(t *testing.T) {
	files := []string{"internal/auth/token.go"}
	a := AssessRiskFromFiles(files)
	if !a.Security {
		t.Error("expected security flag from auth file")
	}
}

func TestAssessRiskFromFilesIrreversible(t *testing.T) {
	files := []string{"docs/VISION.md"}
	a := AssessRiskFromFiles(files)
	if !a.Irreversible {
		t.Error("expected irreversible flag from VISION.md")
	}
}

func TestAssessRiskFromFilesEmpty(t *testing.T) {
	a := AssessRiskFromFiles(nil)
	if a.IsHighRisk() {
		t.Error("empty files should be low risk")
	}
}

func TestRouteFromChangedFiles(t *testing.T) {
	r := NewRiskRouter()
	track := r.RouteFromChangedFiles([]string{"packs/default/breeds/dog-template.json"})
	if track != TrackFullGate {
		t.Error("breed config change should be full_gate")
	}
}
