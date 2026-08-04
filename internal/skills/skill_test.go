package skills

import (
	"reflect"
	"testing"
)

func TestSkillFrontmatterParse(t *testing.T) {
	raw := "---\nid: code-search\nname: Code Search\ndescription: Search codebases using ripgrep\ntrigger: when task involves finding code\n---\n\n# Code Search Skill\n\nYou have access to search tools."
	s, err := ParseSkill("code-search.md", []byte(raw))
	if err != nil {
		t.Fatalf("ParseSkill: %v", err)
	}
	if s.ID != "code-search" {
		t.Errorf("ID = %s", s.ID)
	}
	if s.Name != "Code Search" {
		t.Errorf("Name = %s", s.Name)
	}
	if s.Body != "# Code Search Skill\n\nYou have access to search tools." {
		t.Errorf("Body = %q", s.Body)
	}
}

func TestSkillTriggersArray(t *testing.T) {
	raw := "---\nname: debugging\ndescription: >\n  Systematic bug localization.\n  Use when: bug, test failure.\ntriggers:\n  - \"bug\"\n  - \"test failure\"\n  - \"unexpected behavior\"\n---\n\n# Debugging\n\nFind root cause."
	s, err := ParseSkill("debugging.md", []byte(raw))
	if err != nil {
		t.Fatalf("ParseSkill: %v", err)
	}
	if s.ID != "debugging" {
		t.Errorf("ID = %s, want debugging", s.ID)
	}
	if s.Name != "debugging" {
		t.Errorf("Name = %s", s.Name)
	}
	want := []string{"bug", "test failure", "unexpected behavior"}
	if !reflect.DeepEqual(s.Triggers, want) {
		t.Errorf("Triggers = %v, want %v", s.Triggers, want)
	}
	all := s.AllTriggers()
	if !reflect.DeepEqual(all, want) {
		t.Errorf("AllTriggers = %v, want %v", all, want)
	}
}

func TestSkillIDFallbackFromName(t *testing.T) {
	raw := "---\nname: Thread Orchestration\ndescription: Multi-thread orchestration\n---\n\nBody"
	s, err := ParseSkill("some-file.md", []byte(raw))
	if err != nil {
		t.Fatalf("ParseSkill: %v", err)
	}
	if s.ID != "thread-orchestration" {
		t.Errorf("ID = %s, want thread-orchestration", s.ID)
	}
}

func TestSkillIDFallbackFromFilename(t *testing.T) {
	raw := "---\nname: Some Skill\n---\n\nBody"
	s, err := ParseSkill("custom-skill.md", []byte(raw))
	if err != nil {
		t.Fatalf("ParseSkill: %v", err)
	}
	if s.ID != "some-skill" {
		t.Errorf("ID = %s, want some-skill", s.ID)
	}
}

func TestSkillSKILLmdIDFallback(t *testing.T) {
	raw := "---\nname: Memory Navigation\n---\n\nBody"
	s, err := ParseSkill("skills/memory-navigation/SKILL.md", []byte(raw))
	if err != nil {
		t.Fatalf("ParseSkill: %v", err)
	}
	if s.ID != "memory-navigation" {
		t.Errorf("ID = %s, want memory-navigation", s.ID)
	}
}

func TestSkillAllTriggersSingularFallback(t *testing.T) {
	s := &Skill{Trigger: "when debugging"}
	all := s.AllTriggers()
	if len(all) != 1 || all[0] != "when debugging" {
		t.Errorf("AllTriggers = %v", all)
	}
}
