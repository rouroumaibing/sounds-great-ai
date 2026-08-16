package services

import (
	"sync"

	"sounds-great-ai/internal/domains/routing/ports"
)

// maxWorklistFanout bounds how many distinct targets a single invocation's
// worklist may grow to via runtime expansion (G11). Keeps the dynamic
// pushToWorklist fan-out from running away unbounded.
const maxWorklistFanout = 16

// worklistState is the per-invocation worklist (G2). It is guarded by mu so
// concurrent A2A handoffs within a fan-out share one consistent streak/depth.
// G11 adds the mutable target list (entry.list), the a2aFrom source map, and
// the executedIndex dedup map so a breed can pull in NEW collaborators mid-chain
// (pushToWorklist), not just guard a pre-planned chain.
type worklistState struct {
	mu           sync.Mutex
	depth        int
	maxDepth     int
	streakFrom   string
	streakTo     string
	streakCount  int
	list         []string             // known targets (dynamic fan-out set)
	a2aFrom      map[string]string    // target -> breed that added it
	executedIndex map[string]int      // target -> times it has been executed
}

// WorklistRegistry enforces chain depth and ping-pong breaking for A2A handoffs
// (G2). It mirrors the WorklistRegistry.pushToWorklist pattern: a handoff is
// rejected when it would exceed the per-message depth ceiling, or when the same
// unordered pair (A↔B) has handshaked too many times in a row without doing
// substantive work. G11 extends it with a mutable target list so a running
// worklist can be expanded at runtime (a breed can @-summon a new collaborator),
// recording source attribution and deduping re-entry.
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
	r.states[invID] = newWorklistState(maxDepth)
}

func (r *WorklistRegistry) getOrCreate(invID string) *worklistState {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.states[invID]
	if !ok {
		st = newWorklistState(ports.PingPongBlockThreshold)
		r.states[invID] = st
	}
	return st
}

func newWorklistState(maxDepth int) *worklistState {
	return &worklistState{
		maxDepth:      maxDepth,
		list:         make([]string, 0),
		a2aFrom:      make(map[string]string),
		executedIndex: make(map[string]int),
	}
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
// "pingpong". warn=true means proceed but warn the next dog. On accept it
// records `to` into the dynamic target list (G11) for later fan-out dedup.
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
	// G11: track the accepted target in the dynamic list (idempotent).
	if !contains(st.list, to) {
		st.list = append(st.list, to)
		if _, ok := st.a2aFrom[to]; !ok {
			st.a2aFrom[to] = from
		}
	}
	if _, ok := st.executedIndex[to]; !ok {
		st.executedIndex[to] = 0
	}
	st.executedIndex[to]++
	warn := nc >= ports.PingPongWarnThreshold
	return true, "", warn
}

// PushToWorklist dynamically appends new targets to a running worklist (G11:
// pushToWorklist). It returns the targets actually added — those not
// already known, not yet executed, and within maxWorklistFanout. Each addition
// records fromRef for attribution (a2aFrom) so the audit trail shows who pulled
// in the new collaborator.
func (r *WorklistRegistry) PushToWorklist(invID string, targets []string, fromRef string) []string {
	st := r.getOrCreate(invID)
	st.mu.Lock()
	defer st.mu.Unlock()
	added := make([]string, 0, len(targets))
	for _, t := range targets {
		if contains(st.list, t) {
			continue // already known (planned or previously expanded)
		}
		if st.executedIndex[t] > 0 {
			continue // already executed; do not re-add (dedup re-entry)
		}
		if len(st.list) >= maxWorklistFanout {
			break // fan-out cap reached
		}
		st.list = append(st.list, t)
		st.a2aFrom[t] = fromRef
		st.executedIndex[t] = 0
		added = append(added, t)
	}
	return added
}

// List returns the known targets of a worklist (planned + dynamically expanded).
func (r *WorklistRegistry) List(invID string) []string {
	st := r.getOrCreate(invID)
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]string, len(st.list))
	copy(out, st.list)
	return out
}

// A2AFrom returns the breed that added target to the worklist ("" if unknown).
func (r *WorklistRegistry) A2AFrom(invID, target string) string {
	st := r.getOrCreate(invID)
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.a2aFrom[target]
}

// Done releases the worklist state for invID.
func (r *WorklistRegistry) Done(invID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.states, invID)
}

// Ensure WorklistRegistry satisfies the port at compile time.
var _ ports.IWorklist = (*WorklistRegistry)(nil)

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
