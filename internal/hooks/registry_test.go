package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestHookRegistry_Scan(t *testing.T) {
	dir := t.TempDir()
	createTestHook(t, dir, "s1-identity", "S1", "session-init", 100)
	createTestHook(t, dir, "s2-restrictions", "S2", "session-init", 200)
	createTestHook(t, dir, "d1-anchor", "D1", "per-turn", 100)

	r := NewRegistry(dir)
	if err := r.Scan(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(r.hooks) != 3 {
		t.Errorf("hooks count = %d, want 3", len(r.hooks))
	}
}

func TestHookRegistry_GetStageHooks(t *testing.T) {
	dir := t.TempDir()
	createTestHook(t, dir, "s1", "S1", "session-init", 100)
	createTestHook(t, dir, "s2", "S2", "session-init", 200)
	createTestHook(t, dir, "d1", "D1", "per-turn", 100)

	r := NewRegistry(dir)
	r.Scan()

	sessionHooks := r.GetStageHooks("session-init")
	if len(sessionHooks) != 2 {
		t.Fatalf("session-init hooks = %d, want 2", len(sessionHooks))
	}
	if sessionHooks[0].Manifest.ID != "S1" {
		t.Errorf("first hook = %q, want S1 (ordered by order)", sessionHooks[0].Manifest.ID)
	}

	turnHooks := r.GetStageHooks("per-turn")
	if len(turnHooks) != 1 {
		t.Errorf("per-turn hooks = %d, want 1", len(turnHooks))
	}
}

func createTestHook(t *testing.T, base, dirname, id, stage string, order int) {
	t.Helper()
	dir := filepath.Join(base, dirname)
	os.MkdirAll(dir, 0755)
	yaml := fmt.Sprintf(`id: %s
name: test
stage: %s
order: %d
version: 1
enabled: true
disableable: false
template: template.md
safetyTier: readonly
governanceTier: immutable
`, id, stage, order)
	os.WriteFile(filepath.Join(dir, "hook.yaml"), []byte(yaml), 0644)
	os.WriteFile(filepath.Join(dir, "template.md"), []byte("test"), 0644)
}
