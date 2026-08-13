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
}
