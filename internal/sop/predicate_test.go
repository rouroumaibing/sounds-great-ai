package sop

import (
	"testing"
)

func mockGit(dir string, args ...string) (string, error) {
	// Simulate git rev-list --count @{u}..HEAD = 0
	if len(args) > 2 && args[0] == "rev-list" {
		return "0", nil
	}
	// Simulate git status --porcelain = empty (clean)
	if args[0] == "status" {
		return "", nil
	}
	return "", nil
}

func mockEnv(key string) string {
	if key == "REDIS_URL" {
		return "redis://localhost:6379"
	}
	return ""
}

func TestExecuteGitState(t *testing.T) {
	pe := NewPredicateExecutorWithDeps(mockGit, mockEnv)
	p := Predicate{Type: "git_state", Branch: "main", Checks: []string{"ahead_zero", "behind_zero"}}
	result := pe.Execute(p, PredicateContext{WorkDir: "."})
	if !result.Passed {
		t.Errorf("expected pass, got: %s (err: %v)", result.Message, result.Err)
	}
}

func TestExecuteGitStateDirty(t *testing.T) {
	dirtyGit := func(dir string, args ...string) (string, error) {
		if args[0] == "status" {
			return "M somefile.go", nil
		}
		return "0", nil
	}
	pe := NewPredicateExecutorWithDeps(dirtyGit, mockEnv)
	p := Predicate{Type: "git_state", Checks: []string{"not_dirty"}}
	result := pe.Execute(p, PredicateContext{})
	if result.Passed {
		t.Error("expected fail for dirty tree")
	}
}

func TestExecuteCommandPattern(t *testing.T) {
	pe := NewPredicateExecutor()
	p := Predicate{Type: "command_pattern", MustMatch: "go test|go build"}
	result := pe.Execute(p, PredicateContext{Command: "go test ./..."})
	if !result.Passed {
		t.Errorf("expected pass: %s", result.Message)
	}
	result = pe.Execute(p, PredicateContext{Command: "npm run dev"})
	if result.Passed {
		t.Error("expected fail for non-matching command")
	}
}

func TestExecuteCommandPatternMustNotMatch(t *testing.T) {
	pe := NewPredicateExecutor()
	p := Predicate{Type: "command_pattern", MustMatch: "git merge.*--squash", MustNotMatch: "git reset --soft"}
	result := pe.Execute(p, PredicateContext{Command: "git merge --squash feature"})
	if !result.Passed {
		t.Errorf("expected pass: %s", result.Message)
	}
	result = pe.Execute(p, PredicateContext{Command: "git reset --soft"})
	if result.Passed {
		t.Error("expected fail for forbidden pattern")
	}
}

func TestExecuteHandleCheckReviewerNotAuthor(t *testing.T) {
	pe := NewPredicateExecutor()
	p := Predicate{Type: "handle_check", Constraint: "reviewer_not_author"}
	result := pe.Execute(p, PredicateContext{Author: "bianmu", Reviewer: "xigou"})
	if !result.Passed {
		t.Errorf("expected pass: %s", result.Message)
	}
	result = pe.Execute(p, PredicateContext{Author: "bianmu", Reviewer: "bianmu"})
	if result.Passed {
		t.Error("expected fail: reviewer == author")
	}
}

func TestExecuteHandleCheckGuardianHandoff(t *testing.T) {
	pe := NewPredicateExecutor()
	p := Predicate{Type: "handle_check", Constraint: "guardian_handoff_present"}
	result := pe.Execute(p, PredicateContext{Reviewer: "demu"})
	if !result.Passed {
		t.Errorf("expected pass: %s", result.Message)
	}
	result = pe.Execute(p, PredicateContext{Reviewer: ""})
	if result.Passed {
		t.Error("expected fail for empty guardian")
	}
}

func TestExecuteEnvCheck(t *testing.T) {
	pe := NewPredicateExecutorWithDeps(mockGit, mockEnv)
	p := Predicate{Type: "env_check", Key: "REDIS_URL", MustInclude: ":6379"}
	result := pe.Execute(p, PredicateContext{})
	if !result.Passed {
		t.Errorf("expected pass: %s", result.Message)
	}
}

func TestExecuteEnvCheckFail(t *testing.T) {
	badEnv := func(key string) string {
		if key == "REDIS_URL" {
			return "redis://localhost:6399"
		}
		return ""
	}
	pe := NewPredicateExecutorWithDeps(mockGit, badEnv)
	p := Predicate{Type: "env_check", Key: "REDIS_URL", MustInclude: ":6379"}
	result := pe.Execute(p, PredicateContext{})
	if result.Passed {
		t.Error("expected fail for wrong port")
	}
}

func TestExecuteCommandSequenceAntiPattern(t *testing.T) {
	pe := NewPredicateExecutor()
	p := Predicate{
		Type:        "command_sequence",
		AntiPattern: []string{"git push", "gh pr close"},
		Absent:      []string{"gh pr merge"},
	}
	hist := []string{"git add .", "git push origin main", "gh pr close 123"}
	result := pe.Execute(p, PredicateContext{CommandHistory: hist})
	if result.Passed {
		t.Error("expected fail: anti-pattern detected")
	}
}

func TestExecuteCommandSequenceMustInclude(t *testing.T) {
	pe := NewPredicateExecutor()
	p := Predicate{
		Type:        "command_sequence",
		MustInclude: []any{"git log", "go test"},
	}
	hist := []string{"git log --oneline", "go test ./..."}
	result := pe.Execute(p, PredicateContext{CommandHistory: hist})
	if !result.Passed {
		t.Errorf("expected pass: %s", result.Message)
	}
	hist = []string{"git log --oneline"}
	result = pe.Execute(p, PredicateContext{CommandHistory: hist})
	if result.Passed {
		t.Error("expected fail: missing go test")
	}
}

func TestExecuteShaDedup(t *testing.T) {
	pe := NewPredicateExecutor()
	p := Predicate{Type: "sha_dedup", Scope: "review"}
	result := pe.Execute(p, PredicateContext{SHA: "abc123"})
	if !result.Passed {
		t.Errorf("expected pass first time: %s", result.Message)
	}
	result = pe.Execute(p, PredicateContext{SHA: "abc123"})
	if result.Passed {
		t.Error("expected fail: SHA already seen")
	}
	result = pe.Execute(p, PredicateContext{SHA: "def456"})
	if !result.Passed {
		t.Errorf("expected pass for new SHA: %s", result.Message)
	}
}

func TestExecuteManualOnly(t *testing.T) {
	pe := NewPredicateExecutor()
	p := Predicate{Type: "manual_only", Reason: "semantic check"}
	result := pe.Execute(p, PredicateContext{})
	if !result.Passed {
		t.Error("manual_only should always pass")
	}
}
