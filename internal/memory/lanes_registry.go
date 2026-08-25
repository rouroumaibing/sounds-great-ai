package memory

import (
	"fmt"
	"log"
	"sync"
)

// LaneRegistry holds all typed memory lanes.
type LaneRegistry struct {
	mu        sync.RWMutex
	lanes     map[LaneType]*Lane
	persister lanePersister // nil = non-persistent (NewLaneRegistry)
	graph     *graphStore   // nil = graph store unavailable
	vector    *vectorStore  // nil = vector store unavailable (Gap3)
	cue       *cueStore     // nil = cue-event ledger unavailable (Gap4)
	events    *EventStore   // nil = event-memory index unavailable (F227)
	embedMode string        // off/shadow/on (P1 hybrid RRF gating)
}

// NewLaneRegistry creates an in-memory (non-persistent) registry with all
// standard lanes initialized. Use for tests and callers that do not need
// durability across restarts.
func NewLaneRegistry() *LaneRegistry {
	r := &LaneRegistry{lanes: make(map[LaneType]*Lane)}
	for _, t := range allLaneTypes() {
		r.lanes[t] = NewLane(t)
	}
	return r
}

// NewLaneRegistryAt creates a registry backed by a persistent store at path
// (SQLite-preferred, JSON fallback — mirrors NewMemoryStoreAt /
// NewEvidenceStoreAt). Restarting the process reloads all lanes and their
// lifecycle statuses, so typed memory survives restarts (Persistent Identity
// layer).
func NewLaneRegistryAt(path string) *LaneRegistry {
	r := &LaneRegistry{
		lanes:     make(map[LaneType]*Lane),
		persister: newDefaultLanePersister(path),
	}
	for _, t := range allLaneTypes() {
		l := NewLane(t)
		l.notify = r.flush
		r.lanes[t] = l
	}
	if r.persister != nil {
		doc, err := r.persister.load()
		if err != nil {
			log.Printf("memory: failed to load lanes from %s: %v", path, err)
		} else if doc != nil {
			for _, e := range doc.Entries {
				l := r.lanes[e.Type]
				if l == nil {
					l = NewLane(e.Type)
					l.notify = r.flush
					r.lanes[e.Type] = l
				}
				l.loadEntry(e)
			}
		}
	}
	// Relationship graph (edges + markers) lives in its own SQLite file so its
	// schema evolves independently (Gap1). Fail-open: a missing graph store
	// only disables linking/marking, never lane durability.
	if g, gerr := openGraphDB(path); gerr == nil {
		r.graph = g
	} else {
		log.Printf("memory: graph store unavailable: %v", gerr)
	}
	// Vector store for semantic recall (Gap3). Also fail-open: without it,
	// FTS5 lexical search still works and semantic search returns a clear error.
	if vs, verr := openVectorDB(path); verr == nil {
		r.vector = vs
	} else {
		log.Printf("memory: vector store unavailable: %v", verr)
	}
	// Cue-event ledger (Gap4 consumption trace). Fail-open.
	if cs, cserr := openCueDB(path); cserr == nil {
		r.cue = cs
	} else {
		log.Printf("memory: cue ledger unavailable: %v", cserr)
	}
	// Event-memory index (F227 — cognitive-state-transition first-class citizen).
	// Owner-scoped, fail-open: a missing store only disables event recall.
	if es := NewEventStore(path); es != nil {
		r.events = es
	} else {
		log.Printf("memory: event store unavailable")
	}
	// Default embed mode: on when vectors are enabled, else off.
	if r.vector != nil {
		r.embedMode = "on"
	} else {
		r.embedMode = "off"
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

// MarkEvent records a cat-declared cognitive-state transition in Event Memory
// (F227 Phase B). SG indexes the declaration verbatim — it never classifies
// which message is an "aha" (no-classifier red line). ownerUserID is required;
// an empty owner fails closed (no cross-owner event bleed).
func (r *LaneRegistry) MarkEvent(ownerUserID, cat, threadID, messageID, summary, cognitiveTransition string) (*MemoryEvent, error) {
	if r.events == nil {
		return nil, fmt.Errorf("event memory unavailable")
	}
	ev := &MemoryEvent{
		Type:                 EventTypeMarked,
		Cat:                  cat,
		ThreadID:             threadID,
		MessageID:            messageID,
		Summary:              summary,
		CognitiveTransition:  cognitiveTransition,
		Confidence:           ConfMedium,
		OwnerUserID:          ownerUserID,
	}
	if err := r.events.Record(ev); err != nil {
		return nil, err
	}
	return ev, nil
}

// RecordMagicWordEvent indexes an operator-pulled brake event (F227 Phase A,
// first lane). trigger = the matched magic word; confidence encodes the
// detection tier (magic word + @cat = high; discussing house-rules = low).
// Owner-scoped, fail-closed.
func (r *LaneRegistry) RecordMagicWordEvent(ownerUserID, cat, threadID, messageID, trigger string, conf EventConfidence) (*MemoryEvent, error) {
	if r.events == nil {
		return nil, fmt.Errorf("event memory unavailable")
	}
	ev := &MemoryEvent{
		Type:        EventTypeMagicWord,
		Trigger:     trigger,
		Cat:         cat,
		ThreadID:    threadID,
		MessageID:   messageID,
		Summary:     trigger,
		Confidence:  conf,
		OwnerUserID: ownerUserID,
	}
	if err := r.events.Record(ev); err != nil {
		return nil, err
	}
	return ev, nil
}

// EventTimeline returns the owner-scoped event timeline, newest-first,
// optionally filtered by type and minimum confidence (F227 Phase A/B panels).
func (r *LaneRegistry) EventTimeline(ownerUserID string, typ EventType, minConf EventConfidence) []*MemoryEvent {
	if r.events == nil {
		return nil
	}
	return r.events.Timeline(ownerUserID, typ, minConf)
}

// ResolveEventTeleport returns the precise (thread, message) coordinate for an
// event so the UI can jump to the exact message ("say 'I said scaffold' then
// instantly jump", F227 R1/AC-A4). Owner-scoped, fail-closed.
func (r *LaneRegistry) ResolveEventTeleport(eventID, ownerUserID string) *Teleport {
	if r.events == nil {
		return nil
	}
	return r.events.ResolveTeleport(eventID, ownerUserID)
}

// LinkEventResolution attaches a harness-change reference to an event, forming
// a resolution chain (F227 Phase C — flywheel black box). Owner-scoped.
func (r *LaneRegistry) LinkEventResolution(eventID, ownerUserID, relatedHarness string) error {
	if r.events == nil {
		return fmt.Errorf("event memory unavailable")
	}
	return r.events.LinkResolution(eventID, ownerUserID, relatedHarness)
}

// EventCount returns the owner's event count (trend signal). Must be paired
// with a resolution chain — frequency decline alone is NOT "self-evolution
// proven" (F227 KD-7).
func (r *LaneRegistry) EventCount(ownerUserID string) int {
	if r.events == nil {
		return 0
	}
	return r.events.Count(ownerUserID)
}
