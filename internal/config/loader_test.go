package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	breedJSON := `{
		"id": "bianmu",
		"name": "边牧",
		"display_name": "边牧",
		"default_variant_id": "bianmu-claude",
		"variants": [{"id": "bianmu-claude", "client_id": "anthropic", "default_model": "claude-opus-4-6", "cli": {"command": "claude", "output_format": "stream-json"}}]
	}`
	os.WriteFile(filepath.Join(dir, "bianmu.json"), []byte(breedJSON), 0644)

	loader := NewLoader()
	breeds, err := loader.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(breeds) != 1 {
		t.Fatalf("expected 1 breed, got %d", len(breeds))
	}
	if breeds["bianmu"].ID != "bianmu" {
		t.Errorf("breed ID = %s, want bianmu", breeds["bianmu"].ID)
	}
}

func TestLoaderSkipInvalid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{invalid json`), 0644)
	os.WriteFile(filepath.Join(dir, "good.json"), []byte(`{"id":"good","name":"good","variants":[]}`), 0644)

	loader := NewLoader()
	loader.Policy = LoadPolicySkipInvalid
	breeds, err := loader.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir with skip policy: %v", err)
	}
	if len(breeds) != 1 {
		t.Fatalf("expected 1 breed (bad skipped), got %d", len(breeds))
	}
}
