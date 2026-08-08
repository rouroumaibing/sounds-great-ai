package agent

import (
	"os"
	"path/filepath"
	"sync"
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

func TestSkillManagerLoadNonexistentPath(t *testing.T) {
	sm := &SkillManager{}
	err := sm.Load("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error loading nonexistent path")
	}
}

func TestSkillManagerLoadWithSubdirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "my-skill")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "SKILL.md"), []byte("# My Skill\nContent."), 0644); err != nil {
		t.Fatal(err)
	}
	sm := &SkillManager{}
	if err := sm.Load(dir); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	content, ok := sm.Get("my-skill")
	if !ok {
		t.Fatal("expected to find skill from subdirectory")
	}
	if content == "" {
		t.Error("expected non-empty content for subdirectory skill")
	}
}

func TestSkillManagerLoadSubdirWithoutSkillMd(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "incomplete-skill")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	sm := &SkillManager{}
	if err := sm.Load(dir); err != nil {
		t.Fatalf("Load should not fail with subdir missing SKILL.md: %v", err)
	}
	_, ok := sm.Get("incomplete-skill")
	if ok {
		t.Error("expected skill not loaded when SKILL.md is missing")
	}
}

func TestSkillManagerLoadNonMdFileSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a skill"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "valid.md"), []byte("# Valid Skill"), 0644); err != nil {
		t.Fatal(err)
	}
	sm := &SkillManager{}
	if err := sm.Load(dir); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	_, ok := sm.Get("readme")
	if ok {
		t.Error("expected non-.md file to be skipped")
	}
	content, ok := sm.Get("valid")
	if !ok {
		t.Fatal("expected valid.md to be loaded")
	}
	if content != "# Valid Skill" {
		t.Errorf("expected '# Valid Skill', got '%s'", content)
	}
}

func TestSkillManagerGetNotFound(t *testing.T) {
	sm := &SkillManager{}
	_, ok := sm.Get("anything")
	if ok {
		t.Error("expected Get to return false for unloaded skill")
	}
}

func TestSkillManagerConcurrentGetAndLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "skill1.md"), []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill2.md"), []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}
	sm := &SkillManager{}
	if err := sm.Load(dir); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = sm.Get("skill1")
		}()
		go func() {
			defer wg.Done()
			_ = sm.Load(dir)
		}()
	}
	wg.Wait()
	content, ok := sm.Get("skill1")
	if !ok || content != "content1" {
		t.Errorf("expected skill1 content 'content1' after concurrent access, got ok=%v content=%s", ok, content)
	}
}

func TestSkillManagerReloadClearsOldSkills(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir1, "only-in-dir1.md"), []byte("d1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "only-in-dir2.md"), []byte("d2"), 0644); err != nil {
		t.Fatal(err)
	}
	sm := &SkillManager{}
	if err := sm.Load(dir1); err != nil {
		t.Fatal(err)
	}
	if _, ok := sm.Get("only-in-dir1"); !ok {
		t.Error("expected only-in-dir1 to be loaded")
	}
	if err := sm.Reload(dir2); err != nil {
		t.Fatal(err)
	}
	if _, ok := sm.Get("only-in-dir1"); ok {
		t.Error("expected only-in-dir1 to be cleared after reload from different dir")
	}
	if _, ok := sm.Get("only-in-dir2"); !ok {
		t.Error("expected only-in-dir2 to be loaded after reload")
	}
}
