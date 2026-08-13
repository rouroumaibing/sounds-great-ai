package services

import (
	"sounds-great-ai/internal/a2a"
	sop "sounds-great-ai/internal/sop"
	sopPorts "sounds-great-ai/internal/domains/sop/ports"
)

// SOPGuardianService adapts the flat *sop.SOPGuardian to the hexagonal
// IA2AGuardian port used by the runtime. It owns no new logic — it only
// translates between the port's types (EscalationAction/ReviewPolicy) and the
// concrete sop package types.
type SOPGuardianService struct {
	inner *sop.SOPGuardian
}

// NewSOPGuardianService wraps a concrete guardian in the port interface.
func NewSOPGuardianService(inner *sop.SOPGuardian) *SOPGuardianService {
	return &SOPGuardianService{inner: inner}
}

// CheckA2ADepth delegates to the wrapped guardian, mapping its enum to the
// port's EscalationAction.
func (s *SOPGuardianService) CheckA2ADepth(thread *a2a.Thread) sopPorts.EscalationAction {
	switch s.inner.CheckA2ADepth(thread) {
	case sop.EscalateToCVO:
		return sopPorts.EscalateToCVO
	case sop.Block:
		return sopPorts.Block
	default:
		return sopPorts.Continue
	}
}

// SelectReviewer translates the port policy into the concrete sop policy and
// delegates.
func (s *SOPGuardianService) SelectReviewer(authorBreed string, candidates []string, policy sopPorts.ReviewPolicy) string {
	return sop.SelectReviewer(authorBreed, candidates, sop.ReviewPolicy{
		RequireDifferentBreed: policy.RequireDifferentBreed,
		RequireDifferentCLI:   policy.RequireDifferentCLI,
		PreferredRoles:        policy.PreferredRoles,
		ExcludeUnavailable:    policy.ExcludeUnavailable,
	})
}

// MaxA2ADepth reports the configured depth ceiling.
func (s *SOPGuardianService) MaxA2ADepth() int {
	return s.inner.MaxA2ADepth()
}
