package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformNew(t *testing.T) {
	// Create temp dirs for breeds and skills
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	// Write a minimal breed JSON
	breedJSON := `{
		"id": "bianmu",
		"name": "边牧",
		"display_name": "边牧",
		"default_variant_id": "bianmu-claude",
		"variants": [{"id": "bianmu-claude", "client_id": "anthropic", "default_model": "claude-opus-4-6", "cli": {"command": "claude", "output_format": "stream-json"}}]
	}`
	os.WriteFile(filepath.Join(breedsDir, "bianmu.json"), []byte(breedJSON), 0644)

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, MaxA2ADepth: 3})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Verify adapters
	if len(p.Adapters) != 4 {
		t.Fatalf("expected 4 adapters, got %d", len(p.Adapters))
	}
	for _, name := range []string{"claude", "codex", "gemini", "opencode"} {
		if _, err := p.GetAdapter(name); err != nil {
			t.Errorf("GetAdapter(%s): %v", name, err)
		}
	}

	// Verify breeds
	if p.GetBreed("bianmu") == nil {
		t.Error("expected bianmu breed")
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

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir})
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

	breedJSON := `{"id":"bianmu","name":"边牧","display_name":"边牧","default_variant_id":"v1","variants":[{"id":"v1","client_id":"anthropic","default_model":"claude-opus-4-6","cli":{"command":"claude","output_format":"stream-json"}}]}`
	os.WriteFile(filepath.Join(breedsDir, "bianmu.json"), []byte(breedJSON), 0644)

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if !p.Ready() {
		t.Error("expected Ready()=true with breeds loaded")
	}
}

func TestPlatformReadyNoBreeds(t *testing.T) {
	breedsDir := t.TempDir()
	skillsDir := t.TempDir()

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir})
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

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir})
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

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir})
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

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir})
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

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir})
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
	os.WriteFile(filepath.Join(breedsDir, "bianmu.json"), []byte(breedJSON), 0644)

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
			p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir, MaxA2ADepth: tc.depth})
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

	p, err := New(Config{BreedsDir: breedsDir, SkillsDir: skillsDir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if len(p.Adapters) != 4 {
		t.Errorf("expected 4 adapters, got %d", len(p.Adapters))
	}
}
