package governance

import "testing"

func TestCapabilityRegistry_GlobalSingleSource(t *testing.T) {
	r := NewCapabilityRegistry()
	g := r.Global()
	if len(g) == 0 {
		t.Fatal("global catalog empty")
	}
	// Mutating the returned slice must NOT affect the internal source.
	g[0].Name = "tampered"
	if r.Global()[0].Name == "tampered" {
		t.Fatal("Global() returned the internal slice (mutable) — single-source broken")
	}
}

func TestCapabilityRegistry_PerProjectOverride(t *testing.T) {
	r := NewCapabilityRegistry()
	// Override proj-x to force sg_get_thread writable (ReadOnly=false).
	ro := false
	r.ApplyOverride("proj-x", ToolOverride{Name: "sg_create_thread", Disabled: true})
	r.ApplyOverride("proj-x", ToolOverride{Name: "sg_get_thread", ReadOnly: &ro})

	global := r.Global()
	proj := r.ForProject("proj-x")

	// Global still contains the disabled tool.
	foundGlobal := false
	for _, td := range global {
		if td.Name == "sg_create_thread" {
			foundGlobal = true
		}
	}
	if !foundGlobal {
		t.Fatal("global surface lost sg_create_thread after per-project override")
	}

	// Per-project surface removes the disabled tool.
	for _, td := range proj {
		if td.Name == "sg_create_thread" {
			t.Fatal("per-project disable not applied")
		}
	}
	// Per-project ReadOnly override applied (forced writable).
	for _, td := range proj {
		if td.Name == "sg_get_thread" && td.ReadOnly {
			t.Fatal("per-project ReadOnly override not applied")
		}
	}

	// Another project is unaffected (single source of truth preserved): the
	// override must NOT leak to proj-y (sg_get_thread stays ReadOnly=true).
	other := r.ForProject("proj-y")
	for _, td := range other {
		if td.Name == "sg_get_thread" && !td.ReadOnly {
			t.Fatal("per-project override leaked to an unrelated project")
		}
	}
}

func TestCapabilityRegistry_ResetProject(t *testing.T) {
	r := NewCapabilityRegistry()
	r.ApplyOverride("p", ToolOverride{Name: "sg_create_thread", Disabled: true})
	if len(r.ForProject("p")) == len(r.Global()) {
		t.Fatal("override should have removed a tool")
	}
	r.ResetProject("p")
	if len(r.ForProject("p")) != len(r.Global()) {
		t.Fatal("ResetProject did not restore the global surface")
	}
}

func TestSurfaceLifecycle_LazyAndAtomic(t *testing.T) {
	r := NewCapabilityRegistry()
	lc := SurfaceLifecycle{ResourceFamily: "collab", Lazy: true, AtomicSwitch: true}
	r.SetLifecycle("sg_create_thread", lc)

	// Lazy tool is excluded from the baseline digest until first used (F286).
	if r.InBaseline("sg_create_thread") {
		t.Fatal("lazy tool must be excluded from baseline until used")
	}
	r.MarkUsed("sg_create_thread")
	if !r.InBaseline("sg_create_thread") {
		t.Fatal("after MarkUsed, tool must be in baseline")
	}

	// Atomic family switch toggles the whole resource family (F286).
	r.SetFamilyEnabled("collab", false)
	if r.FamilyEnabled("collab") {
		t.Fatal("resource family should be disabled after atomic switch")
	}
	r.SetFamilyEnabled("collab", true)
	if !r.FamilyEnabled("collab") {
		t.Fatal("resource family should be enabled after atomic switch")
	}
}
