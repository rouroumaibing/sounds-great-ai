package settings

import "sync"

// PeopleMemoryEvent is published whenever an operator-scoped people-memory
// mutation occurs (a proposal decision, a person edit, or a deferred-receipt
// change). Other clients — e.g. other browser tabs viewing the same operator —
// subscribe over SSE and refresh on receipt. The hub is in-process only: SG
// runs as a single local process, so no cross-node fan-out is required.
type PeopleMemoryEvent struct {
	OperatorID  string `json:"operator_id"`
	CandidateID string `json:"candidate_id,omitempty"`
	PersonID    string `json:"person_id,omitempty"`
	Type        string `json:"type"` // "proposed" | "candidate_updated" | "person_updated" | "deferred_updated"
}

// PeopleMemoryEventHub is a tiny in-process pub/sub for PeopleMemoryEvent.
// Each subscriber gets a buffered channel; slow subscribers are dropped
// (non-blocking send) rather than blocking the store on a mutation.
type PeopleMemoryEventHub struct {
	mu   sync.RWMutex
	subs map[chan PeopleMemoryEvent]struct{}
}

// NewPeopleMemoryEventHub creates an empty hub.
func NewPeopleMemoryEventHub() *PeopleMemoryEventHub {
	return &PeopleMemoryEventHub{subs: make(map[chan PeopleMemoryEvent]struct{})}
}

// Subscribe registers a new subscriber and returns its event channel.
func (h *PeopleMemoryEventHub) Subscribe() chan PeopleMemoryEvent {
	ch := make(chan PeopleMemoryEvent, 16)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel. It is safe to call
// more than once; only the first call closes the channel.
func (h *PeopleMemoryEventHub) Unsubscribe(ch chan PeopleMemoryEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
}

// Publish broadcasts an event to all subscribers. The send is non-blocking per
// subscriber so a slow client can never stall a store mutation.
func (h *PeopleMemoryEventHub) Publish(ev PeopleMemoryEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- ev:
		default:
			// slow subscriber: drop to avoid blocking the store
		}
	}
}
