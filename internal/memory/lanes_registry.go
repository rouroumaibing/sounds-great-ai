package memory

import (
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
