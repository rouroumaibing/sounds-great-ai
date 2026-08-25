package services

import (
	"fmt"
	"sync"
)

// CatRoutingError is returned when routing targets a disabled or unavailable
// member (F182). It is a fail-closed rejection, not a transient error.
type CatRoutingError struct {
	DogID  string
	Reason string
}

// Error implements the error interface.
func (e *CatRoutingError) Error() string {
	return fmt.Sprintf("routing to %s rejected: %s", e.DogID, e.Reason)
}

// AvailabilityChecker reports whether a member (dog) is currently enabled.
type AvailabilityChecker interface {
	Enabled(dogID string) bool
}

// MemberAvailability tracks per-dog enable/disable state (F182). An unknown dog
// defaults to enabled: disabling is an explicit operator act, so availability
// failures are opt-in, never silent.
type MemberAvailability struct {
	mu    sync.RWMutex
	state map[string]bool
}

// NewMemberAvailability creates an empty availability tracker.
func NewMemberAvailability() *MemberAvailability {
	return &MemberAvailability{state: make(map[string]bool)}
}

// SetEnabled flips a member's availability. Disabling a member is its
// "disable-impact": subsequent routing to it fails closed with CatRoutingError.
func (m *MemberAvailability) SetEnabled(dogID string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state[dogID] = enabled
}

// Enabled reports availability. Unknown dogs are enabled by default.
func (m *MemberAvailability) Enabled(dogID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.state[dogID]
	return !ok || v
}
