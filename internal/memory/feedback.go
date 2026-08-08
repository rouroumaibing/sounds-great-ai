package memory

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// DispositionAction is the human's disposition on a candidate entry.
type DispositionAction string

const (
	DispositionAccept  DispositionAction = "accept"
	DispositionReject  DispositionAction = "reject"
	DispositionModify  DispositionAction = "modify"
)

// HumanDisposition records a human's feedback on a candidate lane entry.
type HumanDisposition struct {
	ID         string            `json:"id"`
	EntryID    string            `json:"entry_id"`
	Lane       LaneType          `json:"lane"`
	Action     DispositionAction `json:"action"`
	Modified   string            `json:"modified,omitempty"` // new content if action=modify
	By         string            `json:"by"`                 // human identifier
	Timestamp  int64             `json:"timestamp"`
}

// DispositionRecorder records human dispositions and applies them to lanes.
type DispositionRecorder struct {
	mu       sync.RWMutex
	records  []*HumanDisposition
	byEntry  map[string]*HumanDisposition
}

// NewDispositionRecorder creates a new DispositionRecorder.
func NewDispositionRecorder() *DispositionRecorder {
	return &DispositionRecorder{byEntry: make(map[string]*HumanDisposition)}
}

// Record records a human disposition and applies it to the lane registry.
func (dr *DispositionRecorder) Record(reg *LaneRegistry, entryID string, lane LaneType, action DispositionAction, modified, by string) (*HumanDisposition, error) {
	l := reg.Lane(lane)
	if l == nil {
		return nil, ErrLaneNotFound
	}
	entry, ok := l.Get(entryID)
	if !ok {
		return nil, ErrEntryNotFound
	}

	d := &HumanDisposition{
		ID:        uuid.NewString(),
		EntryID:   entryID,
		Lane:      lane,
		Action:    action,
		Modified:  modified,
		By:        by,
		Timestamp: time.Now().UnixMilli(),
	}

	switch action {
	case DispositionAccept:
		l.Approve(entryID)
	case DispositionReject:
		l.Reject(entryID)
	case DispositionModify:
		// For modify: update content then approve.
		dr.mu.Lock()
		entry.Content = modified
		dr.mu.Unlock()
		l.Approve(entryID)
	}

	dr.mu.Lock()
	dr.records = append(dr.records, d)
	dr.byEntry[entryID] = d
	dr.mu.Unlock()

	return d, nil
}

// History returns all disposition records for an entry.
func (dr *DispositionRecorder) History(entryID string) []*HumanDisposition {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	var result []*HumanDisposition
	for _, r := range dr.records {
		if r.EntryID == entryID {
			result = append(result, r)
		}
	}
	return result
}

// All returns all disposition records.
func (dr *DispositionRecorder) All() []*HumanDisposition {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	result := make([]*HumanDisposition, len(dr.records))
	copy(result, dr.records)
	return result
}

// --- Consumption tracking ---

// ConsumptionAction is how a recalled memory was used.
type ConsumptionAction string

const (
	ConsumptionPresented ConsumptionAction = "presented" // shown to agent in prompt
	ConsumptionDrilled   ConsumptionAction = "drilled"   // agent explicitly drilled into it
	ConsumptionApplied   ConsumptionAction = "applied"   // agent used it in output
	ConsumptionDismissed ConsumptionAction = "dismissed" // agent ignored it
)

// ConsumptionEvent records a single recall event (content-free metadata only).
type ConsumptionEvent struct {
	ID        string            `json:"id"`
	EntryID   string            `json:"entry_id"`
	Lane      LaneType          `json:"lane"`
	Action    ConsumptionAction `json:"action"`
	SessionID string            `json:"session_id"`
	Timestamp int64             `json:"timestamp"`
}

// ConsumptionTracker tracks recall events for lane entries.
type ConsumptionTracker struct {
	mu     sync.RWMutex
	events []*ConsumptionEvent
}

// NewConsumptionTracker creates a new ConsumptionTracker.
func NewConsumptionTracker() *ConsumptionTracker {
	return &ConsumptionTracker{}
}

// Record records a consumption event.
func (ct *ConsumptionTracker) Record(entryID string, lane LaneType, action ConsumptionAction, sessionID string) *ConsumptionEvent {
	e := &ConsumptionEvent{
		ID:        uuid.NewString(),
		EntryID:   entryID,
		Lane:      lane,
		Action:    action,
		SessionID: sessionID,
		Timestamp: time.Now().UnixMilli(),
	}
	ct.mu.Lock()
	ct.events = append(ct.events, e)
	ct.mu.Unlock()
	return e
}

// ForEntry returns all consumption events for a given entry.
func (ct *ConsumptionTracker) ForEntry(entryID string) []*ConsumptionEvent {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	var result []*ConsumptionEvent
	for _, e := range ct.events {
		if e.EntryID == entryID {
			result = append(result, e)
		}
	}
	return result
}

// Count returns the total consumption events count.
func (ct *ConsumptionTracker) Count() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.events)
}

// --- Lifecycle trace (three-axis ledger) ---

// LifecycleAxis identifies one of the three lifecycle axes.
type LifecycleAxis string

const (
	AxisCreation    LifecycleAxis = "creation"    // entry was submitted
	AxisConsumption LifecycleAxis = "consumption" // entry was recalled/used
	AxisCorrection  LifecycleAxis = "correction"  // entry was modified/forgotten
)

// LifecycleRecord is a single entry in the three-axis ledger.
type LifecycleRecord struct {
	Axis      LifecycleAxis `json:"axis"`
	EntryID   string        `json:"entry_id"`
	Lane      LaneType      `json:"lane"`
	Detail    string        `json:"detail"`
	Timestamp int64         `json:"timestamp"`
}

// LifecycleTrace is the three-axis ledger: creation / consumption / correction.
type LifecycleTrace struct {
	mu      sync.RWMutex
	records []*LifecycleRecord
}

// NewLifecycleTrace creates a new LifecycleTrace.
func NewLifecycleTrace() *LifecycleTrace {
	return &LifecycleTrace{}
}

// RecordCreation records a creation event.
func (lt *LifecycleTrace) RecordCreation(entryID string, lane LaneType, detail string) {
	lt.append(AxisCreation, entryID, lane, detail)
}

// RecordConsumption records a consumption event.
func (lt *LifecycleTrace) RecordConsumption(entryID string, lane LaneType, detail string) {
	lt.append(AxisConsumption, entryID, lane, detail)
}

// RecordCorrection records a correction event.
func (lt *LifecycleTrace) RecordCorrection(entryID string, lane LaneType, detail string) {
	lt.append(AxisCorrection, entryID, lane, detail)
}

func (lt *LifecycleTrace) append(axis LifecycleAxis, entryID string, lane LaneType, detail string) {
	r := &LifecycleRecord{
		Axis:      axis,
		EntryID:   entryID,
		Lane:      lane,
		Detail:    detail,
		Timestamp: time.Now().UnixMilli(),
	}
	lt.mu.Lock()
	lt.records = append(lt.records, r)
	lt.mu.Unlock()
}

// ForEntry returns all lifecycle records for a given entry.
func (lt *LifecycleTrace) ForEntry(entryID string) []*LifecycleRecord {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	var result []*LifecycleRecord
	for _, r := range lt.records {
		if r.EntryID == entryID {
			result = append(result, r)
		}
	}
	return result
}

// All returns all lifecycle records.
func (lt *LifecycleTrace) All() []*LifecycleRecord {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	result := make([]*LifecycleRecord, len(lt.records))
	copy(result, lt.records)
	return result
}

// --- Errors ---

var (
	ErrLaneNotFound  = &memoryError{msg: "lane not found"}
	ErrEntryNotFound = &memoryError{msg: "entry not found"}
)

type memoryError struct{ msg string }

func (e *memoryError) Error() string { return e.msg }
