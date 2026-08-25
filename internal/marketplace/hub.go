// Package marketplace write-side (F146): the Hub is the operator-facing write
// API for registering/updating plugins in the index. Every write is guarded by a
// mutex and a version CAS (concurrent-safe), and every submission is scanned for
// SKILL.md injection before acceptance (fail-closed).
package marketplace

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// SkillScanner inspects a plugin's SKILL.md content and reports any injection
// hits. ok=false means the submission must be rejected (fail-closed).
type SkillScanner interface {
	Scan(skillMD string) (hits []string, ok bool)
}

// ErrCASConflict is returned when a Hub submit loses a CAS race (stale version).
var ErrCASConflict = errors.New("marketplace: CAS conflict (stale version)")

// Hub is the write-side of the marketplace (F146). Operators submit plugins to
// the index; all writes are serialized by a mutex and validated by a version
// CAS, and every submission is scanned for SKILL.md injection.
type Hub struct {
	mu    sync.Mutex
	index map[string]Item
	scan  SkillScanner
}

// NewHub builds a Hub with the given skill scanner. A nil scanner accepts
// everything (test only); production MUST pass a restrictive scanner so
// injection scans are enforced.
func NewHub(scan SkillScanner) *Hub {
	return &Hub{index: make(map[string]Item), scan: scan}
}

// Submit registers or updates a plugin. CAS: if an existing entry has a version
// greater-or-equal to it.Version the submit loses the race (ErrCASConflict).
// A non-empty skillMD is scanned; on injection hits the submit is rejected
// (fail-closed).
func (h *Hub) Submit(it Item, skillMD string) error {
	if skillMD != "" && h.scan != nil {
		hits, ok := h.scan.Scan(skillMD)
		if !ok {
			return fmt.Errorf("marketplace: SKILL.md injection scan failed: %v", hits)
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if prev, ok := h.index[it.ID]; ok && prev.Version >= it.Version {
		return ErrCASConflict
	}
	h.index[it.ID] = it
	return nil
}

// Get returns a submitted plugin by id.
func (h *Hub) Get(id string) (Item, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	it, ok := h.index[id]
	return it, ok
}

// DefaultSkillScanner rejects SKILL.md content containing prompt-injection or
// privilege-escalation markers (fail-closed). Returns ok=false with the matched
// hits when rejected.
type DefaultSkillScanner struct{}

var injectionMarkers = []string{
	"ignore previous instructions",
	"ignore all previous",
	"disregard the system prompt",
	"<!-- inject",
	"system:",
	"you are now",
}

// Scan implements SkillScanner.
func (DefaultSkillScanner) Scan(skillMD string) ([]string, bool) {
	low := strings.ToLower(skillMD)
	var hits []string
	for _, m := range injectionMarkers {
		if strings.Contains(low, m) {
			hits = append(hits, m)
		}
	}
	return hits, len(hits) == 0
}
