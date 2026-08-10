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

func TestLoaderSkipsTemplate(t *testing.T) {
	dir := t.TempDir()
	breedJSON := `{"id":"good","name":"good","variants":[]}`
	// dog-template.json is an array (roster/avatar template), not a BreedConfig.
	templateJSON := `[{"id":"bianmu","name":"边牧","avatar":"bianmu.png"}]`
	os.WriteFile(filepath.Join(dir, "good.json"), []byte(breedJSON), 0644)
	os.WriteFile(filepath.Join(dir, "dog-template.json"), []byte(templateJSON), 0644)

	// Even under FailFast, the template artifact must be skipped, not parsed.
	loader := NewLoader()
	breeds, err := loader.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir with template file: %v", err)
	}
	if len(breeds) != 1 {
		t.Fatalf("expected 1 breed (template skipped), got %d", len(breeds))
	}
	if _, ok := breeds["good"]; !ok {
		t.Errorf("expected breed 'good' to be loaded")
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

func TestLoaderLoadFromFile(t *testing.T) {
	// Reads the real consolidated pack file (dog-template.json).
	path := filepath.Join("..", "..", "packs", "default", "breeds", "dog-template.json")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("consolidated breed file not found: %v", err)
	}

	loader := NewLoader()
	breeds, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if len(breeds) != 6 {
		t.Fatalf("expected 6 breeds, got %d", len(breeds))
	}
	for _, id := range []string{"bianmu", "jinmao", "xigou", "demu", "zangao", "zhonghuatianyuanquan"} {
		if _, ok := breeds[id]; !ok {
			t.Errorf("expected breed %q in consolidated file", id)
		}
	}
}
