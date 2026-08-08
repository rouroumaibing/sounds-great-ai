package hooks

import "testing"

func TestIdentityResolver(t *testing.T) {
	r := &IdentityResolver{}
	result := r.Resolve(&AssemblerInput{BreedID: "bianmu", BreedName: "Border Collie", RoleDescription: "router"})
	if result.Status != "fired" {
		t.Errorf("Status = %q, want %q", result.Status, "fired")
	}
	if result.Vars["BreedID"] != "bianmu" {
		t.Errorf("Vars[BreedID] = %q, want %q", result.Vars["BreedID"], "bianmu")
	}
}

func TestAlwaysFireResolver(t *testing.T) {
	r := &AlwaysFireResolver{}
	result := r.Resolve(&AssemblerInput{})
	if result.Status != "fired" {
		t.Errorf("Status = %q, want %q", result.Status, "fired")
	}
}

func TestPhaseAnchorResolver(t *testing.T) {
	r := &PhaseAnchorResolver{}
	result := r.Resolve(&AssemblerInput{CurrentPhase: "Phase 2"})
	if result.Status != "fired" {
		t.Errorf("Status = %q, want %q", result.Status, "fired")
	}
	if result.Vars["CurrentPhase"] != "Phase 2" {
		t.Errorf("Vars[CurrentPhase] = %q, want %q", result.Vars["CurrentPhase"], "Phase 2")
	}
}

func TestReAnchorResolver_BelowThreshold(t *testing.T) {
	r := &ReAnchorResolver{}
	result := r.Resolve(&AssemblerInput{ToolCallCount: 3})
	if result.Status != "skipped" {
		t.Errorf("Status = %q, want %q", result.Status, "skipped")
	}
}

func TestReAnchorResolver_AboveThreshold(t *testing.T) {
	r := &ReAnchorResolver{}
	result := r.Resolve(&AssemblerInput{ToolCallCount: 10})
	if result.Status != "fired" {
		t.Errorf("Status = %q, want %q", result.Status, "fired")
	}
}

func TestDefaultResolvers(t *testing.T) {
	resolvers := DefaultResolvers()
	expected := []string{"IdentityResolver", "AlwaysFireResolver", "PhaseAnchorResolver", "ReAnchorResolver", "LeaderRefResolver"}
	for _, name := range expected {
		if _, ok := resolvers[name]; !ok {
			t.Errorf("resolver %q not found in DefaultResolvers", name)
		}
	}
}
