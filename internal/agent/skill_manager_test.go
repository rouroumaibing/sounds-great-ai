package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillManagerLoadAndGet(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tdd.md"), []byte("# TDD Skill\nWrite tests first."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hard_rails.md"), []byte("# Hard Rails\nSafety rules."), 0644); err != nil {
		t.Fatal(err)
	}
	sm := &SkillManager{}
	if err := sm.Load(dir); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	content, ok := sm.Get("tdd")
	if !ok {
		t.Error("expected to find skill 'tdd'")
	}
	if content == "" {
		t.Error("expected non-empty content for 'tdd'")
	}
	content, ok = sm.Get("hard_rails")
	if !ok {
		t.Error("expected to find skill 'hard_rails'")
	}
	if content == "" {
		t.Error("expected non-empty content for 'hard_rails'")
	}
	_, ok = sm.Get("nonexistent")
	if ok {
		t.Error("expected not to find skill 'nonexistent'")
	}
}

func TestSkillManagerReload(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tdd.md"), []byte("version 1"), 0644); err != nil {
		t.Fatal(err)
	}
	sm := &SkillManager{}
	if err := sm.Load(dir); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	content, _ := sm.Get("tdd")
	if content != "version 1" {
		t.Errorf("expected 'version 1', got '%s'", content)
	}
	if err := os.WriteFile(filepath.Join(dir, "tdd.md"), []byte("version 2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := sm.Reload(dir); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	content, _ = sm.Get("tdd")
	if content != "version 2" {
		t.Errorf("expected 'version 2' after reload, got '%s'", content)
	}
}

func TestSkillManagerLoadEmptyDir(t *testing.T) {
	dir := t.TempDir()
	sm := &SkillManager{}
	if err := sm.Load(dir); err != nil {
		t.Fatalf("Load empty dir should not fail: %v", err)
	}
	_, ok := sm.Get("anything")
	if ok {
		t.Error("expected no skills in empty dir")
	}
}
