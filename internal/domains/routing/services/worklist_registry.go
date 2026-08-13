package services

import (
	"sync"

	"sounds-great-ai/internal/domains/routing/ports"
)

// worklistState is the per-invocation worklist (G2). It is guarded by mu so
// concurrent A2A handoffs within a fan-out share one consistent streak/depth.
type worklistState struct {
	mu          sync.Mutex
	depth       int
	maxDepth    int
	streakFrom  string
	streakTo    string
	streakCount int
}

// WorklistRegistry enforces chain depth and ping-pong breaking for A2A handoffs
// (G2). It mirrors clowder-ai's WorklistRegistry.pushToWorklist: a handoff is
// rejected when it would exceed the per-message depth ceiling, or when the same
// unordered pair (A↔B) has handshaked too many times in a row without doing
// substantive work.
type WorklistRegistry struct {
	mu     sync.Mutex
	states map[string]*worklistState
}

// NewWorklistRegistry creates an empty registry.
func NewWorklistRegistry() *WorklistRegistry {
	return &WorklistRegistry{states: make(map[string]*worklistState)}
}

// Register creates the worklist for invID (idempotent). A second call on the
// same invID is a no-op so recursive handoffs never reset the depth budget.
func (r *WorklistRegistry) Register(invID string, maxDepth int) {
	if maxDepth <= 0 {
		maxDepth = ports.PingPongBlockThreshold
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.states[invID]; ok {
		return
	}
	r.states[invID] = &worklistState{maxDepth: maxDepth}
}

func (r *WorklistRegistry) getOrCreate(invID string) *worklistState {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.states[invID]
	if !ok {
		st = &worklistState{maxDepth: ports.PingPongBlockThreshold}
		r.states[invID] = st
	}
	return st
}

// unorderedPair returns a canonical key for an unordered breed pair (A↔B).
func unorderedPair(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

// Push consults the depth and ping-pong guards for a handoff from→to.
// accepted=false means the handoff must NOT proceed; reason is "depth" or
// "pingpong". warn=true means proceed but warn the next dog.
func (r *WorklistRegistry) Push(invID, from, to string, activity ports.SubstantiveActivity) (bool, string, bool) {
	st := r.getOrCreate(invID)
	st.mu.Lock()
	defer st.mu.Unlock()

	// 1) Depth ceiling (per message).
	if st.depth >= st.maxDepth {
		return false, "depth", false
	}

	// 2) Ping-pong streak (consecutive same-pair, substantive-work exempt).
	pair := unorderedPair(from, to)
	nc := 1
	if activity.HadToolCall || activity.OutputLen > ports.SubstantiveOutputLenT {
		// Real work: inertia broken; this push counts as 1.
		nc = 1
	} else if pair == unorderedPair(st.streakFrom, st.streakTo) && st.streakCount > 0 {
		nc = st.streakCount + 1
	} else {
		nc = 1
	}

	if nc >= ports.PingPongBlockThreshold {
		// Blocked: the chain terminates. Do NOT mutate streak/depth.
		return false, "pingpong", false
	}

	// 3) Accept: record streak + advance depth.
	st.streakFrom = from
	st.streakTo = to
	st.streakCount = nc
	st.depth++
	warn := nc >= ports.PingPongWarnThreshold
	return true, "", warn
}

// Done releases the worklist state for invID.
func (r *WorklistRegistry) Done(invID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.states, invID)
}

// Ensure WorklistRegistry satisfies the port at compile time.
var _ ports.IWorklist = (*WorklistRegistry)(nil)
