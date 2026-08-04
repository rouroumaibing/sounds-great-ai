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
