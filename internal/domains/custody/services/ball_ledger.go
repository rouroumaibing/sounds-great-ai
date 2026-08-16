package services

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
)

// errPostCommit is returned by the guard when a disposition passed all three
// assertions but the post-commit re-read shows the ball did NOT resolve as
// expected (a concurrency anomaly). The audit trail records it as
// ball.disposition_rejected (payload "postcommit").
var errPostCommit = errors.New("custody: post-commit invariant mismatch")

// BallLedger records ball custody events into an append-only store and projects
// the current custody state. It is the SG equivalent of the
// ball-custody domain (event ledger + pure-function state machine).
//
// The ledger is a passive observer during P0: the orchestration writes events
// but never branches on the projected state ("只写不读"). Later phases (P2/P3/P4)
// read Snapshot for hold_ball, disposition closure, and brief/trail APIs.
type BallLedger struct {
	store custodyPorts.IBallLedgerStore
	// mu guards the read-judge-write sequence of the conditional Try* methods
	// so two goroutines disposing the same thread cannot both pass the guard.
	mu sync.Mutex
}

// NewBallLedger creates a BallLedger over the given store.
func NewBallLedger(store custodyPorts.IBallLedgerStore) *BallLedger {
	return &BallLedger{store: store}
}

func (l *BallLedger) append(ctx context.Context, ev custodyPorts.BallEvent) error {
	if ev.Timestamp == 0 {
		ev.Timestamp = time.Now().Unix()
	}
	return l.store.Append(ctx, ev)
}

// nextGeneration computes the next handoff generation from the existing ledger.
// This reads the ledger's OWN store (not the orchestration reading it), which is
// consistent with "只写不读" for the orchestration control flow.
func (l *BallLedger) nextGeneration(ctx context.Context, threadID string) int {
	events, err := l.store.GetEvents(ctx, threadID)
	if err != nil {
		return 1
	}
	gen := 1
	for _, e := range events {
		if e.Type == custodyPorts.BallHanded || e.Type == custodyPorts.DispatchDispositioned {
			gen++
		}
	}
	return gen
}

func (l *BallLedger) RecordHanded(ctx context.Context, threadID, from, to string) error {
	return l.append(ctx, custodyPorts.BallEvent{
		ThreadID:  threadID,
		Type:      custodyPorts.BallHanded,
		From:      from,
		To:        to,
		Holder:    to,
		Generation: l.nextGeneration(ctx, threadID),
	})
}

func (l *BallLedger) RecordInvocationStarted(ctx context.Context, threadID, holder string) error {
	return l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.InvocationStarted,
		Holder:   holder,
	})
}

func (l *BallLedger) RecordInvocationHeartbeat(ctx context.Context, threadID, holder string) error {
	return l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.InvocationHeartbeat,
		Holder:   holder,
	})
}

func (l *BallLedger) RecordInvocationDied(ctx context.Context, threadID, holder string) error {
	return l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.InvocationDied,
		Holder:   holder,
	})
}

func (l *BallLedger) RecordDispatchDispositioned(ctx context.Context, threadID, from, to string) error {
	// A dispatch disposition hands the ball to `to`; bump generation here too.
	_ = l.append(ctx, custodyPorts.BallEvent{
		ThreadID:  threadID,
		Type:      custodyPorts.DispatchDispositioned,
		From:      from,
		To:        to,
		Holder:    to,
		Generation: l.nextGeneration(ctx, threadID),
	})
	return nil
}

func (l *BallLedger) RecordHeld(ctx context.Context, threadID, holder string) error {
	return l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.BallHeld,
		Holder:   holder,
	})
}

func (l *BallLedger) RecordWakeConditionMet(ctx context.Context, threadID, holder string) error {
	return l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.WakeConditionMet,
		Holder:   holder,
	})
}

func (l *BallLedger) RecordHoldDispositioned(ctx context.Context, threadID, holder string) error {
	return l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.HoldDispositioned,
		Holder:   holder,
	})
}

func (l *BallLedger) RecordTaskDone(ctx context.Context, threadID, holder string) error {
	return l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.TaskDone,
		Holder:   holder,
	})
}

func (l *BallLedger) RecordTaskBlocked(ctx context.Context, threadID, holder string) error {
	return l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.TaskBlocked,
		Holder:   holder,
	})
}

// Record appends an arbitrary custody event (G4 add-only lifecycle events).
// The store assigns the Seq; callers set the fields they need.
func (l *BallLedger) Record(ctx context.Context, ev custodyPorts.BallEvent) error {
	return l.append(ctx, ev)
}

// tryCommitDispatch attempts to dispose the ball from `from` to `to` only if
// `from` is still the current projected holder. On success it appends
// dispatch_dispositioned; on failure it appends ball.disposition_rejected
// (audit-only) and returns ok=false.
func (l *BallLedger) tryCommitDispatch(ctx context.Context, threadID, from, to string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	events, err := l.store.GetEvents(ctx, threadID)
	if err != nil {
		return false, err
	}
	snap := Project(events)
	// G13: three-piece invariant guard (assertCurrentHolder /
	// assertLatestInvocation / assertExactHandoffIsLive). Any failure appends a
	// ball.disposition_rejected audit event (state unchanged) and returns false.
	if err := assertCurrentHolder(snap, from); err != nil {
		l.appendRejected(threadID, from, to, snap.Holder, snap.Generation, guardReason(err))
		return false, nil
	}
	if err := assertLatestInvocation(events, from); err != nil {
		l.appendRejected(threadID, from, to, snap.Holder, snap.Generation, guardReason(err))
		return false, nil
	}
	if err := assertExactHandoffIsLive(events, from, to); err != nil {
		l.appendRejected(threadID, from, to, snap.Holder, snap.Generation, guardReason(err))
		return false, nil
	}
	if err := l.append(ctx, custodyPorts.BallEvent{
		ThreadID:  threadID,
		Type:      custodyPorts.DispatchDispositioned,
		From:      from,
		To:        to,
		Holder:    to,
		Generation: nextGenerationFrom(events),
	}); err != nil {
		return false, err
	}
	// G13 post-commit verify: re-read and confirm the ball resolved to `to`. A
	// mismatch indicates a concurrency anomaly; record it and fail loudly.
	if snap2, perr := l.Snapshot(ctx, threadID); perr == nil && snap2.Holder != to {
		l.appendRejected(threadID, from, to, snap2.Holder, snap2.Generation, "postcommit")
		return false, errPostCommit
	}
	return true, nil
}

// TryDispatchDispositioned is the guarded variant of RecordDispatchDispositioned
// (G1+G13): it converges the ball from `from` to `to` only when `from` remains
// the current holder AND passes the three-piece invariant guard. A stale/
// duplicate/ superseded handoff callback therefore cannot wrongly resolve a
// ball that has since moved on.
func (l *BallLedger) TryDispatchDispositioned(ctx context.Context, threadID, from, to string) (bool, error) {
	return l.tryCommitDispatch(ctx, threadID, from, to)
}

// tryCommitHold attempts to dispose a parked hold only if the ball is still
// parked and held by `holder` AND passes the invariant guard. On success it
// appends wake_condition_met + hold_dispositioned; on failure it appends
// ball.disposition_rejected and returns ok=false (e.g. the hold was superseded
// by a newer handoff).
func (l *BallLedger) tryCommitHold(ctx context.Context, threadID, holder string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	events, err := l.store.GetEvents(ctx, threadID)
	if err != nil {
		return false, err
	}
	snap := Project(events)
	if snap.State != custodyPorts.BallStateParked || snap.Holder != holder {
		l.appendRejected(threadID, "", "", holder, snap.Generation, "hold")
		return false, nil
	}
	if err := assertLatestInvocation(events, holder); err != nil {
		l.appendRejected(threadID, "", "", holder, snap.Generation, guardReason(err))
		return false, nil
	}
	if err := assertExactHandoffIsLive(events, holder, ""); err != nil {
		l.appendRejected(threadID, "", "", holder, snap.Generation, guardReason(err))
		return false, nil
	}
	if err := l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.WakeConditionMet,
		Holder:   holder,
	}); err != nil {
		return false, err
	}
	if err := l.append(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.HoldDispositioned,
		Holder:   holder,
	}); err != nil {
		return false, err
	}
	// G13 post-commit verify: the ball must now be active (dispositioned).
	if snap2, perr := l.Snapshot(ctx, threadID); perr == nil && snap2.State != custodyPorts.BallStateActive {
		l.appendRejected(threadID, "", "", holder, snap2.Generation, "postcommit")
		return false, errPostCommit
	}
	return true, nil
}

// TryHoldDispositioned is the guarded variant of the wake+dispose path (G1+G13).
func (l *BallLedger) TryHoldDispositioned(ctx context.Context, threadID, holder string) (bool, error) {
	return l.tryCommitHold(ctx, threadID, holder)
}

// nextGenerationFrom computes the next handoff generation from an event slice
// (without re-reading the store, unlike nextGeneration).
func nextGenerationFrom(events []custodyPorts.BallEvent) int {
	gen := 1
	for _, e := range events {
		if e.Type == custodyPorts.BallHanded || e.Type == custodyPorts.DispatchDispositioned {
			gen++
		}
	}
	return gen
}

// appendRejected writes a ball.disposition_rejected audit event (state
// unchanged). It is called by the invariant guard when a disposition is refused.
func (l *BallLedger) appendRejected(threadID, from, to, holder string, gen int, reason string) {
	_ = l.append(context.Background(), custodyPorts.BallEvent{
		ThreadID:  threadID,
		Type:      custodyPorts.BallDispositionRejected,
		From:      from,
		To:        to,
		Holder:    holder,
		Generation: gen,
		Payload:   reason,
	})
}

// Snapshot projects the current custody state for a thread from its event log.
func (l *BallLedger) Snapshot(ctx context.Context, threadID string) (custodyPorts.BallSnapshot, error) {
	events, err := l.store.GetEvents(ctx, threadID)
	if err != nil {
		return custodyPorts.BallSnapshot{}, err
	}
	return Project(events), nil
}

// ProjectDutyBriefing folds every thread's custody state into an operations
// view (G6). It performs only reads: ListThreadIDs → Snapshot per thread →
// classify by state. stalledBlocked threads are sorted by UpdatedAt descending
// so the most-neglected surface first.
func (l *BallLedger) ProjectDutyBriefing(ctx context.Context) (custodyPorts.DutyBriefing, error) {
	ids, err := l.store.ListThreadIDs(ctx)
	if err != nil {
		return custodyPorts.DutyBriefing{}, err
	}
	brief := custodyPorts.DutyBriefing{
		GeneratedAt: time.Now().Unix(),
		Counts:      make(map[string]int),
	}
	for _, tid := range ids {
		snap, err := l.Snapshot(ctx, tid)
		if err != nil || snap.State == custodyPorts.BallStateNew {
			continue
		}
		entry := custodyPorts.BriefingSummary{
			ThreadID: tid,
			Holder:   snap.Holder,
			State:    string(snap.State),
			UpdatedAt: snap.UpdatedAt,
		}
		switch snap.State {
		case custodyPorts.BallStateParked:
			brief.NeedsUser = append(brief.NeedsUser, entry)
		case custodyPorts.BallStateZombie, custodyPorts.BallStateDead:
			brief.DeadBalls = append(brief.DeadBalls, entry)
		case custodyPorts.BallStateVoid:
			brief.VoidPasses = append(brief.VoidPasses, entry)
		case custodyPorts.BallStateBlocked:
			brief.StaleBlocked = append(brief.StaleBlocked, entry)
		}
		brief.Counts[string(snap.State)]++
	}
	sort.SliceStable(brief.StaleBlocked, func(i, j int) bool {
		return brief.StaleBlocked[i].UpdatedAt > brief.StaleBlocked[j].UpdatedAt
	})
	return brief, nil
}

// ReconcileZombies closes "in-flight" invocations that have stopped advancing.
// An invocation is in-flight after InvocationStarted and before a terminal event
// (InvocationDied / TaskDone / TaskBlocked). If the last heartbeat (or start time,
// when no heartbeat exists) is older than timeout, we append InvocationDied so the
// projected state settles into dead/zombie instead of hanging in active forever.
//
// This is the reconcileZombies sweep: when a spawned CLI agent process
// crashes or a fiber leaks without closing its event channel, the orchestration
// goroutine may exit without writing a terminal event. The reconciler sweeps the
// ledger on a timer and heals those dangling invocations. It is safe to run
// concurrently; the append-only store serializes writes per thread id.
func (l *BallLedger) ReconcileZombies(ctx context.Context, timeout time.Duration) (int, error) {
	ids, err := l.store.ListThreadIDs(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	threshold := int64(timeout.Seconds())
	reconciled := 0
	for _, tid := range ids {
		events, err := l.store.GetEvents(ctx, tid)
		if err != nil || len(events) == 0 {
			continue
		}
		inFlight := false
		holder := ""
		lastActivity := int64(0)
		lastEventType := custodyPorts.BallEventType("")
		for _, e := range events {
			lastEventType = e.Type
			switch e.Type {
			case custodyPorts.InvocationStarted:
				inFlight = true
				holder = e.Holder
				if e.Timestamp > lastActivity {
					lastActivity = e.Timestamp
				}
			case custodyPorts.InvocationHeartbeat:
				if inFlight && e.Timestamp > lastActivity {
					lastActivity = e.Timestamp
				}
			case custodyPorts.InvocationDied, custodyPorts.TaskDone, custodyPorts.TaskBlocked:
				if inFlight {
					inFlight = false
					holder = ""
				}
			}
		}
		if inFlight {
			idle := now - lastActivity
			switch {
			case idle > threshold:
				// Hard timeout: heal the dangling invocation into died/zombie.
				if err := l.append(ctx, custodyPorts.BallEvent{
					ThreadID: tid, Type: custodyPorts.InvocationDied, Holder: holder,
				}); err == nil {
					reconciled++
				}
			case idle > int64(degradeThreshold.Seconds()):
				// G4: heartbeat late but within grace → flagged degraded (audit).
				if lastEventType != custodyPorts.BallDegraded {
					_ = l.append(ctx, custodyPorts.BallEvent{
						ThreadID: tid, Type: custodyPorts.BallDegraded, Holder: holder,
					})
				}
			}
			continue
		}
		// Not in-flight: flag long-stale blocked threads as idle_long (G4).
		snap := Project(events)
		if snap.State == custodyPorts.BallStateBlocked && now-snap.UpdatedAt > int64(longIdleThreshold.Seconds()) {
			if lastEventType != custodyPorts.TaskIdleLong {
				_ = l.append(ctx, custodyPorts.BallEvent{
					ThreadID: tid, Type: custodyPorts.TaskIdleLong, Holder: snap.Holder,
				})
			}
		}
	}
	return reconciled, nil
}
