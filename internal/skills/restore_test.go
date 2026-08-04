package skills

import "testing"

func TestRestoreCapabilityAsSkill(t *testing.T) {
	skill := RestoreCapabilityAsSkill("code_search", "Search code using grep/glob")
	if skill.ID != "code-search" { t.Errorf("ID = %s, want code-search", skill.ID) }
	if skill.Body == "" { t.Error("expected non-empty body") }
}
