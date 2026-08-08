package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillManagerLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "code-search.md"), []byte("---\nid: code-search\nname: Code Search\ndescription: Search code\ntrigger: finding code\n---\nBody content"), 0644)
	m := NewManager(dir)
	if err := m.LoadFromDir(); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	s := m.Get("code-search")
	if s == nil {
		t.Fatal("expected code-search skill")
	}
	if s.Name != "Code Search" {
		t.Errorf("Name = %s", s.Name)
	}
}

func TestSkillManagerLoadFromDirDirectoryBased(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "debugging")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: debugging\ndescription: Systematic debugging\ntriggers:\n  - \"bug\"\n  - \"test failure\"\n---\n# Debugging\nFind root cause."), 0644)
	m := NewManager(dir)
	if err := m.LoadFromDir(); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	s := m.Get("debugging")
	if s == nil {
		t.Fatal("expected debugging skill from directory")
	}
	if s.Name != "debugging" {
		t.Errorf("Name = %s", s.Name)
	}
}

func TestSkillManagerInjectSkills(t *testing.T) {
	m := NewManager("")
	skills := []*Skill{
		{ID: "code-search", Body: "## Code Search\nUse grep."},
		{ID: "debugging", Body: "## Debugging\nCheck logs."},
	}
	result := m.InjectSkills("You are 边牧.", skills)
	if !strings.Contains(result, "You are 边牧.") {
		t.Error("expected original prompt preserved")
	}
	if !strings.Contains(result, "Code Search") {
		t.Error("expected skill content injected")
	}
	if !strings.Contains(result, "Debugging") {
		t.Error("expected second skill injected")
	}
}

func TestSkillManagerLoadFromNonexistentDir(t *testing.T) {
	m := NewManager("/nonexistent/path/xyz_abc")
	err := m.LoadFromDir()
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestSkillManagerLoadFromEmptyDir(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	if err := m.LoadFromDir(); err != nil {
		t.Fatalf("LoadFromDir on empty dir failed: %v", err)
	}
	if len(m.All()) != 0 {
		t.Errorf("expected 0 skills, got %d", len(m.All()))
	}
}

func TestSkillManagerLoadFromDirSkipsNonMarkdown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "not_a_skill.txt"), []byte("text content"), 0644)
	os.WriteFile(filepath.Join(dir, "valid.md"), []byte("---\nname: Valid\ndescription: valid\n---\nBody"), 0644)
	m := NewManager(dir)
	if err := m.LoadFromDir(); err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}
	if m.Get("valid") == nil {
		t.Error("expected 'valid' skill to be loaded")
	}
}

func TestSkillManagerGetNonexistent(t *testing.T) {
	m := NewManager("")
	if m.Get("nonexistent") != nil {
		t.Error("expected nil for nonexistent skill")
	}
}

func TestSkillManagerInjectSkillsEmpty(t *testing.T) {
	m := NewManager("")
	result := m.InjectSkills("original prompt", nil)
	if result != "original prompt" {
		t.Errorf("expected original prompt, got %q", result)
	}
	result = m.InjectSkills("original prompt", []*Skill{})
	if result != "original prompt" {
		t.Errorf("expected original prompt, got %q", result)
	}
}

func TestSkillManagerAllEmpty(t *testing.T) {
	m := NewManager("")
	all := m.All()
	if len(all) != 0 {
		t.Errorf("expected 0 skills, got %d", len(all))
	}
}
