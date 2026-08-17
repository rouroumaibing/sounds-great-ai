package platform

import (
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/settings"
)

func TestPlatformNew(t *testing.T) {
	// Create temp dirs for breeds and skills
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()
	workspaceDir := t.TempDir()

	// Write a minimal breed JSON
	breedJSON := `{
		"id": "bianmu",
		"name": "边牧",
		"display_name": "边牧",
		"default_variant_id": "bianmu-claude",
		"variants": [{"id": "bianmu-claude", "client_id": "anthropic", "default_model": "claude-opus-4-6", "cli": {"command": "claude", "output_format": "stream-json"}}]
	}`
	os.WriteFile(filepath.Join(breedsDir, "dog-template.json"), []byte(`{"version":2,"breeds":[`+breedJSON+`]}`), 0644)

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: workspaceDir, MaxA2ADepth: 3})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Verify adapters
	if p.AgentExecutor.Count() != 6 {
		t.Fatalf("expected 6 adapters (claude/codex/gemini/kimi/opencode + a2a protocol client), got %d", p.AgentExecutor.Count())
	}
	for _, name := range []string{"claude", "codex", "gemini", "kimi", "opencode", "a2a"} {
		if _, err := p.GetAdapter(name); err != nil {
			t.Errorf("GetAdapter(%s): %v", name, err)
		}
	}

	// Decision D1: on a fresh first run (no catalog file) the runtime registry
	// is empty — no dogs are auto-injected; the template is only a menu.
	if len(p.Breeds) != 0 {
		t.Errorf("expected empty breed registry on first run (D1), got %d", len(p.Breeds))
	}
	if p.GetBreed("bianmu") != nil {
		t.Error("did not expect bianmu on first run (D1)")
	}

	// Verify SOP
	if p.SOP.MaxA2ADepth() != 3 {
		t.Errorf("MaxA2ADepth = %d, want 3", p.SOP.MaxA2ADepth())
	}

	// Verify memory is initialized
	if p.Memory == nil {
		t.Error("expected non-nil memory store")
	}
}

func TestPlatformGetAdapterUnknown(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := p.GetAdapter("unknown"); err == nil {
		t.Error("expected error for unknown CLI")
	}
}

// ---------------------------------------------------------------------------
// Edge case tests
// ---------------------------------------------------------------------------

func TestPlatformReadyWithBreeds(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()
	workspaceDir := t.TempDir()

	// Pre-seed an existing catalog (so this exercises the existing-catalog
	// path, not first-run) with a ready breed.
	catalogDir := settings.ConfigRoot(workspaceDir)
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatalf("mkdir catalog dir: %v", err)
	}
	catalogJSON := `{"version":2,"breeds":[{"id":"bianmu","name":"边牧","display_name":"边牧","default_variant_id":"v1","enabled":true,"variants":[{"id":"v1","client_id":"anthropic","default_model":"claude-opus-4-6","cli":{"command":"claude","output_format":"stream-json"}}]}],"seen_template_breeds":["bianmu"]}`
	if err := os.WriteFile(filepath.Join(catalogDir, settings.CatalogFileName), []byte(catalogJSON), 0644); err != nil {
		t.Fatalf("write catalog: %v", err)
	}

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: workspaceDir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if p.GetBreed("bianmu") == nil {
		t.Error("expected bianmu from existing catalog")
	}
	if !p.Ready() {
		t.Error("expected Ready()=true with breeds loaded")
	}
}

func TestPlatformReadyNoBreeds(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if p.Ready() {
		t.Error("expected Ready()=false with no breeds")
	}
}

func TestPlatformGetBreedNotFound(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if breed := p.GetBreed("nonexistent"); breed != nil {
		t.Errorf("expected nil for nonexistent breed, got %v", breed)
	}
}

func TestPlatformGetAdapterTableDriven(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		cliName string
		wantErr bool
	}{
		{"claude", false},
		{"codex", false},
		{"gemini", false},
		{"opencode", false},
		{"unknown", true},
		{"", true},
		{"CLAUDE", true}, // case-sensitive
	}

	for _, tc := range tests {
		t.Run(tc.cliName, func(t *testing.T) {
			_, err := p.GetAdapter(tc.cliName)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tc.cliName)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.cliName, err)
			}
		})
	}
}

func TestPlatformClose(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestPlatformBuildMCPConfigEmpty(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cfg := p.BuildMCPConfig()
	if cfg != nil {
		t.Errorf("expected nil MCP config with no servers, got %v", cfg)
	}
}

func TestPlatformNewDefaultMaxA2ADepth(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	breedJSON := `{"id":"bianmu","name":"边牧","display_name":"边牧","default_variant_id":"v1","variants":[{"id":"v1","client_id":"anthropic","default_model":"claude-opus-4-6","cli":{"command":"claude","output_format":"stream-json"}}]}`
	os.WriteFile(filepath.Join(breedsDir, "dog-template.json"), []byte(`{"version":2,"breeds":[`+breedJSON+`]}`), 0644)

	tests := []struct {
		name    string
		depth   int
		wantDepth int
	}{
		{"zero defaults to 3", 0, 3},
		{"negative defaults to 3", -1, 3},
		{"explicit 5", 5, 5},
		{"explicit 1", 1, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: t.TempDir(), MaxA2ADepth: tc.depth})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if got := p.SOP.MaxA2ADepth(); got != tc.wantDepth {
				t.Errorf("MaxA2ADepth = %d, want %d", got, tc.wantDepth)
			}
		})
	}
}

func TestPlatformNewWithWorkspaceDir(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()
	workspaceDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: workspaceDir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if p.WorkspaceDir != workspaceDir {
		t.Errorf("WorkspaceDir = %q, want %q", p.WorkspaceDir, workspaceDir)
	}
}

func TestPlatformAdaptersCount(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if p.AgentExecutor.Count() != 6 {
		t.Errorf("expected 6 adapters (claude/codex/gemini/kimi/opencode + a2a protocol client), got %d", p.AgentExecutor.Count())
	}
}

// TestPlatformCarrierChainClaudeFirst locks in the per-provider default chain
// (2026-08-15): the three long-session CLIs — claude/codex/gemini — each lead
// with bg_daemon (per-provider warm-pool tier, Point 4); opencode/kimi stay
// one-shot print_sdk. The bg_daemon transport is only wired under
// -tags pty (WireWarmPools), so in the default build those three transparently
// falls back to print_sdk — but the chain order itself must still reflect the
// claude-first intent so the warm pool activates without a re-register.
func TestPlatformCarrierChainClaudeFirst(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.CarrierRegistry == nil {
		t.Fatal("expected non-nil CarrierRegistry")
	}

	for _, name := range []string{"claude", "codex", "gemini"} {
		want := []string{"bg_daemon", "print_sdk"}
		got := kinds(p.CarrierRegistry.ResolveChain(name))
		if !equalKinds(got, want) {
			t.Errorf("%s chain = %v, want %v", name, got, want)
		}
	}

	for _, name := range []string{"opencode", "kimi"} {
		want := []string{"print_sdk"}
		got := kinds(p.CarrierRegistry.ResolveChain(name))
		if !equalKinds(got, want) {
			t.Errorf("%s chain = %v, want %v", name, got, want)
		}
	}
}

func kinds(in []unified.TransportKind) []string {
	out := make([]string, len(in))
	for i, k := range in {
		out[i] = string(k)
	}
	return out
}

func equalKinds(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
