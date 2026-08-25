// Package capability defines the CapabilityTip schema and validates its source
// reference (F244). Tips fail closed when the sourceRef is missing or malformed.
package capability

import (
	"fmt"
	"strings"
)

// CapabilityTip is a just-in-time capability hint shown to an agent.
type CapabilityTip struct {
	ID         string
	Capability string
	Message    string
	// SourceRef must reference an existing source, of the form "<kind>:<id>"
	// (e.g. "doc:ft-mem-001" or "skill:foo"). Fail-closed when absent/malformed.
	SourceRef string
}

// ErrInvalidTip is returned when a tip fails validation.
var ErrInvalidTip = fmt.Errorf("capability: invalid tip")

// Validate checks the tip's schema. Fails closed when id/capability/message is
// empty or sourceRef is missing / not "<kind>:<id>".
func (t CapabilityTip) Validate() error {
	if strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("%w: empty id", ErrInvalidTip)
	}
	if strings.TrimSpace(t.Capability) == "" {
		return fmt.Errorf("%w: empty capability", ErrInvalidTip)
	}
	if strings.TrimSpace(t.Message) == "" {
		return fmt.Errorf("%w: empty message", ErrInvalidTip)
	}
	ref := strings.TrimSpace(t.SourceRef)
	if ref == "" {
		return fmt.Errorf("%w: missing sourceRef", ErrInvalidTip)
	}
	if !strings.Contains(ref, ":") {
		return fmt.Errorf("%w: sourceRef must be <kind>:<id>, got %q", ErrInvalidTip, ref)
	}
	return nil
}
