package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPipeline_ExecuteStage(t *testing.T) {
	dir := t.TempDir()
	createTestHook(t, dir, "s1", "S1", "session-init", 100)
	createTestHook(t, dir, "s2", "S2", "session-init", 200)

	r := NewRegistry(dir)
	r.Scan()

	p := NewPipeline(r, DefaultResolvers())
	input := &AssemblerInput{
		BreedID:   "bianmu",
		BreedName: "Border Collie",
	}

	result := p.ExecuteStage("session-init", input)
	if len(result.Patches) != 2 {
		t.Errorf("patches = %d, want 2", len(result.Patches))
	}
	if result.Patches[0].HookID != "S1" {
		t.Errorf("first patch = %q, want S1", result.Patches[0].HookID)
	}
}

func TestPipeline_ResolverSkip(t *testing.T) {
	dir := t.TempDir()
	hookDir := filepath.Join(dir, "d2-reanchor")
	os.MkdirAll(hookDir, 0755)
	yaml := `id: D2
name: ReAnchor
stage: per-turn
order: 200
version: 1
enabled: true
disableable: false
template: template.md
resolver: ReAnchorResolver
safetyTier: readonly
governanceTier: immutable
`
	os.WriteFile(filepath.Join(hookDir, "hook.yaml"), []byte(yaml), 0644)
	os.WriteFile(filepath.Join(hookDir, "template.md"), []byte("re-anchor"), 0644)

	r := NewRegistry(dir)
	r.Scan()

	p := NewPipeline(r, DefaultResolvers())
	input := &AssemblerInput{ToolCallCount: 3}

	result := p.ExecuteStage("per-turn", input)
	if len(result.Patches) != 0 {
		t.Errorf("patches = %d, want 0 (skipped)", len(result.Patches))
	}
	if len(result.Events) != 1 {
		t.Errorf("events = %d, want 1", len(result.Events))
	}
	if result.Events[0].Status != "skipped" {
		t.Errorf("event status = %q, want skipped", result.Events[0].Status)
	}
}

func TestPipeline_AssemblePatches(t *testing.T) {
	patches := []PromptPatch{
		{HookID: "S1", Content: "identity", Order: 100},
		{HookID: "S2", Content: "restrictions", Order: 200},
	}
	result := AssemblePatches(patches)
	if result != "identity\n\nrestrictions" {
		t.Errorf("assembled = %q", result)
	}
}
