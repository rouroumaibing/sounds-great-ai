package sop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefinition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dev.yaml")
	yamlContent := `
id: development
domain: engineering
label: Development SOP
stages:
  - id: kickoff
    label: 立项
    suggested_skill: feat-lifecycle
    hard_rules:
      - id: kickoff-ac
        text: spec must have AC
        severity: blocker
        predicate:
          type: manual_only
          reason: semantic
    pitfalls:
      - id: kickoff-no-confirm
        text: no confirm
        severity: warn
        predicate:
          type: manual_only
  - id: impl
    label: 实现
    hard_rules:
      - id: impl-sync
        text: sync before worktree
        severity: blocker
        predicate:
          type: git_state
          branch: main
          checks: [ahead_zero, behind_zero]
      - id: impl-redis
        text: redis port
        severity: blocker
        predicate:
          type: env_check
          key: REDIS_URL
          must_include: ":6379"
`
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	def, err := LoadDefinition(path)
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}
	if def.ID != "development" {
		t.Errorf("expected ID=development, got %s", def.ID)
	}
	if len(def.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(def.Stages))
	}
	if def.Stages[0].ID != "kickoff" {
		t.Errorf("expected first stage kickoff, got %s", def.Stages[0].ID)
	}
	if len(def.Stages[0].HardRules) != 1 {
		t.Errorf("expected 1 hard rule, got %d", len(def.Stages[0].HardRules))
	}
	if def.Stages[0].HardRules[0].Severity != "blocker" {
		t.Errorf("expected blocker severity")
	}
	if def.Stages[1].HardRules[0].Predicate.Type != "git_state" {
		t.Errorf("expected git_state predicate type")
	}
}

func TestFindStage(t *testing.T) {
	def := &SOPDefinition{
		Stages: []SOPStage{
			{ID: "kickoff"},
			{ID: "impl"},
			{ID: "review"},
		},
	}
	if s := def.FindStage("impl"); s == nil || s.ID != "impl" {
		t.Error("FindStage(impl) failed")
	}
	if s := def.FindStage("nonexistent"); s != nil {
		t.Error("FindStage(nonexistent) should return nil")
	}
}

func TestBlockerRules(t *testing.T) {
	stage := SOPStage{
		HardRules: []HardRule{
			{ID: "r1", Severity: "blocker"},
			{ID: "r2", Severity: "warn"},
			{ID: "r3", Severity: "blocker"},
		},
	}
	blockers := stage.BlockerRules()
	if len(blockers) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(blockers))
	}
}

func TestStageIDs(t *testing.T) {
	def := &SOPDefinition{
		Stages: []SOPStage{
			{ID: "kickoff"},
			{ID: "impl"},
			{ID: "review"},
		},
	}
	ids := def.StageIDs()
	if len(ids) != 3 || ids[0] != "kickoff" || ids[2] != "review" {
		t.Errorf("StageIDs = %v", ids)
	}
}

func TestLoadDefinitionFileNotFound(t *testing.T) {
	_, err := LoadDefinition("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadActualDevelopmentYAML(t *testing.T) {
	path := filepath.Join("..", "..", "packs", "default", "sop", "development.yaml")
	def, err := LoadDefinition(path)
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}
	if def.ID != "development" {
		t.Errorf("expected ID=development, got %s", def.ID)
	}
	expectedStages := []string{"kickoff", "impl", "quality_gate", "fresh_context", "review", "merge", "completion"}
	if len(def.Stages) != len(expectedStages) {
		t.Fatalf("expected %d stages, got %d", len(expectedStages), len(def.Stages))
	}
	for i, expected := range expectedStages {
		if def.Stages[i].ID != expected {
			t.Errorf("stage %d: expected %s, got %s", i, expected, def.Stages[i].ID)
		}
	}
}
