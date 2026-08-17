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
	AuthorDogID        string
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

// E2: review independence via dual-route intent. A merge may be gated by a
// local cross-dog review (the peer-review handoff routed back through the
// review cycle) or by an external review that binds the exact target revision
// (the cloud review SHA matches the current head). If the intent is ambiguous —
// neither route supplies valid evidence — the gate fails closed rather than
// admitting an unaudited merge.
func (g *MergeGate) checkE2CrossBreedReview(input MergeGateInput) ConditionResult {
	localSHA := input.ReviewMatrix.LocalPeerReviewSHA
	cloudSHA := input.ReviewMatrix.CloudReviewSHA
	authorID := input.AuthorDogID
	if authorID == "" {
		authorID = input.AuthorBreed
	}

	hasLocal := localSHA != ""
	hasCloud := cloudSHA != ""

	if !hasLocal && !hasCloud {
		return ConditionResult{
			ID:      "E2_cross_breed",
			Passed:  false,
			Message: "review completion intent ambiguous: a local cross-dog review or a same-target external review is required (fail-closed)",
		}
	}

	var msgs []string
	localOK := false
	if hasLocal {
		if authorID == "" {
			// No author identity to compare against; advisory pass.
			localOK = true
		} else if input.ReviewCycle != nil {
			if input.ReviewCycle.IsSelfReview(localSHA, authorID) {
				msgs = append(msgs, "local review performed by author's own dog identity")
			} else if input.ReviewCycle.IsCrossBreedReview(localSHA, authorID) {
				localOK = true
			} else {
				msgs = append(msgs, "local review identity not confirmed cross-dog")
			}
		} else {
			// SHA recorded but no cycle evidence; legacy advisory pass.
			localOK = true
		}
	}

	cloudOK := false
	if hasCloud {
		head := input.ReviewMatrix.CurrentHead
		if head != "" && cloudSHA != head {
			msgs = append(msgs, "external review does not bind the exact target revision")
		} else {
			cloudOK = true
		}
	}

	// Fail closed when neither route validates.
	if !localOK && !cloudOK {
		return ConditionResult{
			ID:      "E2_cross_breed",
			Passed:  false,
			Message: "review independence not established: " + strings.Join(msgs, "; "),
		}
	}
	if len(msgs) > 0 {
		return ConditionResult{
			ID:      "E2_cross_breed",
			Passed:  false,
			Message: "review independence weakened: " + strings.Join(msgs, "; "),
		}
	}
	return ConditionResult{ID: "E2_cross_breed", Passed: true, Message: "cross-model review (local or external) confirmed"}
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
