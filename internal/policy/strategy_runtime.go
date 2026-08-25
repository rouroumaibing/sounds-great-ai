// Package policy implements the runtime side of session strategy and context
// capacity governance (roadmap P0-3).
//
// The settings layer already persists strategy *definitions*; this package
// resolves them at call time into a concrete decision, records an immutable
// snapshot for audit, and drives the seal state machine. Capacity governance
// produces a snapshot, supports pinning a session, evaluates a seal gate, and
// emits a usage-warning event.
//
// Safety invariant: every decision path is fail-closed. An empty chain, a
// missing resolver, or a resolver error all resolve to DENY — policies never
// implicitly allow.
package policy

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// StrategyRef identifies a strategy entry in a chain by id and priority rank.
// A lower PriorityRank means higher precedence (it is consulted first).
type StrategyRef struct {
	ID           string
	PriorityRank int
	Enabled      bool
}

// StrategyContext is the input a resolver receives when evaluating a ref.
type StrategyContext struct {
	ThreadID  string
	UserID    string
	Operation string
	// Inputs carries arbitrary facts the resolver may consult (e.g. token
	// budget, current trust boundary, time of day).
	Inputs map[string]any
}

// StrategyDecision is the output a strategy produces when it applies.
type StrategyDecision struct {
	StrategyID string
	Allow      bool
	Reason     string
	// Overrides carries policy knobs the winning strategy sets (e.g.
	// maxTokens, silenceWindow). May be nil.
	Overrides map[string]any
}

// StrategyResolver resolves a single strategy ref to a decision. A nil result
// means "this strategy abstains" and the chain continues to the next ref.
type StrategyResolver interface {
	Resolve(ref StrategyRef, ctx StrategyContext) (*StrategyDecision, error)
}

// StrategyChain is an ordered set of strategy refs, sorted by precedence.
type StrategyChain struct {
	refsByRank []StrategyRef
}

// NewStrategyChain builds a chain and sorts refs by priority rank ascending
// (stable, so equal ranks keep declaration order).
func NewStrategyChain(refs ...StrategyRef) *StrategyChain {
	c := &StrategyChain{}
	c.refsByRank = append(c.refsByRank, refs...)
	sort.SliceStable(c.refsByRank, func(i, j int) bool {
		return c.refsByRank[i].PriorityRank < c.refsByRank[j].PriorityRank
	})
	return c
}

// Refs returns the chain refs in precedence order (read-only copy).
func (c *StrategyChain) Refs() []StrategyRef {
	out := make([]StrategyRef, len(c.refsByRank))
	copy(out, c.refsByRank)
	return out
}

// Resolve walks the chain in precedence order and returns the first decision a
// resolver produces. Fail-closed semantics:
//   - nil resolver                 -> Deny
//   - ref disabled                 -> skipped
//   - resolver error               -> Deny (never bypass)
//   - no ref produces a decision   -> Deny
func (c *StrategyChain) Resolve(resolver StrategyResolver, ctx StrategyContext) StrategyDecision {
	if resolver == nil {
		return StrategyDecision{Allow: false, Reason: "policy: no resolver configured (fail-closed)"}
	}
	for _, ref := range c.refsByRank {
		if !ref.Enabled {
			continue
		}
		d, err := resolver.Resolve(ref, ctx)
		if err != nil {
			return StrategyDecision{
				Allow:  false,
				Reason: fmt.Sprintf("policy: resolver %q error -> deny: %v", ref.ID, err),
			}
		}
		if d != nil {
			return *d
		}
	}
	return StrategyDecision{Allow: false, Reason: "policy: no strategy resolved -> deny (fail-closed)"}
}

// --- seal state machine ---------------------------------------------------

// SealState is the lifecycle of a session seal.
type SealState string

const (
	// SealOpen is the normal operating state; new context may be admitted.
	SealOpen SealState = "open"
	// SealSealing is a transient state while a seal is being applied.
	SealSealing SealState = "sealing"
	// SealSealed blocks admission until released.
	SealSealed SealState = "sealed"
	// SealReleased is the post-seal state before reopening.
	SealReleased SealState = "released"
)

var sealTransitions = map[SealState][]SealState{
	SealOpen:     {SealSealing},
	SealSealing:  {SealSealed, SealOpen},
	SealSealed:   {SealReleased},
	SealReleased: {SealOpen},
}

// CanTransition reports whether a move from->to is permitted.
func (s SealState) CanTransition(to SealState) bool {
	for _, next := range sealTransitions[s] {
		if next == to {
			return true
		}
	}
	return false
}

// ErrInvalidSealTransition is returned when a seal move is not permitted.
var ErrInvalidSealTransition = errors.New("policy: invalid seal state transition")

// TransitionSeal validates and returns the target state. It returns
// ErrInvalidSealTransition on an illegal move (the source is left untouched by
// the caller since this is a value function).
func TransitionSeal(from, to SealState) (SealState, error) {
	if from == to {
		return from, nil
	}
	if !from.CanTransition(to) {
		return from, fmt.Errorf("%w: %s -> %s", ErrInvalidSealTransition, from, to)
	}
	return to, nil
}

// StrategySnapshot is an immutable audit record of a single call's resolution.
type StrategySnapshot struct {
	ChainID  string
	ThreadID string
	UserID   string
	Decision StrategyDecision
	TakenAt  time.Time
	TraceID  string
}

// Snapshot records a decision against a chain for audit/logging.
func Snapshot(chainID, threadID, userID, traceID string, d StrategyDecision) StrategySnapshot {
	return StrategySnapshot{
		ChainID:  chainID,
		ThreadID: threadID,
		UserID:   userID,
		Decision: d,
		TakenAt:  time.Now(),
		TraceID:  traceID,
	}
}
