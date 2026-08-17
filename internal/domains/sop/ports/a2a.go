package ports

import "sounds-great-ai/internal/a2a"

// EscalationAction mirrors the A2A depth-limit decision surfaced by the
// guardian. It is declared here (rather than imported from the flat sop
// package) so the port stays dependency-free of the concrete adapter, per the
// hexagonal convention used by the other orchestration domains.
type EscalationAction int

const (
	// Continue allows the handoff to proceed.
	Continue EscalationAction = iota
	// EscalateToCVO means the A2A review depth limit was exceeded.
	EscalateToCVO
	// Block hard-stops the handoff.
	Block
)

// ReviewPolicy configures how a reviewer is selected across breeds.
type ReviewPolicy struct {
	RequireDifferentBreed bool
	RequireDifferentCLI  bool
	PreferredRoles       []string
	ExcludeUnavailable   bool
}

// ReviewHandoffVerdict reports the outcome of cross-breed review enforcement
// at an A2A handoff.
type ReviewHandoffVerdict struct {
	// Blocked is true when the handoff violates the cross-breed review policy
	// and must not proceed.
	Blocked bool
	// Messages explains each violation (non-empty only when Blocked).
	Messages []string
}

// ReviewHandoffInput carries the identities in play at a review handoff. Dog
// IDs are the canonical agent identity (the executing variant / model); breed
// is a fallback when dog_id is absent. SessionID identifies the thread so
// review assignment and write-back routing can be tracked per thread.
// Generation, when non-zero, binds the handoff to a specific review lease so a
// stale assignment cannot be reused after reassignment. AuthorVariantID /
// ReviewerVariantID name the executing variant so the gate resolves the model
// identity even when several variants share a breed.
type ReviewHandoffInput struct {
	AuthorBreed      string
	AuthorDogID      string
	AuthorVariantID  string
	ReviewerBreed    string
	ReviewerDogID    string
	ReviewerVariantID string
	SessionID        string
	Generation       uint64
}

// IA2AGuardian is the port for A2A depth limiting and cross-breed reviewer
// selection. It replaces the direct dependency on *sop.SOPGuardian in the
// runtime (execution.go / platform.go) so the SOP domain joins the hexagonal
// boundary like threads/routing/agents/custody.
type IA2AGuardian interface {
	// CheckA2ADepth returns the escalation action for a handoff on thread.
	CheckA2ADepth(thread *a2a.Thread) EscalationAction
	// SelectReviewer picks a reviewer breed from candidates under policy.
	SelectReviewer(authorBreed string, candidates []string, policy ReviewPolicy) string
	// MaxA2ADepth reports the configured depth ceiling.
	MaxA2ADepth() int
	// EnforceReviewHandoff validates that a handoff to a reviewer respects the
	// cross-breed review policy. It fails closed: a dog may not hand its own
	// authored work to itself for review, and a review verdict may only be
	// written back by the assigned reviewer into the direct review thread.
	EnforceReviewHandoff(input ReviewHandoffInput) ReviewHandoffVerdict
}
