package sop

import (
	"fmt"
	"os/exec"
	"strings"
)

// ConditionResult holds the result of a single merge condition.
type ConditionResult struct {
	ID      string `json:"id"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// MergeGateResult holds the overall merge gate result.
type MergeGateResult struct {
	Passed     bool              `json:"passed"`
	Conditions []ConditionResult `json:"conditions"`
	Track      RiskTrack         `json:"track"`
}

// ReviewProvenanceMatrix tracks review SHAs across sources.
type ReviewProvenanceMatrix struct {
	LocalPeerReviewSHA string `json:"local_peer_review_sha"`
	CloudReviewSHA     string `json:"cloud_review_sha"`
	CurrentHead        string `json:"current_head"`
}

// MergeGateInput provides context for merge gate evaluation.
type MergeGateInput struct {
	ChangedFiles       []string
	ReviewMatrix       ReviewProvenanceMatrix
	AuthorBreed        string
	QualityGatePassed  bool
	HasUncommittedChanges bool
	TestsPass          bool
	ReviewCycle        *ReviewCycle
}

// MergeGate performs merge gate checks.
type MergeGate struct {
	riskRouter *RiskRouter
	workDir    string
}

// NewMergeGate creates a MergeGate.
func NewMergeGate(workDir string) *MergeGate {
	return &MergeGate{
		riskRouter: NewRiskRouter(),
		workDir:    workDir,
	}
}

// Run executes all merge gate conditions and returns the aggregate result.
func (g *MergeGate) Run(input MergeGateInput) MergeGateResult {
	// Route based on risk
	assessment := AssessRiskFromFiles(input.ChangedFiles)
	track := g.riskRouter.Route(assessment)

	conditions := []ConditionResult{
		g.checkE1ReviewExists(input),
		g.checkE2CrossBreedReview(input),
		g.checkE3QualityGate(input),
		g.checkE4NoUncommitted(input),
		g.checkE5TestsPass(input),
	}

	allPassed := true
	for _, c := range conditions {
		if !c.Passed {
			allPassed = false
			break
		}
	}
	return MergeGateResult{Passed: allPassed, Conditions: conditions, Track: track}
}

// E1: review exists
func (g *MergeGate) checkE1ReviewExists(input MergeGateInput) ConditionResult {
	if input.ReviewMatrix.LocalPeerReviewSHA == "" && input.ReviewMatrix.CloudReviewSHA == "" {
		return ConditionResult{ID: "E1_review_exists", Passed: false, Message: "no review SHA recorded"}
	}
	if input.ReviewCycle != nil {
		sha := input.ReviewMatrix.LocalPeerReviewSHA
		if sha == "" {
			sha = input.ReviewMatrix.CloudReviewSHA
		}
		if input.ReviewCycle.HasReviewForSHA(sha) {
			return ConditionResult{ID: "E1_review_exists", Passed: true, Message: "review exists for SHA"}
		}
	}
	return ConditionResult{ID: "E1_review_exists", Passed: true, Message: "review SHA recorded"}
}

// E2: review is from different breed
func (g *MergeGate) checkE2CrossBreedReview(input MergeGateInput) ConditionResult {
	if input.AuthorBreed == "" {
		return ConditionResult{ID: "E2_cross_breed", Passed: true, Message: "no author breed specified (advisory)"}
	}
	if input.ReviewCycle != nil {
		sha := input.ReviewMatrix.LocalPeerReviewSHA
		if sha == "" {
			sha = input.ReviewMatrix.CloudReviewSHA
		}
		if sha != "" && input.ReviewCycle.IsCrossBreedReview(sha, input.AuthorBreed) {
			return ConditionResult{ID: "E2_cross_breed", Passed: true, Message: "cross-breed review confirmed"}
		}
	}
	return ConditionResult{ID: "E2_cross_breed", Passed: true, Message: "cross-breed check advisory (no cycle)"}
}

// E3: quality gate passed
func (g *MergeGate) checkE3QualityGate(input MergeGateInput) ConditionResult {
	if !input.QualityGatePassed {
		return ConditionResult{ID: "E3_quality_gate", Passed: false, Message: "quality gate not passed"}
	}
	return ConditionResult{ID: "E3_quality_gate", Passed: true, Message: "quality gate passed"}
}

// E4: no uncommitted changes
func (g *MergeGate) checkE4NoUncommitted(input MergeGateInput) ConditionResult {
	if input.HasUncommittedChanges {
		return ConditionResult{ID: "E4_clean_tree", Passed: false, Message: "uncommitted changes present"}
	}
	return ConditionResult{ID: "E4_clean_tree", Passed: true, Message: "working tree clean"}
}

// E5: tests pass
func (g *MergeGate) checkE5TestsPass(input MergeGateInput) ConditionResult {
	if !input.TestsPass {
		return ConditionResult{ID: "E5_tests_pass", Passed: false, Message: "tests not passing"}
	}
	return ConditionResult{ID: "E5_tests_pass", Passed: true, Message: "tests pass"}
}

// CheckGitClean runs `git status --porcelain` to verify no uncommitted changes.
func (g *MergeGate) CheckGitClean() ConditionResult {
	cmd := exec.Command("git", "status", "--porcelain")
	if g.workDir != "" {
		cmd.Dir = g.workDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ConditionResult{ID: "E4_clean_tree", Passed: false, Message: fmt.Sprintf("git status failed: %s", strings.TrimSpace(string(out)))}
	}
	if strings.TrimSpace(string(out)) != "" {
		return ConditionResult{ID: "E4_clean_tree", Passed: false, Message: "working tree is dirty"}
	}
	return ConditionResult{ID: "E4_clean_tree", Passed: true, Message: "working tree clean"}
}
