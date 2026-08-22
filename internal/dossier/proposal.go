package dossier

import (
	"errors"
	"fmt"
	"time"
)

// Distillation proposal state machine:
//
//	pending → approved   (operator approves)
//	pending → rejected   (operator rejects, with reason)
//	approved → applied   (target dog applies the draft to dog-dossier.md)
//
// Separation of duties: createdBy (proposer, any dog/operator) ≠ approvedBy
// (operator) ≠ appliedBy (must equal targetDogId).

// ProposalStatus enumerates proposal states.
type ProposalStatus string

const (
	ProposalPending  ProposalStatus = "pending"
	ProposalApproved ProposalStatus = "approved"
	ProposalRejected ProposalStatus = "rejected"
	ProposalApplied  ProposalStatus = "applied"
)

// EvidenceRefType enumerates admissible evidence kinds.
var evidenceRefTypes = []string{"observation", "review", "trajectory", "operator-comment"}

// IsEvidenceRefType checks the evidence whitelist.
func IsEvidenceRefType(v string) bool {
	for _, t := range evidenceRefTypes {
		if t == v {
			return true
		}
	}
	return false
}

// EvidenceRef anchors a proposal claim to a verifiable artifact.
type EvidenceRef struct {
	Type    string `json:"type"` // observation | review | trajectory | operator-comment
	ID      string `json:"id"`   // anchor id (observation id, review thread id, …)
	Summary string `json:"summary,omitempty"`
}

// DistillationProposal promotes accumulated evidence into the dossier
// summary layer — the only write path into docs/team/dog-dossier.md.
type DistillationProposal struct {
	ProposalID string         `json:"proposalId"`
	Status     ProposalStatus `json:"status"`

	// Trigger
	SourceEvent string `json:"sourceEvent"` // whitelist: DistillationSourceEvents
	SourceID    string `json:"sourceId"`    // idempotency key, unique per event instance

	// Target
	TargetDogID  string   `json:"targetDogId"`
	TargetFields []string `json:"targetFields"` // dossier fields being updated

	// Content
	BeforeSnapshot string        `json:"beforeSnapshot"` // current text, replaced verbatim
	AfterDraft     string        `json:"afterDraft"`     // proposed replacement
	Rationale      string        `json:"rationale"`      // why — shown on the approval card
	EvidenceRefs   []EvidenceRef `json:"evidenceRefs"`   // fail-closed: must be non-empty

	// Safety
	// BaseHash is the SHA-256 of dog-dossier.md at proposal time; apply
	// re-hashes and rejects on mismatch (stale-write optimistic lock).
	BaseHash string `json:"baseHash"`

	// Audit
	CreatedAt        time.Time  `json:"createdAt"`
	CreatedBy        string     `json:"createdBy"`
	ApprovedBy       string     `json:"approvedBy,omitempty"`
	ApprovedAt       *time.Time `json:"approvedAt,omitempty"`
	RejectedBy       string     `json:"rejectedBy,omitempty"`
	RejectedAt       *time.Time `json:"rejectedAt,omitempty"`
	RejectReason     string     `json:"rejectReason,omitempty"`
	AppliedBy        string     `json:"appliedBy,omitempty"`
	AppliedAt        *time.Time `json:"appliedAt,omitempty"`
	AppliedCommitSHA string     `json:"appliedCommitSha,omitempty"`
}

// CreateProposalInput is the validated creation payload.
type CreateProposalInput struct {
	SourceEvent    string
	SourceID       string
	TargetDogID    string
	TargetFields   []string
	BeforeSnapshot string
	AfterDraft     string
	Rationale      string
	EvidenceRefs   []EvidenceRef
	BaseHash       string
	CreatedBy      string
}

// ErrValidation marks structurally invalid proposal input.
var ErrValidation = errors.New("dossier: proposal validation failed")

// ValidateCreateProposal enforces the KD-17 contract. Empty evidenceRefs is
// the fail-closed core: no evidence, no summary-layer change.
func ValidateCreateProposal(in CreateProposalInput) error {
	if !IsDistillationSourceEvent(in.SourceEvent) {
		return fmt.Errorf("%w: sourceEvent %q not in whitelist %v", ErrValidation, in.SourceEvent, DistillationSourceEvents)
	}
	if in.SourceID == "" {
		return fmt.Errorf("%w: sourceId is required", ErrValidation)
	}
	if in.TargetDogID == "" {
		return fmt.Errorf("%w: targetDogId is required", ErrValidation)
	}
	if len(in.TargetFields) == 0 {
		return fmt.Errorf("%w: targetFields must be non-empty", ErrValidation)
	}
	if in.BeforeSnapshot == "" {
		return fmt.Errorf("%w: beforeSnapshot is required", ErrValidation)
	}
	if in.AfterDraft == "" {
		return fmt.Errorf("%w: afterDraft is required", ErrValidation)
	}
	if in.Rationale == "" {
		return fmt.Errorf("%w: rationale is required", ErrValidation)
	}
	if in.BaseHash == "" {
		return fmt.Errorf("%w: baseHash is required", ErrValidation)
	}
	if len(in.EvidenceRefs) == 0 {
		return fmt.Errorf("%w: evidenceRefs must be non-empty (fail-closed)", ErrValidation)
	}
	seen := make(map[string]bool)
	for _, ref := range in.EvidenceRefs {
		if !IsEvidenceRefType(ref.Type) {
			return fmt.Errorf("%w: evidenceRef type %q not in %v", ErrValidation, ref.Type, evidenceRefTypes)
		}
		if ref.ID == "" {
			return fmt.Errorf("%w: evidenceRef id is required", ErrValidation)
		}
		key := ref.Type + ":" + ref.ID
		if seen[key] {
			return fmt.Errorf("%w: duplicate evidenceRef %s", ErrValidation, key)
		}
		seen[key] = true
	}
	return nil
}
