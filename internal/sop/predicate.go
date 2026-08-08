package sop

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// PredicateResult holds the outcome of a predicate evaluation.
type PredicateResult struct {
	Passed  bool
	Message string
	Err     error
}

// PredicateExecutor executes machine-checkable predicates.
// It uses a git command runner for git_state checks and an env getter
// for env_check, enabling testability without real git/env.
type PredicateExecutor struct {
	gitRunner  GitRunner
	envGetter  EnvGetter
	shaSeen    map[string]bool
	shaMu      sync.Mutex
}

// GitRunner runs a git command and returns stdout.
type GitRunner func(dir string, args ...string) (string, error)

// EnvGetter returns an environment variable value.
type EnvGetter func(key string) string

// NewPredicateExecutor creates an executor with default git and env.
func NewPredicateExecutor() *PredicateExecutor {
	return &PredicateExecutor{
		gitRunner: defaultGitRunner,
		envGetter: os.Getenv,
		shaSeen:   make(map[string]bool),
	}
}

// NewPredicateExecutorWithDeps creates an executor with custom dependencies (for testing).
func NewPredicateExecutorWithDeps(git GitRunner, env EnvGetter) *PredicateExecutor {
	return &PredicateExecutor{
		gitRunner: git,
		envGetter: env,
		shaSeen:   make(map[string]bool),
	}
}

func defaultGitRunner(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Execute evaluates a predicate and returns the result.
func (e *PredicateExecutor) Execute(p Predicate, ctx PredicateContext) PredicateResult {
	switch p.Type {
	case "git_state":
		return e.executeGitState(p, ctx)
	case "command_pattern":
		return e.executeCommandPattern(p, ctx)
	case "handle_check":
		return e.executeHandleCheck(p, ctx)
	case "env_check":
		return e.executeEnvCheck(p)
	case "command_sequence":
		return e.executeCommandSequence(p, ctx)
	case "sha_dedup":
		return e.executeShaDedup(p, ctx)
	case "manual_only":
		return PredicateResult{Passed: true, Message: "manual_only: " + p.Reason}
	default:
		return PredicateResult{Passed: true, Message: fmt.Sprintf("unknown predicate type %s, treated as pass", p.Type)}
	}
}

// PredicateContext provides runtime data for predicate evaluation.
type PredicateContext struct {
	WorkDir     string
	Command     string   // the command being checked
	Author      string   // author breed/id
	Reviewer    string   // reviewer breed/id
	AuthorCLI   string   // author CLI
	ReviewerCLI string   // reviewer CLI
	SHA         string   // current SHA
	CommandHistory []string // recent commands
}

// executeGitState checks git ahead/behind/dirty state.
func (e *PredicateExecutor) executeGitState(p Predicate, ctx PredicateContext) PredicateResult {
	for _, check := range p.Checks {
		switch check {
		case "ahead_zero":
			out, err := e.gitRunner(ctx.WorkDir, "rev-list", "--count", "@{u}..HEAD")
			if err != nil {
				return PredicateResult{Passed: false, Message: "git ahead check failed", Err: err}
			}
			if out != "0" {
				return PredicateResult{Passed: false, Message: fmt.Sprintf("ahead=%s, expected 0", out)}
			}
		case "behind_zero":
			out, err := e.gitRunner(ctx.WorkDir, "rev-list", "--count", "HEAD..@{u}")
			if err != nil {
				return PredicateResult{Passed: false, Message: "git behind check failed", Err: err}
			}
			if out != "0" {
				return PredicateResult{Passed: false, Message: fmt.Sprintf("behind=%s, expected 0", out)}
			}
		case "not_dirty":
			out, err := e.gitRunner(ctx.WorkDir, "status", "--porcelain")
			if err != nil {
				return PredicateResult{Passed: false, Message: "git status failed", Err: err}
			}
			if out != "" {
				return PredicateResult{Passed: false, Message: "working tree is dirty"}
			}
		}
	}
	return PredicateResult{Passed: true, Message: "git_state checks passed"}
}

// executeCommandPattern checks if a command matches a regex pattern.
func (e *PredicateExecutor) executeCommandPattern(p Predicate, ctx PredicateContext) PredicateResult {
	if p.MustMatch != "" {
		re, err := regexp.Compile(p.MustMatch)
		if err != nil {
			return PredicateResult{Passed: false, Message: "invalid must_match regex", Err: err}
		}
		if !re.MatchString(ctx.Command) {
			return PredicateResult{Passed: false, Message: fmt.Sprintf("command %q does not match %q", ctx.Command, p.MustMatch)}
		}
	}
	if p.MustNotMatch != "" {
		re, err := regexp.Compile(p.MustNotMatch)
		if err != nil {
			return PredicateResult{Passed: false, Message: "invalid must_not_match regex", Err: err}
		}
		if re.MatchString(ctx.Command) {
			return PredicateResult{Passed: false, Message: fmt.Sprintf("command %q matches forbidden pattern %q", ctx.Command, p.MustNotMatch)}
		}
	}
	return PredicateResult{Passed: true, Message: "command_pattern passed"}
}

// executeHandleCheck checks reviewer ≠ author and cross-breed constraints.
func (e *PredicateExecutor) executeHandleCheck(p Predicate, ctx PredicateContext) PredicateResult {
	switch p.Constraint {
	case "reviewer_not_author":
		if ctx.Reviewer == ctx.Author {
			return PredicateResult{Passed: false, Message: "reviewer is same as author"}
		}
		return PredicateResult{Passed: true, Message: "reviewer ≠ author"}
	case "cross_breed_preferred":
		if ctx.Reviewer == ctx.Author {
			return PredicateResult{Passed: true, Message: "same breed (warn, not blocker)"}
		}
		return PredicateResult{Passed: true, Message: "cross-breed review"}
	case "guardian_handoff_present":
		if ctx.Reviewer == "" {
			return PredicateResult{Passed: false, Message: "no guardian handoff"}
		}
		return PredicateResult{Passed: true, Message: "guardian handoff present"}
	default:
		return PredicateResult{Passed: true, Message: "unknown handle_check constraint: " + p.Constraint}
	}
}

// executeEnvCheck checks environment variable values.
func (e *PredicateExecutor) executeEnvCheck(p Predicate) PredicateResult {
	val := e.envGetter(p.Key)
	if p.MustIncludeString() != "" && !strings.Contains(val, p.MustIncludeString()) {
		return PredicateResult{Passed: false, Message: fmt.Sprintf("env %s=%q does not include %q", p.Key, val, p.MustIncludeString())}
	}
	if p.MustNotInclude != "" && strings.Contains(val, p.MustNotInclude) {
		return PredicateResult{Passed: false, Message: fmt.Sprintf("env %s=%q includes forbidden %q", p.Key, val, p.MustNotInclude)}
	}
	return PredicateResult{Passed: true, Message: "env_check passed"}
}

// executeCommandSequence detects anti-patterns in command history.
func (e *PredicateExecutor) executeCommandSequence(p Predicate, ctx PredicateContext) PredicateResult {
	hist := ctx.CommandHistory

	// Check anti_pattern: all commands in anti_pattern appear in history
	// AND none of the commands in absent appear.
	if len(p.AntiPattern) > 0 {
		allPresent := true
		for _, ap := range p.AntiPattern {
			found := false
			for _, h := range hist {
				if strings.Contains(h, ap) {
					found = true
					break
				}
			}
			if !found {
				allPresent = false
				break
			}
		}
		if allPresent && len(p.Absent) > 0 {
			for _, ab := range p.Absent {
				for _, h := range hist {
					if strings.Contains(h, ab) {
						return PredicateResult{Passed: true, Message: "absent command present, anti-pattern not triggered"}
					}
				}
			}
			return PredicateResult{Passed: false, Message: "anti-pattern detected: " + strings.Join(p.AntiPattern, " → ")}
		}
	}

	// Check must_include: all commands must appear in history
	if len(p.MustIncludeList()) > 0 {
		for _, mi := range p.MustIncludeList() {
			found := false
			for _, h := range hist {
				if strings.Contains(h, mi) {
					found = true
					break
				}
			}
			if !found {
				return PredicateResult{Passed: false, Message: fmt.Sprintf("required command %q not in history", mi)}
			}
		}
	}
	return PredicateResult{Passed: true, Message: "command_sequence passed"}
}

// executeShaDedup checks if a SHA has been seen before.
func (e *PredicateExecutor) executeShaDedup(p Predicate, ctx PredicateContext) PredicateResult {
	e.shaMu.Lock()
	defer e.shaMu.Unlock()
	if e.shaSeen[ctx.SHA] {
		return PredicateResult{Passed: false, Message: fmt.Sprintf("SHA %s already reviewed (dedup)", ctx.SHA)}
	}
	e.shaSeen[ctx.SHA] = true
	return PredicateResult{Passed: true, Message: "SHA not seen before"}
}
