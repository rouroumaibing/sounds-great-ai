package skills

import "testing"

func TestRestoreCapabilityAsSkill(t *testing.T) {
	skill := RestoreCapabilityAsSkill("code_search", "Search code using grep/glob")
	if skill.ID != "code-search" { t.Errorf("ID = %s, want code-search", skill.ID) }
	if skill.Body == "" { t.Error("expected non-empty body") }
}

func TestRestoreCapabilityAsSkillEmptyName(t *testing.T) {
	skill := RestoreCapabilityAsSkill("", "some description")
	if skill.ID != "" {
		t.Errorf("ID = %s, want empty", skill.ID)
	}
	if skill.Description != "some description" {
		t.Errorf("Description = %s, want 'some description'", skill.Description)
	}
}

func TestRestoreCapabilityAsSkillEmptyDescription(t *testing.T) {
	skill := RestoreCapabilityAsSkill("code_search", "")
	if skill.ID != "code-search" {
		t.Errorf("ID = %s, want 'code-search'", skill.ID)
	}
	if skill.Description != "" {
		t.Errorf("Description = %s, want empty", skill.Description)
	}
	if skill.Body == "" {
		t.Error("expected non-empty body even with empty description")
	}
}

func TestRestoreCapabilityAsSkillNoUnderscores(t *testing.T) {
	skill := RestoreCapabilityAsSkill("codesearch", "Search code")
	if skill.ID != "codesearch" {
		t.Errorf("ID = %s, want 'codesearch'", skill.ID)
	}
	if skill.Name != "Codesearch" {
		t.Errorf("Name = %s, want 'Codesearch'", skill.Name)
	}
}
