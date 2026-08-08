package sop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sounds-great-ai/internal/a2a"
)

type EscalationAction int

const (
	Continue EscalationAction = iota
	EscalateToCVO
	Block
)

type SOPGate struct {
	ID        string
	Trigger   string
	Condition func(*a2a.Thread) bool
	Action    EscalationAction
}

type SOPGuardian struct {
	gates       []SOPGate
	maxA2ADepth int
}

func NewGuardian(gates []SOPGate, maxA2ADepth int) *SOPGuardian {
	if maxA2ADepth <= 0 {
		maxA2ADepth = 3
	}
	return &SOPGuardian{gates: gates, maxA2ADepth: maxA2ADepth}
}

func (g *SOPGuardian) CheckA2ADepth(thread *a2a.Thread) EscalationAction {
	if thread.ReviewRoundCount >= g.maxA2ADepth {
		return EscalateToCVO
	}
	return Continue
}

func (g *SOPGuardian) MaxA2ADepth() int {
	return g.maxA2ADepth
}

// QuestionResult holds the result of a guardian question check.
type QuestionResult struct {
	Question string `json:"question"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
}

// GuardianResult holds the overall guardian sign-off result.
type GuardianResult struct {
	SignedOff  bool             `json:"signed_off"`
	Questions  []QuestionResult `json:"questions"`
}

// GuardianInput provides context for guardian sign-off.
type GuardianInput struct {
	FeatureName string // feature name for spec lookup
	SpecDir     string // directory containing specs (docs/superpowers/specs/)
	WorkDir     string // working directory for running tests
	TestPkg     string // go test package path
}

// SignOff performs the guardian three questions check.
// All checks are machine-executable (file existence, test exit code, grep).
// No LLM calls — VISION §3 compliance.
func (g *SOPGuardian) SignOff(input GuardianInput) GuardianResult {
	questions := []QuestionResult{
		g.checkHasSpec(input),
		g.checkHasTests(input),
		g.checkVisionCompatibility(input),
	}

	allPassed := true
	for _, q := range questions {
		if !q.Passed {
			allPassed = false
			break
		}
	}
	return GuardianResult{SignedOff: allPassed, Questions: questions}
}

// checkHasSpec: Does feature have spec? (file exists in docs/superpowers/specs/)
func (g *SOPGuardian) checkHasSpec(input GuardianInput) QuestionResult {
	specDir := input.SpecDir
	if specDir == "" {
		specDir = filepath.Join("docs", "superpowers", "specs")
	}
	if input.WorkDir != "" && !filepath.IsAbs(specDir) {
		specDir = filepath.Join(input.WorkDir, specDir)
	}

	// Look for any file containing the feature name
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return QuestionResult{
			Question: "Does feature have spec?",
			Passed:   false,
			Message:  fmt.Sprintf("cannot read spec dir: %v", err),
		}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if input.FeatureName == "" || strings.Contains(entry.Name(), input.FeatureName) {
			return QuestionResult{
				Question: "Does feature have spec?",
				Passed:   true,
				Message:  fmt.Sprintf("spec found: %s", entry.Name()),
			}
		}
	}
	return QuestionResult{
		Question: "Does feature have spec?",
		Passed:   false,
		Message:  "no spec file matching feature name found",
	}
}

// checkHasTests: Does feature have tests? (go test passes)
func (g *SOPGuardian) checkHasTests(input GuardianInput) QuestionResult {
	pkg := input.TestPkg
	if pkg == "" {
		pkg = "./..."
	}
	q := NewQualityGate(input.WorkDir)
	result := q.RunTests(pkg)
	return QuestionResult{
		Question: "Does feature have tests?",
		Passed:   result.Passed,
		Message:  result.Message,
	}
}

// checkVisionCompatibility: Does feature have VISION compatibility? (grep in spec file)
func (g *SOPGuardian) checkVisionCompatibility(input GuardianInput) QuestionResult {
	specDir := input.SpecDir
	if specDir == "" {
		specDir = filepath.Join("docs", "superpowers", "specs")
	}
	if input.WorkDir != "" && !filepath.IsAbs(specDir) {
		specDir = filepath.Join(input.WorkDir, specDir)
	}

	entries, err := os.ReadDir(specDir)
	if err != nil {
		return QuestionResult{
			Question: "Does feature have VISION compatibility?",
			Passed:   false,
			Message:  fmt.Sprintf("cannot read spec dir: %v", err),
		}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if input.FeatureName != "" && !strings.Contains(entry.Name(), input.FeatureName) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(specDir, entry.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "VISION Compatibility") || strings.Contains(content, "VISION") {
			return QuestionResult{
				Question: "Does feature have VISION compatibility?",
				Passed:   true,
				Message:  fmt.Sprintf("VISION compatibility found in %s", entry.Name()),
			}
		}
	}
	return QuestionResult{
		Question: "Does feature have VISION compatibility?",
		Passed:   false,
		Message:  "no VISION compatibility section found in spec",
	}
}

// RequiresGuardianSignOff checks if the completion stage requires guardian sign-off.
// When stage=completion, guardian sign-off is mandatory.
func RequiresGuardianSignOff(stage string) bool {
	return stage == "completion"
}
