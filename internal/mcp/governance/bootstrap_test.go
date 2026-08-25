package governance

import "testing"

func TestBootstrapReport_ZeroWritePreview(t *testing.T) {
	reg := NewCapabilityRegistry()
	rep := NewBootstrapReport()

	acts := rep.Preview([]ToolOverride{{Name: "sg_create_thread", Disabled: true}})
	if len(acts) != 1 || acts[0].Kind != "disable" {
		t.Fatalf("preview wrong: %+v", acts)
	}
	// Preview must not have mutated the registry (zero-write).
	if len(reg.ForProject("proj-b")) != len(reg.Global()) {
		t.Fatal("Preview mutated the registry (not zero-write)")
	}
}

func TestBootstrapReport_ExecuteAndUndo(t *testing.T) {
	reg := NewCapabilityRegistry()
	rep := NewBootstrapReport()
	overrides := []ToolOverride{{Name: "sg_create_thread", Disabled: true}}

	applied := rep.Execute(reg, "proj-b", overrides)
	if applied != 1 {
		t.Fatalf("expected 1 applied, got %d", applied)
	}
	// The tool is gone from the project surface.
	for _, td := range reg.ForProject("proj-b") {
		if td.Name == "sg_create_thread" {
			t.Fatal("execute did not disable the tool for the project")
		}
	}
	// Reversible: undo restores the global surface.
	rep.Undo(reg, "proj-b")
	if len(reg.ForProject("proj-b")) != len(reg.Global()) {
		t.Fatal("Undo did not restore the global surface")
	}
}
