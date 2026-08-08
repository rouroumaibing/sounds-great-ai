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
)

// LaneEntry is a single record in a memory lane.
type LaneEntry struct {
	ID        string          `json:"id"`
	Type      LaneType        `json:"type"`
	Content   string          `json:"content"`
	Source    string          `json:"source"`    // origin: session id, breed, external
	Timestamp int64           `json:"timestamp"` // unix milli
	Status    LaneEntryStatus `json:"status"`
}

// Lane is a typed memory lane with canonical truth ownership.
// Only approved entries are considered truth.
type Lane struct {
	mu      sync.RWMutex
	laneType LaneType
	entries map[string]*LaneEntry // id -> entry
	order   []string             // insertion order for stable iteration
}

// NewLane creates a new typed memory lane.
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
	defer l.mu.Unlock()
	l.entries[e.ID] = e
	l.order = append(l.order, e.ID)
	return e
}

// Approve promotes a pending entry to approved (canonical truth).
// Any previously approved entry with overlapping content is retired.
func (l *Lane) Approve(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[id]
	if !ok || e.Status != StatusPending {
		return false
	}
	e.Status = StatusApproved
	return true
}

// Reject removes a pending candidate. Returns true if rejected.
func (l *Lane) Reject(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[id]
	if !ok || e.Status != StatusPending {
		return false
	}
	// Rejected candidates are forgotten (not truth, not kept as retired).
	e.Status = StatusForgotten
	return true
}

// Retire marks an approved entry as retired (superseded, kept for audit).
func (l *Lane) Retire(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[id]
	if !ok || e.Status != StatusApproved {
		return false
	}
	e.Status = StatusRetired
	return true
}

// Forget marks an entry as forgotten (excluded from recall).
func (l *Lane) Forget(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[id]
	if !ok {
		return false
	}
	e.Status = StatusForgotten
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

// LaneRegistry holds all typed memory lanes.
type LaneRegistry struct {
	mu    sync.RWMutex
	lanes map[LaneType]*Lane
}

// NewLaneRegistry creates a registry with all standard lanes initialized.
func NewLaneRegistry() *LaneRegistry {
	r := &LaneRegistry{lanes: make(map[LaneType]*Lane)}
	for _, t := range []LaneType{LaneTaste, LaneProfile, LaneEntity, LanePerson, LaneEvent, LaneDecision, LaneLesson} {
		r.lanes[t] = NewLane(t)
	}
	return r
}

// Lane returns the lane for a given type, or nil if unknown.
func (r *LaneRegistry) Lane(t LaneType) *Lane {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lanes[t]
}

// LaneTypes returns all registered lane types.
func (r *LaneRegistry) LaneTypes() []LaneType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]LaneType, 0, len(r.lanes))
	for t := range r.lanes {
		result = append(result, t)
	}
	return result
}
