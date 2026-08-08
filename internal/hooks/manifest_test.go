package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseHookManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `id: S1
name: Identity
stage: session-init
order: 100
version: 1
enabled: true
disableable: false
template: template.md
resolver: IdentityResolver
inputs: [breedId, breedName]
safetyTier: readonly
governanceTier: immutable
`
	os.WriteFile(filepath.Join(dir, "hook.yaml"), []byte(yamlContent), 0644)
	os.WriteFile(filepath.Join(dir, "template.md"), []byte("Hello {{.BreedName}}"), 0644)

	m, err := ParseHookManifest(dir)
	if err != nil {
		t.Fatalf("ParseHookManifest: %v", err)
	}
	if m.ID != "S1" {
		t.Errorf("ID = %q, want S1", m.ID)
	}
	if m.Stage != "session-init" {
		t.Errorf("Stage = %q, want session-init", m.Stage)
	}
	if m.Order != 100 {
		t.Errorf("Order = %d, want 100", m.Order)
	}
	if m.Disableable != false {
		t.Error("Disableable should be false")
	}
}

func TestParseHookManifest_MissingTemplate(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `id: S1
name: Test
stage: session-init
order: 100
version: 1
enabled: true
disableable: false
template: missing.md
safetyTier: readonly
governanceTier: immutable
`
	os.WriteFile(filepath.Join(dir, "hook.yaml"), []byte(yamlContent), 0644)

	_, err := ParseHookManifest(dir)
	if err == nil {
		t.Error("expected error for missing template")
	}
}
