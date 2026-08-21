package sop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// QCStepResult holds the result of one QC step.
type QCStepResult struct {
	Step    int    `json:"step"`
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// QCLoopResult holds the overall QC loop result plus the state-machine and
// risk-tier metadata added to close the gaps (stateless→stateful, no
// risk-tiering).
type QCLoopResult struct {
	Passed      bool           `json:"passed"`
	Steps       []QCStepResult `json:"steps"`
	Stale       bool           `json:"stale"`
	ReviewedSha string         `json:"reviewed_sha"`
	RiskTier    string         `json:"risk_tier"`
}

// QCLoopInput provides context for the QC loop.
type QCLoopInput struct {
	WorkDir      string
	AuthorBreed  string
	ReviewerBreed string
	// FinalApproverBreed is the Layer-3 final approver (a second
	// named identity, independent of both author and reviewer). Hygiene (Layer 1)
	// is automated by step1 and needs no breed field.
	FinalApproverBreed string
	SpecFile     string
	FeatureName  string
	SpecDir      string
	ChangedFiles []string
	// Fix enables deterministic hygiene auto-fix (gofmt -w).
	Fix bool
	// FixCommit commits auto-fixes with a [qc-bot] signature (implies Fix).
	FixCommit bool
	// ServerMode marks an automated server-heartbeat run. Cross-model review
	// verification (steps 2/3) and guardian sign-off (step 7) are human/merge
	// gates and are treated as advisory so the periodic heartbeat only fails on
	// real hygiene/CI issues rather than missing spec context.
	ServerMode bool
	// SkipHeavy skips the heavy ci_repair build/test step. The server
	// auto-runner sets this so it does not hammer the host with `go test ./...`
	// every interval — build/test stay owned by CI/pre-merge.
	SkipHeavy bool
}

// QCLoop runs the 7-step automated QC pipeline.
type QCLoop struct {
	workDir  string
	guardian *SOPGuardian
	// StatePath overrides the on-disk QC state file (stateless→stateful gap fix).
	StatePath string
}

// NewQCLoop creates a QCLoop.
func NewQCLoop(workDir string) *QCLoop {
	return &QCLoop{
		workDir:   workDir,
		guardian:  NewGuardian(nil, 3),
		StatePath: DefaultQCStatePath(workDir),
	}
}

func (q *QCLoop) resolveStatePath() string {
	if q.StatePath != "" {
		return q.StatePath
	}
	return DefaultQCStatePath(q.workDir)
}

// assessRisk maps changed files to a QC depth (trigger strategy):
//   - unknown (no file list) → "full" (conservative default)
//   - any Go file touched     → "full"  (shared capability, full 7-step)
//   - docs/markdown only      → "light" (hygiene + fresh_context + sign_off)
func assessRisk(changed []string) string {
	if len(changed) == 0 {
		return "full"
	}
	for _, f := range changed {
		if strings.EqualFold(filepath.Ext(f), ".go") {
			return "full"
		}
	}
	return "light"
}

// Run executes the QC pipeline, applying risk-tiering and persisting state.
func (q *QCLoop) Run(input QCLoopInput) QCLoopResult {
	sha := headSHA(input.WorkDir)
	state := LoadQCState(q.resolveStatePath())
	stale := ComputeStale(state, sha)

	tier := assessRisk(input.ChangedFiles)
	steps := []QCStepResult{
		q.step1Hygiene(input),
		q.step2FreshContext(input),
	}
	if tier == "light" {
		skip := func(name string) QCStepResult {
			return QCStepResult{
				Step:    len(steps) + 1,
				Name:    name,
				Passed:  true,
				Message: "skipped (low-risk doc-only change)",
			}
		}
		steps = append(steps, skip("cross_breed_review"), skip("evidence_manifest"), skip("ci_repair"))
	} else {
		steps = append(steps, q.step3CrossBreedReview(input), q.step4EvidenceManifest(input), q.step5CIRepair(input))
	}

	verdict := q.step6Verdict(steps)
	steps = append(steps, verdict)
	signOff := q.step7SignOff(input)
	steps = append(steps, signOff)

	allPassed := true
	for _, s := range steps {
		if !s.Passed {
			allPassed = false
			break
		}
	}

	result := QCLoopResult{Passed: allPassed, Steps: steps, Stale: stale, ReviewedSha: sha, RiskTier: tier}

	// Persist state only inside a real git repo (sha != ""). Outside one (e.g.
	// temp dir in tests) the loop stays stateless and writes nothing.
	if sha != "" {
		state.ReviewedSha = sha
		state.IdempotencyKey = fmt.Sprintf("%s-%d", sha, time.Now().UnixNano())
		state.StaleFlag = stale
		if allPassed {
			state.Phase = "qc.archived"
		} else {
			state.Phase = "qc.verdict_blocked"
		}
		state.LastRun = time.Now()
		_ = SaveQCState(q.resolveStatePath(), state)
	}
	return result
}

// Step 1: Hygiene — detect (default) or auto-fix (--fix). Auto-fix uses gofmt
// (deterministic, allowlist-equivalent for Go); --fix-commit stamps a [qc-bot]
// commit but refuses to swallow pre-existing user WIP (dirty-before guard).
func (q *QCLoop) step1Hygiene(input QCLoopInput) QCStepResult {
	dir := q.workDir
	if dir == "" {
		dir = "."
	}
	if input.Fix {
		if out, err := exec.Command("gofmt", "-w", ".").CombinedOutput(); err != nil {
			return QCStepResult{Step: 1, Name: "hygiene", Passed: false, Message: fmt.Sprintf("gofmt -w failed: %v\n%s", err, out)}
		}
		if input.FixCommit {
			if dirty, _ := runGit(dir, "status", "--porcelain"); strings.TrimSpace(dirty) != "" {
				return QCStepResult{Step: 1, Name: "hygiene", Passed: true, Message: "auto-fixed; worktree dirty before fix, skipped auto-commit"}
			}
			if _, err := runGit(dir, "add", "-u"); err != nil {
				return QCStepResult{Step: 1, Name: "hygiene", Passed: false, Message: fmt.Sprintf("git add failed: %v", err)}
			}
			if _, err := runGit(dir, "commit", "-m", "style: auto-fix hygiene [qc-bot]"); err != nil {
				return QCStepResult{Step: 1, Name: "hygiene", Passed: false, Message: fmt.Sprintf("git commit failed: %v", err)}
			}
			return QCStepResult{Step: 1, Name: "hygiene", Passed: true, Message: "auto-fixed + committed [qc-bot]"}
		}
		return QCStepResult{Step: 1, Name: "hygiene", Passed: true, Message: "auto-fixed (gofmt -w)"}
	}
	cmd := exec.Command("gofmt", "-l", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return QCStepResult{Step: 1, Name: "hygiene", Passed: false, Message: fmt.Sprintf("gofmt failed: %v", err)}
	}
	if strings.TrimSpace(string(out)) != "" {
		return QCStepResult{Step: 1, Name: "hygiene", Passed: false, Message: "files need formatting: " + strings.TrimSpace(string(out))}
	}
	return QCStepResult{Step: 1, Name: "hygiene", Passed: true, Message: "code formatting OK"}
}

// Step 2: Fresh-context review (cross-breed review with no prior context)
func (q *QCLoop) step2FreshContext(input QCLoopInput) QCStepResult {
	if input.ServerMode {
		return QCStepResult{Step: 2, Name: "fresh_context", Passed: true, Message: "server auto-mode: review is a human gate, not auto-verified (advisory)"}
	}
	if input.ReviewerBreed == "" {
		return QCStepResult{Step: 2, Name: "fresh_context", Passed: true, Message: "no reviewer specified (advisory)"}
	}
	if input.ReviewerBreed == input.AuthorBreed {
		return QCStepResult{Step: 2, Name: "fresh_context", Passed: false, Message: "reviewer same as author"}
	}
	return QCStepResult{Step: 2, Name: "fresh_context", Passed: true, Message: "fresh-context review configured"}
}

// Step 3: Cross-breed review (Layer-2 reviewer from different breed) plus
// Layer-3 final-approver independence (three-role split).
func (q *QCLoop) step3CrossBreedReview(input QCLoopInput) QCStepResult {
	if input.ServerMode {
		return QCStepResult{Step: 3, Name: "cross_breed_review", Passed: true, Message: "server auto-mode: cross-model review verified at merge gate (advisory)"}
	}
	if input.AuthorBreed == "" || input.ReviewerBreed == "" {
		return QCStepResult{Step: 3, Name: "cross_breed_review", Passed: true, Message: "no breeds specified (advisory)"}
	}
	if input.AuthorBreed == input.ReviewerBreed {
		return QCStepResult{Step: 3, Name: "cross_breed_review", Passed: false, Message: "reviewer must differ from author (Layer 2)"}
	}
	if input.FinalApproverBreed != "" {
		if input.FinalApproverBreed == input.AuthorBreed {
			return QCStepResult{Step: 3, Name: "cross_breed_review", Passed: false, Message: "final approver must differ from author (Layer 3)"}
		}
		if input.FinalApproverBreed == input.ReviewerBreed {
			return QCStepResult{Step: 3, Name: "cross_breed_review", Passed: false, Message: "final approver must differ from reviewer (Layer 3)"}
		}
	}
	return QCStepResult{Step: 3, Name: "cross_breed_review", Passed: true, Message: "cross-breed review + independent final approver OK"}
}

// Step 4: Evidence manifest (collect all evidence)
func (q *QCLoop) step4EvidenceManifest(input QCLoopInput) QCStepResult {
	dir := q.workDir
	if dir == "" {
		dir = "."
	}
	// Check if there are any changed files
	if len(input.ChangedFiles) == 0 {
		return QCStepResult{Step: 4, Name: "evidence_manifest", Passed: true, Message: "no changed files (advisory)"}
	}
	return QCStepResult{Step: 4, Name: "evidence_manifest", Passed: true, Message: fmt.Sprintf("%d changed files tracked", len(input.ChangedFiles))}
}

// Step 5: CI repair (run go build + go test, report failures).
// Skipped (advisory)
// when workDir is not a Go module so the loop stays usable outside a repo.
func (q *QCLoop) step5CIRepair(input QCLoopInput) QCStepResult {
	if input.SkipHeavy {
		return QCStepResult{Step: 5, Name: "ci_repair", Passed: true, Message: "skipped (server auto-mode: heavy build/test deferred to CI/pre-merge)"}
	}
	dir := q.workDir
	if dir == "" {
		dir = "."
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return QCStepResult{Step: 5, Name: "ci_repair", Passed: true, Message: "no go.mod (advisory, skipped)"}
	}
	// Run go build
	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = dir
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		return QCStepResult{Step: 5, Name: "ci_repair", Passed: false, Message: "go build failed: " + strings.TrimSpace(string(buildOut))}
	}
	// Run go test
	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = dir
	testOut, testErr := testCmd.CombinedOutput()
	if testErr != nil {
		return QCStepResult{Step: 5, Name: "ci_repair", Passed: false, Message: "go test failed: " + strings.TrimSpace(string(testOut))}
	}
	return QCStepResult{Step: 5, Name: "ci_repair", Passed: true, Message: "go build + go test passed"}
}

// Step 6: Verdict — real aggregation of steps 1-5. A failing prior step fails
// the verdict rather than silently passing.
func (q *QCLoop) step6Verdict(prior []QCStepResult) QCStepResult {
	for _, s := range prior {
		if !s.Passed {
			return QCStepResult{
				Step:    6,
				Name:    "verdict",
				Passed:  false,
				Message: fmt.Sprintf("verdict: fail — %s did not pass (%s)", s.Name, s.Message),
			}
		}
	}
	return QCStepResult{Step: 6, Name: "verdict", Passed: true, Message: "verdict: pass — all prior steps passed"}
}

// Step 7: Sign-off (guardian sign-off if all pass)
func (q *QCLoop) step7SignOff(input QCLoopInput) QCStepResult {
	if input.ServerMode {
		return QCStepResult{Step: 7, Name: "sign_off", Passed: true, Message: "server auto-mode: guardian sign-off is a human gate (advisory)"}
	}
	result := q.guardian.SignOff(GuardianInput{
		FeatureName: input.FeatureName,
		SpecDir:     input.SpecDir,
		WorkDir:     q.workDir,
	})
	if result.SignedOff {
		return QCStepResult{Step: 7, Name: "sign_off", Passed: true, Message: "guardian signed off"}
	}
	// Collect failed questions
	var failed []string
	for _, q := range result.Questions {
		if !q.Passed {
			failed = append(failed, q.Question)
		}
	}
	return QCStepResult{Step: 7, Name: "sign_off", Passed: false, Message: "guardian sign-off failed: " + strings.Join(failed, "; ")}
}

// runGit runs a git subcommand in dir and returns combined stdout/stderr.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
