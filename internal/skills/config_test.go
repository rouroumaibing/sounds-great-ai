package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillConfigStoreSetEnabledPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills-config.json")
	store := NewSkillConfigStore(path)

	if store.Enabled("code-search") {
		t.Error("expected code-search disabled by default")
	}
	if err := store.SetEnabled("code-search", true, "project"); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if !store.Enabled("code-search") {
		t.Error("expected code-search enabled after SetEnabled")
	}
	it := store.GetIntent("code-search")
	if it == nil || !it.Enabled || it.Scope != "project" {
		t.Errorf("intent = %+v, want enabled+scope=project", it)
	}

	// 重新从磁盘加载，确认落盘持久化。
	reloaded := NewSkillConfigStore(path)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reloaded.Enabled("code-search") {
		t.Error("expected code-search enabled after reload from disk")
	}
}

func TestSkillConfigStoreMountPointsAndSyncState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills-config.json")
	store := NewSkillConfigStore(path)

	if err := store.SetMountPoints("debugging", []string{"claude", "codex"}); err != nil {
		t.Fatalf("SetMountPoints: %v", err)
	}
	if err := store.SetSyncState("abc123", "2026-08-18T00:00:00Z"); err != nil {
		t.Fatalf("SetSyncState: %v", err)
	}
	it := store.GetIntent("debugging")
	if it == nil || len(it.MountPoints) != 2 {
		t.Fatalf("mountPoints = %+v", it)
	}
	if store.data.Sync.SourceManifestHash != "abc123" {
		t.Errorf("sync hash = %q", store.data.Sync.SourceManifestHash)
	}
}

func TestSkillConfigStoreMissingFileDefaultsEmpty(t *testing.T) {
	store := NewSkillConfigStore(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err := store.Load(); err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}
	if len(store.AllIntents()) != 0 {
		t.Errorf("expected empty intents, got %d", len(store.AllIntents()))
	}
}

func TestSkillManagerEnabledFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("---\nname: A\ndescription: a\n---\nBody A"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("---\nname: B\ndescription: b\n---\nBody B"), 0644)

	cfgPath := filepath.Join(t.TempDir(), "skills-config.json")
	m := NewManagerWithConfig(cfgPath, "", map[string]string{dir: "packs"})
	if err := m.Scan(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(m.All()) != 2 {
		t.Fatalf("expected 2 loaded skills, got %d", len(m.All()))
	}
	if len(m.AllEnabled()) != 0 {
		t.Errorf("expected 0 enabled before SetEnabled, got %d", len(m.AllEnabled()))
	}

	if err := m.SetEnabled("a", true, "project"); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	enabled := m.AllEnabled()
	if len(enabled) != 1 || enabled[0].ID != "a" {
		t.Errorf("AllEnabled = %v, want [a]", idsOf(enabled))
	}

	// 重新构造 manager 并加载磁盘意图，确认启用态持久化。
	m2 := NewManagerWithConfig(cfgPath, "", map[string]string{dir: "packs"})
	if err := m2.Config().Load(); err != nil {
		t.Fatalf("Config.Load: %v", err)
	}
	if err := m2.Scan(); err != nil {
		t.Fatalf("Scan2: %v", err)
	}
	if len(m2.AllEnabled()) != 1 || m2.AllEnabled()[0].ID != "a" {
		t.Errorf("AllEnabled after reload = %v, want [a]", idsOf(m2.AllEnabled()))
	}
}

func TestSkillManagerResolve(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("---\nname: A\ndescription: a\n---\nBody A"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("---\nname: B\ndescription: b\n---\nBody B"), 0644)
	m := NewManager(dir)
	if err := m.Scan(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	resolved := m.Resolve([]string{"a", "missing"})
	if len(resolved) != 1 || resolved[0].ID != "a" {
		t.Errorf("Resolve = %v, want [a]", idsOf(resolved))
	}
	if m.Resolve(nil) != nil {
		t.Error("Resolve(nil) should return nil")
	}
}

func TestSkillSourceTaggedFromDir(t *testing.T) {
	packsDir := t.TempDir()
	userDir := t.TempDir()
	os.WriteFile(filepath.Join(packsDir, "p.md"), []byte("---\nname: P\ndescription: p\n---\nBody"), 0644)
	os.WriteFile(filepath.Join(userDir, "u.md"), []byte("---\nname: U\ndescription: u\n---\nBody"), 0644)
	m := NewManagerWithConfig("", "", map[string]string{packsDir: "packs", userDir: "user"})
	if err := m.Scan(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if p := m.Get("p"); p == nil || p.Source != "packs" {
		t.Errorf("packs skill source = %+v", p)
	}
	if u := m.Get("u"); u == nil || u.Source != "user" {
		t.Errorf("user skill source = %+v", u)
	}
}

func idsOf(skills []*Skill) []string {
	out := make([]string, 0, len(skills))
	for _, s := range skills {
		out = append(out, s.ID)
	}
	return out
}
