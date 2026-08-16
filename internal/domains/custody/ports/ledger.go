package ports

import (
	"context"
	"time"
)

// BallState enumerates the ball custody lifecycle states. Mirrors the
// 8-state machine (new/active/blocked/parked/dead/void/zombie/resolved).
type BallState string

const (
	BallStateNew     BallState = "new"
	BallStateActive  BallState = "active"
	BallStateBlocked BallState = "blocked"
	BallStateParked  BallState = "parked"
	BallStateDead    BallState = "dead"
	BallStateVoid    BallState = "void"
	BallStateZombie  BallState = "zombie"
	BallStateResolved BallState = "resolved"
)

// BallEventType enumerates ball custody events (append-only ledger).
type BallEventType string

const (
	BallHanded              BallEventType = "ball.handed"
	BallHeld                BallEventType = "ball.held"
	WakeConditionMet        BallEventType = "wake_condition_met"
	HoldDispositioned       BallEventType = "hold_dispositioned"
	InvocationStarted       BallEventType = "invocation.started"
	InvocationHeartbeat     BallEventType = "invocation.heartbeat"
	InvocationDied          BallEventType = "invocation.died"
	DispatchDispositioned   BallEventType = "dispatch_dispositioned"
	TaskDone                BallEventType = "task.done"
	TaskBlocked             BallEventType = "task.blocked"

	// G1: a disposition (dispatch_dispositioned / hold_dispositioned) was
	// attempted but rejected by the guard (holder/generation mismatch or the
	// ball was superseded). Audit-only: Project() must NOT change state.
	BallDispositionRejected BallEventType = "ball.disposition_rejected"

	// G4: lifecycle events that close the gap between SG's 10-event set and
	// the 19-event set. All add-only; each has a pinned projection.
	BallHandedCVO  BallEventType = "ball.handed_cvo"  // escalated to operator/CVO
	BallVoidPass   BallEventType = "ball.void_pass"   // handoff had no valid target
	BallHoldExpired BallEventType = "ball.hold_expired" // hold reached its deadline
	BallWakeSent   BallEventType = "ball.wake_sent"   // external wake signal emitted
	BallFrozen     BallEventType = "ball.frozen"      // coordination frozen
	BallDegraded   BallEventType = "ball.degraded"    // heartbeat late but within grace
	BallAbandoned  BallEventType = "ball.abandoned"   // task abandoned
	TaskUnblocked  BallEventType = "task.unblocked"   // a blocked ball was unblocked
	TaskIdleLong   BallEventType = "task.idle_long"   // parked/idle too long
)

// WakeKind enumerates how a parked hold may be awakened (D3 scope: manual +
// webhook only; wakeWhen(command) is deferred). Mirrors the
// hold_ball wake-condition kinds.
type WakeKind string

const (
	// WakeManual is awakened by a human action (button click / WS WAKE_HOLD).
	WakeManual WakeKind = "manual"
	// WakeWebhook is awakened by an external POST to the custody webhook endpoint.
	WakeWebhook WakeKind = "webhook"
	// WakeCommand is awakened when a shell command (run by the ProcessManager)
	// completes. Added in G5 (D3 deferred scope: wakeWhen(command)).
	WakeCommand WakeKind = "command"
)

// WakeCondition describes the condition that releases a parked hold.
// For WakeWebhook, Token is the shared secret validated on the webhook call.
type WakeCondition struct {
	Kind  WakeKind `json:"kind"`
	Token string   `json:"token,omitempty"`
	Note  string   `json:"note,omitempty"`
	// G5: timed/command wake extensions emitted by a dog inside ```hold_ball.
	// FireAfterMs wakes the hold automatically after N milliseconds
	// (wakeAfterMs). Command is a shell command run to completion whose success
	// wakes the hold (wakeWhen(command)).
	FireAfterMs int64  `json:"fire_after_ms,omitempty"`
	Command     string `json:"command,omitempty"`
	// G15: WaitSourceRef is the structured grounding for a timed (FireAfterMs)
	// hold — it declares WHAT the hold is waiting for (a source/event ref) before
	// a time-based wake is permitted. Mirrors the waitSourceRef rule: a
	// wakeAfterMs hold without waitSourceRef is rejected (the hold must state
	// what it is waiting on). Command wakes are self-grounded and exempt.
	WaitSourceRef string `json:"wait_source_ref,omitempty"`
}

// HoldRecord is an active park awaiting wake. It lives in the HoldScheduler's
// in-memory map; the secret Token is intentionally NOT written to the append-only
// ledger (only the lifecycle events ball.held → wake_condition_met →
// hold_dispositioned are).
type HoldRecord struct {
	ThreadID      string       `json:"thread_id"`
	Holder        string       `json:"holder"`
	Condition     WakeCondition `json:"condition"`
	ResumeMessage string       `json:"resume_message"`
	CreatedAt     int64        `json:"created_at"`
	Dispositioned bool         `json:"dispositioned"`

	// G5: timed/command wake extensions.
	// FireAt is the absolute unix time at which the hold auto-wakes (主动唤醒点).
	// Deadline is the absolute unix time after which, if still parked, the hold
	// expires (heldUntil) and is written off as ball.hold_expired → dead.
	// Command is the shell command run (WakeCommand) whose completion wakes the hold.
	FireAt   int64  `json:"fire_at,omitempty"`
	Deadline int64  `json:"deadline,omitempty"`
	Command  string `json:"command,omitempty"`
}

// BallEvent is a single append-only custody event.
type BallEvent struct {
	Seq       int64        `json:"seq"`
	ThreadID  string       `json:"thread_id"`
	Type      BallEventType `json:"type"`
	Holder    string       `json:"holder"`
	From      string       `json:"from,omitempty"`
	To        string       `json:"to,omitempty"`
	Generation int         `json:"generation"`
	Payload   string       `json:"payload,omitempty"`
	Timestamp int64        `json:"timestamp"`
}

// BallSnapshot is the projected current state for a thread.
type BallSnapshot struct {
	ThreadID   string    `json:"thread_id"`
	State      BallState `json:"state"`
	Holder     string    `json:"holder"`
	Generation int       `json:"generation"`
	UpdatedAt  int64     `json:"updated_at"`
	// Degraded is set when a heartbeat arrived late (beyond the interval) but
	// still within the zombie grace window — the ball is alive but sluggish
	// (G4: ball.degraded). It never changes the 8-state value.
	Degraded bool `json:"degraded,omitempty"`
}

// IBallLedgerStore persists append-only ball events.
type IBallLedgerStore interface {
	Append(ctx context.Context, ev BallEvent) error
	GetEvents(ctx context.Context, threadID string) ([]BallEvent, error)
	// ListThreadIDs returns every thread that has at least one ledger event.
	// Used by the zombie reconciler (P1) to scan in-flight invocations.
	ListThreadIDs(ctx context.Context) ([]string, error)
}

// IBallLedger records ball custody events and projects current state.
// This is the orchestration "ball custody ledger" (the ball-custody domain).
type IBallLedger interface {
	RecordHanded(ctx context.Context, threadID, from, to string) error
	RecordInvocationStarted(ctx context.Context, threadID, holder string) error
	RecordInvocationHeartbeat(ctx context.Context, threadID, holder string) error
	RecordInvocationDied(ctx context.Context, threadID, holder string) error
	RecordDispatchDispositioned(ctx context.Context, threadID, from, to string) error
	RecordHeld(ctx context.Context, threadID, holder string) error
	RecordWakeConditionMet(ctx context.Context, threadID, holder string) error
	RecordHoldDispositioned(ctx context.Context, threadID, holder string) error
	RecordTaskDone(ctx context.Context, threadID, holder string) error
	RecordTaskBlocked(ctx context.Context, threadID, holder string) error
	// G1: conditional disposition guards. They read the projected state and only
	// append the disposition event when the caller is authorized to converge the
	// ball (current holder matches, or the ball is parked & held by the waker).
	// Returns ok=false (and appends a ball.disposition_rejected audit event)
	// when the guard fails, so a stale/duplicate callback can never wrongly
	// converge a live ball.
	TryDispatchDispositioned(ctx context.Context, threadID, from, to string) (bool, error)
	TryHoldDispositioned(ctx context.Context, threadID, holder string) (bool, error)
	// G4: generic append for the add-only lifecycle events. Callers set the
	// fields they need (Holder/From/To/Generation/Payload); the store assigns Seq.
	Record(ctx context.Context, ev BallEvent) error
	Snapshot(ctx context.Context, threadID string) (BallSnapshot, error)
	// ProjectTrail folds the ledger events for threadID into a Briefing (the
	// read model behind the Brief & Trail API). It performs only reads.
	ProjectTrail(ctx context.Context, threadID string) (Briefing, error)
	// G6: aggregate projection across every thread into a duty briefing
	// (needsUser / deadBalls / voidPasses / staleBlocked). Pure read; no writes.
	ProjectDutyBriefing(ctx context.Context) (DutyBriefing, error)
	// ReconcileZombies closes dangling in-flight invocations that have stopped
	// heartbeating (see services.BallLedger.ReconcileZombies).
	ReconcileZombies(ctx context.Context, timeout time.Duration) (int, error)
}

// IHoldScheduler manages active parked holds and releases them when woken
// (manual click / webhook POST). It writes the lifecycle events
// ball.held → wake_condition_met → hold_dispositioned into the ledger and
// tracks the in-memory HoldRecord (including the secret webhook token) so a
// resumed thread can be re-dispatched to its holder.
type IHoldScheduler interface {
	// Hold parks a thread: writes ball.held and registers an active hold.
	Hold(ctx context.Context, threadID, holder string, cond WakeCondition, resumeMsg string) error
	// Wake releases a parked hold after validating the wake kind/token, writing
	// wake_condition_met + hold_dispositioned and returning the record so the
	// caller can resume the holder. Returns an error if there is no active hold
	// or the token does not match.
	Wake(ctx context.Context, threadID string, kind WakeKind, token string) (*HoldRecord, error)
	// GetHold returns the active hold for a thread, if any.
	GetHold(ctx context.Context, threadID string) (*HoldRecord, bool)
	// ActiveHolds returns a snapshot of all currently parked holds.
	ActiveHolds(ctx context.Context) []HoldRecord
	// G5: Cancel revokes a parked hold without resuming it (user-initiated
	// cancel, distinct from Wake). Returns ErrNoActiveHold if none is active.
	Cancel(ctx context.Context, threadID string) error
	// Tick advances timed holds: auto-wakes those whose FireAt has passed
	// (writes ball.wake_sent + wake_condition_met + hold_dispositioned and
	// resumes via OnWake), and expires those past Deadline (ball.hold_expired →
	// dead). now is the current unix time. It is called by the platform sweep.
	Tick(now int64) error
	// SetOnWake registers the callback invoked when a timed/command hold
	// auto-wakes, so the transport layer can resume the holder breed.
	SetOnWake(fn func(ctx context.Context, threadID, holder, resumeMsg string))
}
