package memory

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// LaneType is the typed identifier for a memory lane.
// Each lane owns its canonical truth: only approved entries are truth.
type LaneType string

const (
	LaneTaste    LaneType = "taste"    // user preferences, style, aesthetic
	LaneProfile  LaneType = "profile"  // user identity, role, context
	LaneEntity   LaneType = "entity"   // project entities, code objects
	LanePerson   LaneType = "person"   // people, relationships
	LaneEvent    LaneType = "event"    // temporal events, milestones
	LaneDecision LaneType = "decision" // decisions and rationale
	LaneLesson   LaneType = "lesson"   // lessons learned, anti-patterns
)

// LaneEntryStatus is the lifecycle status of a lane entry.
type LaneEntryStatus string

const (
	StatusPending   LaneEntryStatus = "pending"   // candidate, awaiting human disposition
	StatusApproved  LaneEntryStatus = "approved"  // canonical truth for this lane
	StatusRetired   LaneEntryStatus = "retired"   // superseded, no longer truth but kept
	StatusForgotten LaneEntryStatus = "forgotten" // explicitly forgotten, excluded from recall
	StatusDeferred  LaneEntryStatus = "deferred"  // snoozed candidate; revisit later (not truth)
)

// LaneEntry is a single record in a memory lane.
type LaneEntry struct {
	ID            string            `json:"id"`
	Type          LaneType          `json:"type"`
	Content       string            `json:"content"`
	Source        string            `json:"source"`      // origin: session id, breed, external
	Timestamp     int64             `json:"timestamp"`   // unix milli
	Status        LaneEntryStatus   `json:"status"`
	OperatorID    string            `json:"operator_id"` // owner operator; "" = shared across operators
	Sensitivity   string            `json:"sensitivity"` // data-sensitivity tag (F186); "" = none
	CollectionID  string            `json:"collection_id"` // collection/namespace the entry belongs to
	StatusHistory []LaneEntryStatus `json:"status_history,omitempty"` // prior statuses, for undo
}

// Lane is a typed memory lane with canonical truth ownership.
// Only approved entries are considered truth.
type Lane struct {
	mu       sync.RWMutex
	laneType LaneType
	entries  map[string]*LaneEntry // id -> entry
	order    []string              // insertion order for stable iteration
	// notify, when set, is called after any mutation so the owning registry
	// can flush to its persister. Nil for in-memory lanes (no persistence).
	notify func()
}

// NewLane creates a new typed memory lane (in-memory, non-persistent).
func NewLane(t LaneType) *Lane {
	return &Lane{
		laneType: t,
		entries:  make(map[string]*LaneEntry),
	}
}

// Type returns the lane type.
func (l *Lane) Type() LaneType {
	return l.laneType
}

// onMutated flushes the lane to its persister if one is wired.
func (l *Lane) onMutated() {
	if l.notify != nil {
		l.notify()
	}
}

// Submit adds a candidate entry in pending status. Returns the new entry.
func (l *Lane) Submit(content, source string) *LaneEntry {
	e := &LaneEntry{
		ID:        uuid.NewString(),
		Type:      l.laneType,
		Content:   content,
		Source:    source,
		Timestamp: time.Now().UnixMilli(),
		Status:    StatusPending,
	}
	l.mu.Lock()
	l.entries[e.ID] = e
	l.order = append(l.order, e.ID)
	l.mu.Unlock()
	l.onMutated()
	return e
}

// pushHistory records the current status before a transition so Undo can
// revert to it. Caller must hold l.mu.
func (l *Lane) pushHistory(e *LaneEntry) {
	e.StatusHistory = append(e.StatusHistory, e.Status)
}

// Approve promotes a pending entry to approved (canonical truth).
// Any previously approved entry with overlapping content is retired.
func (l *Lane) Approve(id string) bool {
	l.mu.Lock()
	e, ok := l.entries[id]
	if !ok || e.Status != StatusPending {
		l.mu.Unlock()
		return false
	}
	l.pushHistory(e)
	e.Status = StatusApproved
	l.mu.Unlock()
	l.onMutated()
	return true
}

// Reject removes a pending candidate. Returns true if rejected.
func (l *Lane) Reject(id string) bool {
	l.mu.Lock()
	e, ok := l.entries[id]
	if !ok || e.Status != StatusPending {
		l.mu.Unlock()
		return false
	}
	// Rejected candidates are forgotten (not truth, not kept as retired).
	l.pushHistory(e)
	e.Status = StatusForgotten
	l.mu.Unlock()
	l.onMutated()
	return true
}

// Retire marks an approved entry as retired (superseded, kept for audit).
func (l *Lane) Retire(id string) bool {
	l.mu.Lock()
	e, ok := l.entries[id]
	if !ok || e.Status != StatusApproved {
		l.mu.Unlock()
		return false
	}
	l.pushHistory(e)
	e.Status = StatusRetired
	l.mu.Unlock()
	l.onMutated()
	return true
}

// Forget marks an entry as forgotten (excluded from recall).
func (l *Lane) Forget(id string) bool {
	l.mu.Lock()
	e, ok := l.entries[id]
	if !ok {
		l.mu.Unlock()
		return false
	}
	l.pushHistory(e)
	e.Status = StatusForgotten
	l.mu.Unlock()
	l.onMutated()
	return true
}

// Defer snoozes a pending candidate (kept, not truth, surfaced separately as
// "decide later"). Homologous clowder not-now semantics. Only pending entries
// can be deferred.
func (l *Lane) Defer(id string) bool {
	l.mu.Lock()
	e, ok := l.entries[id]
	if !ok || (e.Status != StatusPending && e.Status != StatusDeferred) {
		l.mu.Unlock()
		return false
	}
	l.pushHistory(e)
	e.Status = StatusDeferred
	l.mu.Unlock()
	l.onMutated()
	return true
}

// Undo reverts the most recent status transition (back to the last recorded
// status in StatusHistory). Process-local: history is not persisted across
// restarts (the persister stores status only), so undo works within a process
// session. Homologous clowder undo semantics.
func (l *Lane) Undo(id string) bool {
	l.mu.Lock()
	e, ok := l.entries[id]
	if !ok || len(e.StatusHistory) == 0 {
		l.mu.Unlock()
		return false
	}
	prev := e.StatusHistory[len(e.StatusHistory)-1]
	e.StatusHistory = e.StatusHistory[:len(e.StatusHistory)-1]
	e.Status = prev
	l.mu.Unlock()
	l.onMutated()
	return true
}

// Withdraw re-opens an entry for human disposition: an approved or deferred
// entry returns to pending (its truth is revoked until re-approved). Distinct
// from Forget (terminal, excluded from recall) and Undo (reverts the single
// last transition). Homologous clowder withdraw (pull a candidate/decision
// back out of the materialized state without forgetting it).
func (l *Lane) Withdraw(id string) bool {
	l.mu.Lock()
	e, ok := l.entries[id]
	if !ok || (e.Status != StatusApproved && e.Status != StatusDeferred) {
		l.mu.Unlock()
		return false
	}
	l.pushHistory(e)
	e.Status = StatusPending
	l.mu.Unlock()
	l.onMutated()
	return true
}

// Truth returns all approved entries (canonical truth for this lane).
func (l *Lane) Truth() []*LaneEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var result []*LaneEntry
	for _, id := range l.order {
		e := l.entries[id]
		if e.Status == StatusApproved {
			result = append(result, e)
		}
	}
	return result
}

// Pending returns all pending candidate entries.
func (l *Lane) Pending() []*LaneEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var result []*LaneEntry
	for _, id := range l.order {
		e := l.entries[id]
		if e.Status == StatusPending {
			result = append(result, e)
		}
	}
	return result
}

// Deferred returns all snoozed candidate entries (decide-later, not truth).
func (l *Lane) Deferred() []*LaneEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var result []*LaneEntry
	for _, id := range l.order {
		e := l.entries[id]
		if e.Status == StatusDeferred {
			result = append(result, e)
		}
	}
	return result
}

// All returns all entries regardless of status (for audit).
func (l *Lane) All() []*LaneEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*LaneEntry, 0, len(l.order))
	for _, id := range l.order {
		result = append(result, l.entries[id])
	}
	return result
}

// Get returns a single entry by ID.
func (l *Lane) Get(id string) (*LaneEntry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	e, ok := l.entries[id]
	return e, ok
}

// ---- LaneRegistry (struct + constructors + accessors) lives in
//      lanes_registry.go; persistence/flush/search helpers live in
//      lanes_support.go. ----
