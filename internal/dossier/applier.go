package dossier

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// ApplyError codes for PrepareDraft failures.
const (
	ErrCodeNotApproved      = "NOT_APPROVED"
	ErrCodeBaseHashMismatch = "BASE_HASH_MISMATCH"
	ErrCodeSectionNotFound  = "SECTION_NOT_FOUND"
	ErrCodeBeforeNotFound   = "BEFORE_SNAPSHOT_NOT_FOUND"
)

// ApplyError is a structured PrepareDraft failure.
type ApplyError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	CurrentHash string `json:"currentHash,omitempty"`
}

func (e *ApplyError) Error() string { return e.Message }

// ApplyDraftResult carries the computed file mutation. The caller
// (execute-apply endpoint) owns the I/O: write file, git commit, mark applied.
type ApplyDraftResult struct {
	ModifiedContent string
	TargetPath      string
	CommitMessage   string
}

// ComputeFileHash is the SHA-256 hex digest used for the baseHash
// stale-write lock. The same function fills baseHash at proposal creation.
func ComputeFileHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// PrepareDraft validates a proposal against the current dossier content and
// computes the modified file. Pure function — no I/O, trivially testable.
//
// Safety properties (ported from the clowder F208 applier, which fixed these
// the hard way):
//   - baseHash optimistic lock: file changed since proposal → reject.
//   - Section anchoring: replacement is bounded to the target dog's
//     `### … dog:{dogId}` section; a matching text in ANOTHER dog's section
//     can never be touched. Fail-closed when the section or the snapshot is
//     not found — no whole-file fallback.
func PrepareDraft(proposal DistillationProposal, currentFileContent string) (ApplyDraftResult, error) {
	if proposal.Status != ProposalApproved {
		return ApplyDraftResult{}, &ApplyError{
			Code:    ErrCodeNotApproved,
			Message: fmt.Sprintf("proposal status is %q, expected approved", proposal.Status),
		}
	}

	currentHash := ComputeFileHash(currentFileContent)
	if currentHash != proposal.BaseHash {
		return ApplyDraftResult{}, &ApplyError{
			Code:        ErrCodeBaseHashMismatch,
			Message:     fmt.Sprintf("dossier changed since proposal (expected %s…, got %s…) — re-propose against the new baseline", proposal.BaseHash[:8], currentHash[:8]),
			CurrentHash: currentHash,
		}
	}

	sectionStart, err := findSectionStart(currentFileContent, proposal.TargetDogID)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	sectionEnd := findSectionEnd(currentFileContent, sectionStart)
	scope := currentFileContent[sectionStart:sectionEnd]

	offset := strings.Index(scope, proposal.BeforeSnapshot)
	if offset < 0 {
		return ApplyDraftResult{}, &ApplyError{
			Code:    ErrCodeBeforeNotFound,
			Message: fmt.Sprintf("beforeSnapshot not found in target dog section (dog:%s) despite baseHash match — proposal may be malformed", proposal.TargetDogID),
		}
	}

	absolute := sectionStart + offset
	modified := currentFileContent[:absolute] + proposal.AfterDraft + currentFileContent[absolute+len(proposal.BeforeSnapshot):]

	commitMessage := strings.Join([]string{
		fmt.Sprintf("docs(FT-DS-001): apply distillation to %s [%s]", proposal.TargetDogID, strings.Join(proposal.TargetFields, ", ")),
		"",
		fmt.Sprintf("Proposal: %s", proposal.ProposalID),
		fmt.Sprintf("Source: %s (%s)", proposal.SourceEvent, proposal.SourceID),
		fmt.Sprintf("Rationale: %s", proposal.Rationale),
		"",
		fmt.Sprintf("Approved by: %s", orUnknown(proposal.ApprovedBy)),
		fmt.Sprintf("Applied by: %s", orUnknown(proposal.AppliedBy)),
		"Applied via distillation pipeline (no push — operator pushes through the normal SOP flow).",
	}, "\n")

	return ApplyDraftResult{
		ModifiedContent: modified,
		TargetPath:      DossierRelativePath,
		CommitMessage:   commitMessage,
	}, nil
}

// findSectionStart locates the `### … dog:{dogId}` heading. Fail-closed when
// missing: no whole-file fallback (an unanchored replace could corrupt
// another dog's section).
func findSectionStart(content, dogID string) (int, error) {
	pattern := regexp.MustCompile(`(?m)^###\s+.*` + "`dog:" + regexp.QuoteMeta(dogID) + "`")
	loc := pattern.FindStringIndex(content)
	if loc == nil {
		return 0, &ApplyError{
			Code:    ErrCodeSectionNotFound,
			Message: fmt.Sprintf("target dog section heading (dog:%s) not found in dossier — refusing unanchored apply", dogID),
		}
	}
	return loc[0], nil
}

// findSectionEnd bounds the section at the next L3 heading (or EOF),
// preventing a malformed snapshot from matching into a later dog's section.
func findSectionEnd(content string, sectionStart int) int {
	rest := content[sectionStart:]
	headerEnd := strings.Index(rest, "\n")
	if headerEnd < 0 {
		return len(content)
	}
	after := rest[headerEnd+1:]
	loc := regexp.MustCompile(`(?m)^###\s`).FindStringIndex(after)
	if loc == nil {
		return len(content)
	}
	return sectionStart + headerEnd + 1 + loc[0]
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
