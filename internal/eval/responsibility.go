package eval

import (
	"errors"
	"sync"
)

// ResponsibilityState is a node in the per-item responsibility state machine
// (F278: 爪感处置收件箱 — itemized responsibility inbox).
type ResponsibilityState string

const (
	RespOpen      ResponsibilityState = "open"
	RespClaimed   ResponsibilityState = "claimed"
	RespResolved  ResponsibilityState = "resolved"
	RespEscalated ResponsibilityState = "escalated"
)

// ErrInvalidRespTransition is returned for an illegal responsibility transition.
var ErrInvalidRespTransition = errors.New("eval: invalid responsibility transition")

var respTransitions = map[ResponsibilityState][]ResponsibilityState{
	RespOpen:      {RespClaimed, RespEscalated},
	RespClaimed:   {RespResolved, RespEscalated, RespOpen},
	RespResolved:  {},
	RespEscalated: {RespOpen, RespResolved},
}

// CanTransition reports whether from->to is a legal transition.
func CanTransition(from, to ResponsibilityState) bool {
	for _, n := range respTransitions[from] {
		if n == to {
			return true
		}
	}
	return false
}

// ResponsibilityInbox tracks per-item responsibility state (F278). Single source
// of truth; transitions fail closed on illegal moves.
type ResponsibilityInbox struct {
	mu    sync.Mutex
	state map[string]ResponsibilityState
}

// NewResponsibilityInbox creates an empty inbox.
func NewResponsibilityInbox() *ResponsibilityInbox {
	return &ResponsibilityInbox{state: make(map[string]ResponsibilityState)}
}

// Open creates a new open responsibility item (idempotent on existing id).
func (r *ResponsibilityInbox) Open(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.state[id]; !ok {
		r.state[id] = RespOpen
	}
}

// Transition moves an item to a new state. Fails closed on illegal transitions
// or unknown ids.
func (r *ResponsibilityInbox) Transition(id string, to ResponsibilityState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.state[id]
	if !ok {
		return ErrInvalidRespTransition
	}
	if !CanTransition(cur, to) {
		return ErrInvalidRespTransition
	}
	r.state[id] = to
	return nil
}

// State returns the current state of an item.
func (r *ResponsibilityInbox) State(id string) (ResponsibilityState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.state[id]
	return s, ok
}
