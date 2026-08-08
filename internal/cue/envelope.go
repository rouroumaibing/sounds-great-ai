package cue

import (
	"sync"
	"time"
)

// CueScope defines the visibility scope of a cue.
type CueScope string

const (
	ScopeSession CueScope = "session" // visible for this session only
	ScopeThread  CueScope = "thread"  // visible for this thread
	ScopeGlobal  CueScope = "global"  // visible across threads
)

// CueEnvelope is the packaged cue ready for injection into a prompt.
type CueEnvelope struct {
	ID               string    `json:"id"`
	Lane             string    `json:"lane"`
	WhyNow           string    `json:"whyNow"`           // why this cue is relevant now
	Summary          string    `json:"summary"`          // content summary
	SourceCoordinate string    `json:"sourceCoordinate"` // origin of the memory
	DrillHandle      string    `json:"drillHandle"`      // handle for drilling deeper
	Scope            CueScope `json:"scope"`
	Expiry           int64     `json:"expiry"`           // unix milli, 0 = no expiry
	Invalidator      string    `json:"invalidator"`      // what would invalidate this cue
	TokenBudget      int       `json:"tokenBudget"`      // max tokens for this cue
	Timestamp        int64     `json:"timestamp"`
}

// IsExpired checks if the cue has expired.
func (e *CueEnvelope) IsExpired() bool {
	if e.Expiry == 0 {
		return false
	}
	return time.Now().UnixMilli() > e.Expiry
}

// laneBudgets defines the token budget per lane.
var laneBudgets = map[string]int{
	"person":      300,
	"taste":       300,
	"profile":     300,
	"entity":      420,
	"decision":    420,
	"event":       420,
	"lesson":      420,
	"operational": 420,
}

// BudgetForLane returns the token budget for a given lane.
func BudgetForLane(lane string) int {
	if b, ok := laneBudgets[lane]; ok {
		return b
	}
	return 300 // default conservative budget
}

// EnvelopeBuilder builds CueEnvelopes from recall opportunities and memory content.
type EnvelopeBuilder struct {
	mu    sync.Mutex
	seen  map[string]bool // dedup within invocation/session
}

// NewEnvelopeBuilder creates a new EnvelopeBuilder.
func NewEnvelopeBuilder() *EnvelopeBuilder {
	return &EnvelopeBuilder{seen: make(map[string]bool)}
}

// Build creates a CueEnvelope from the given parameters.
// Returns nil if the cue is a duplicate (dedup within session).
func (b *EnvelopeBuilder) Build(id, lane, whyNow, summary, sourceCoord, drillHandle string, scope CueScope, expiry int64, invalidator string) *CueEnvelope {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Dedup: if we've already built an envelope with this ID, skip
	if b.seen[id] {
		return nil
	}
	b.seen[id] = true

	return &CueEnvelope{
		ID:               id,
		Lane:             lane,
		WhyNow:           whyNow,
		Summary:          summary,
		SourceCoordinate: sourceCoord,
		DrillHandle:      drillHandle,
		Scope:            scope,
		Expiry:           expiry,
		Invalidator:      invalidator,
		TokenBudget:      BudgetForLane(lane),
		Timestamp:        time.Now().UnixMilli(),
	}
}

// Reset clears the dedup set (for a new invocation).
func (b *EnvelopeBuilder) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seen = make(map[string]bool)
}

// SeenCount returns the number of unique cues built.
func (b *EnvelopeBuilder) SeenCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.seen)
}
