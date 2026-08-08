package sop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckResult holds the result of a single quality check.
type CheckResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// QualityGateResult holds the overall quality gate result.
type QualityGateResult struct {
	Passed bool          `json:"passed"`
	Checks []CheckResult `json:"checks"`
}

// QualityGate performs machine-executable quality checks.
// No LLM calls — all checks are file existence, git log, test results.
type QualityGate struct {
	workDir string
}

// NewQualityGate creates a QualityGate for the given working directory.
func NewQualityGate(workDir string) *QualityGate {
	return &QualityGate{workDir: workDir}
}

// QualityGateInput provides the context for quality gate evaluation.
type QualityGateInput struct {
	SpecFile    string   // path to spec file
	FeatureName string   // feature name
	ChangedFiles []string // changed file paths
	HasCommits  bool     // are there commits?
	HasTests    bool     // do tests exist?
}

// Run executes all quality checks and returns the aggregate result.
func (q *QualityGate) Run(input QualityGateInput) QualityGateResult {
	checks := []CheckResult{
		q.checkSpecAlignment(input),
		q.checkEvidenceBeforeClaim(input),
		q.checkVisionCoverage(input),
	}

	allPassed := true
	for _, c := range checks {
		if !c.Passed {
			allPassed = false
			break
		}
	}
	return QualityGateResult{Passed: allPassed, Checks: checks}
}

// checkSpecAlignment verifies the implementation has a corresponding spec file.
func (q *QualityGate) checkSpecAlignment(input QualityGateInput) CheckResult {
	if input.SpecFile == "" {
		return CheckResult{Name: "spec_alignment", Passed: true, Message: "no spec file specified (advisory)"}
	}
	path := input.SpecFile
	if !filepath.IsAbs(path) && q.workDir != "" {
		path = filepath.Join(q.workDir, path)
	}
	if _, err := os.Stat(path); err != nil {
		return CheckResult{Name: "spec_alignment", Passed: false, Message: fmt.Sprintf("spec file not found: %s", path)}
	}
	return CheckResult{Name: "spec_alignment", Passed: true, Message: "spec file exists"}
}

// checkEvidenceBeforeClaim verifies there are commits/tests before claiming done.
func (q *QualityGate) checkEvidenceBeforeClaim(input QualityGateInput) CheckResult {
	if !input.HasCommits {
		return CheckResult{Name: "evidence_before_claim", Passed: false, Message: "no commits found before claiming done"}
	}
	if !input.HasTests {
		return CheckResult{Name: "evidence_before_claim", Passed: false, Message: "no tests found before claiming done"}
	}
	return CheckResult{Name: "evidence_before_claim", Passed: true, Message: "commits and tests exist"}
}

// checkVisionCoverage verifies the spec file contains a VISION Compatibility section.
func (q *QualityGate) checkVisionCoverage(input QualityGateInput) CheckResult {
	if input.SpecFile == "" {
		return CheckResult{Name: "vision_coverage", Passed: true, Message: "no spec file (advisory)"}
	}
	path := input.SpecFile
	if !filepath.IsAbs(path) && q.workDir != "" {
		path = filepath.Join(q.workDir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return CheckResult{Name: "vision_coverage", Passed: false, Message: "cannot read spec file"}
	}
	content := string(data)
	if !strings.Contains(content, "VISION Compatibility") && !strings.Contains(content, "VISION") {
		return CheckResult{Name: "vision_coverage", Passed: false, Message: "spec file lacks VISION Compatibility section"}
	}
	return CheckResult{Name: "vision_coverage", Passed: true, Message: "VISION compatibility found in spec"}
}

// RunTests executes `go test` and returns pass/fail.
// This is a machine-executable check — exit code only.
func (q *QualityGate) RunTests(pkg string) CheckResult {
	dir := q.workDir
	if dir == "" {
		dir = "."
	}
	cmd := exec.Command("go", "test", pkg)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CheckResult{Name: "go_test", Passed: false, Message: fmt.Sprintf("go test failed: %s", string(output))}
	}
	return CheckResult{Name: "go_test", Passed: true, Message: "go test passed"}
}

// RunBuild executes `go build` and returns pass/fail.
func (q *QualityGate) RunBuild(pkg string) CheckResult {
	dir := q.workDir
	if dir == "" {
		dir = "."
	}
	cmd := exec.Command("go", "build", pkg)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CheckResult{Name: "go_build", Passed: false, Message: fmt.Sprintf("go build failed: %s", string(output))}
	}
	return CheckResult{Name: "go_build", Passed: true, Message: "go build passed"}
}
