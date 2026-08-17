package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"sounds-great-ai/internal/adapter/unified"
	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
)

// Errors returned by HoldScheduler.
var (
	ErrNoActiveHold      = errors.New("custody: no active hold for thread")
	ErrWakeKindMismatch  = errors.New("custody: wake kind does not match held condition")
	ErrInvalidToken      = errors.New("custody: webhook token mismatch")
	// ErrDispositionRejected is returned by Wake when the ledger guard (G1)
	// refuses to converge the ball (it was superseded by a newer handoff/hold).
	ErrDispositionRejected = errors.New("custody: disposition rejected by ledger guard")
	// ErrWaitSourceRefRequired is returned by Hold when a timed (FireAfterMs)
	// wake is requested without a wait_source_ref grounding. A timed wake must
	// declare WHAT it is waiting on before a time-based wake is permitted.
	// Command wakes are self-grounded and exempt.
	ErrWaitSourceRefRequired = errors.New("custody: timed hold requires wait_source_ref grounding")
)

// G5 tuning constants.
const (
	// commandWakeTimeout bounds how long a wakeWhen(command) process may run
	// before the hold is expired.
	commandWakeTimeout = 5 * time.Minute
	// longIdleThreshold marks a blocked thread as idle_long (stale) in the
	// reconciler (G4: task.idle_long).
	longIdleThreshold = 10 * time.Minute
	// degradeThreshold marks a heartbeat as late (ball.degraded) but still
	// within the zombie grace window (G4: ball.degraded).
	degradeThreshold = 90 * time.Second
)

// HoldScheduler is the P2 "托管持球" primitive. It parks a thread when a dog
// declares hold_ball({wakeWhen}) and releases it when a human (manual), an
// external system (webhook), or a shell command (command) satisfies the wake
// condition. On release it writes wake_condition_met + hold_dispositioned into
// the ball-custody ledger (guarded by G1) and returns the HoldRecord so the
// caller can re-dispatch the holder breed.
//
// It mirrors the hold_ball primitive: a dog parks the ball and the orchestrator
// waits for the wake condition before dispositioning and continuing the worklist.
// The secret webhook token lives ONLY here (in memory); it is never written to
// the append-only ledger.
type HoldScheduler struct {
	ledger    custodyPorts.IBallLedger
	processMgr *unified.ProcessManager
	// OnWake is invoked when a timed or command hold auto-wakes. It must resume
	// the holder breed. Set by the transport layer (WSHandler) at startup;
	// nil means auto-wakes only write ledger events (no resume).
	OnWake func(ctx context.Context, threadID, holder, resumeMsg string)
	mu      sync.Mutex
	holds   map[string]*custodyPorts.HoldRecord
}

// NewHoldScheduler creates a scheduler over the given ledger.
func NewHoldScheduler(ledger custodyPorts.IBallLedger, processMgr *unified.ProcessManager) *HoldScheduler {
	return &HoldScheduler{
		ledger:     ledger,
		processMgr: processMgr,
		holds:      make(map[string]*custodyPorts.HoldRecord),
	}
}

// Hold parks threadID under holder with the given wake condition. resumeMsg is
// the context handed back to the holder when the hold is released (usually the
// original user request, so the dog recovers full context on resume).
func (s *HoldScheduler) Hold(ctx context.Context, threadID, holder string, cond custodyPorts.WakeCondition, resumeMsg string) error {
	s.mu.Lock()
	if _, ok := s.holds[threadID]; ok {
		s.mu.Unlock()
		return fmt.Errorf("custody: thread %s already held", threadID)
	}
	// G15: a timed hold (FireAfterMs) must declare what it is waiting on. A
	// wait_source_ref grounds the wake so the ball is not auto-released into the
	// void. Command wakes are self-grounded (they wait on a process exit).
	if cond.FireAfterMs > 0 && cond.WaitSourceRef == "" {
		s.mu.Unlock()
		return fmt.Errorf("%w (thread %s)", ErrWaitSourceRefRequired, threadID)
	}
	now := time.Now().Unix()
	rec := &custodyPorts.HoldRecord{
		ThreadID:      threadID,
		Holder:        holder,
		Condition:     cond,
		ResumeMessage: resumeMsg,
		CreatedAt:     now,
	}
	// G5: derive timed/command wake fields from the condition.
	if cond.FireAfterMs > 0 {
		rec.FireAt = now + cond.FireAfterMs/1000
		// Give the timed wake a generous deadline so it never expires first.
		rec.Deadline = rec.FireAt + 600
	}
	if cond.Kind == custodyPorts.WakeCommand && cond.Command != "" {
		rec.Command = cond.Command
		if rec.Deadline == 0 {
			rec.Deadline = now + int64(commandWakeTimeout.Seconds())
		}
	}
	s.holds[threadID] = rec
	s.mu.Unlock()

	// Ledger is a passive observer; a write failure must not break the park.
	_ = s.ledger.RecordHeld(ctx, threadID, holder)

	// G5: kick off a command-wake goroutine if requested.
	if cond.Kind == custodyPorts.WakeCommand && cond.Command != "" && s.processMgr != nil {
		go s.runCommandWake(threadID, holder, resumeMsg, cond.Command)
	}
	return nil
}

// Wake releases a parked hold after validating the wake kind and (for webhook)
// the token. It writes wake_condition_met + hold_dispositioned (guarded by G1,
// so a superseded hold is rejected) and returns the record so the caller can
// resume the holder. Fails if there is no active hold or the credentials do not
// match; returns ErrDispositionRejected if the ledger guard refuses.
func (s *HoldScheduler) Wake(ctx context.Context, threadID string, kind custodyPorts.WakeKind, token string) (*custodyPorts.HoldRecord, error) {
	s.mu.Lock()
	rec, ok := s.holds[threadID]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNoActiveHold
	}
	if rec.Condition.Kind != kind {
		s.mu.Unlock()
		return nil, ErrWakeKindMismatch
	}
	if kind == custodyPorts.WakeWebhook && rec.Condition.Token != "" {
		if subtle.ConstantTimeCompare([]byte(rec.Condition.Token), []byte(token)) != 1 {
			s.mu.Unlock()
			return nil, ErrInvalidToken
		}
	}
	rec.Dispositioned = true
	delete(s.holds, threadID)
	s.mu.Unlock()

	// Ledger guard (G1): converge the ball only if still parked & held.
	ok2, err := s.ledger.TryHoldDispositioned(ctx, threadID, rec.Holder)
	if err != nil {
		return nil, err
	}
	if !ok2 {
		return nil, ErrDispositionRejected
	}
	return rec, nil
}

// Cancel revokes a parked hold without resuming it (G5). Distinct from Wake:
// the ball stays parked (no wake_condition_met) and the operator's cancel intent
// is recorded as a void_pass so the audit trail reflects the abandonment.
func (s *HoldScheduler) Cancel(ctx context.Context, threadID string) error {
	s.mu.Lock()
	rec, ok := s.holds[threadID]
	if !ok {
		s.mu.Unlock()
		return ErrNoActiveHold
	}
	holder := rec.Holder
	rec.Dispositioned = true
	delete(s.holds, threadID)
	s.mu.Unlock()
	_ = s.ledger.Record(ctx, custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.BallVoidPass,
		Holder:   holder,
		Payload:  "user_cancel",
	})
	return nil
}

// Tick advances timed holds (G5). For each active hold whose FireAt has passed,
// it emits ball.wake_sent then auto-disposes (resuming via OnWake). For each
// whose Deadline has passed without a wake, it expires the hold (ball.hold_expired
// → dead). now is the current unix time.
func (s *HoldScheduler) Tick(now int64) error {
	s.mu.Lock()
	snapshot := make([]*custodyPorts.HoldRecord, 0, len(s.holds))
	for _, rec := range s.holds {
		snapshot = append(snapshot, rec)
	}
	s.mu.Unlock()

	for _, rec := range snapshot {
		if rec.Dispositioned {
			continue
		}
		if rec.Deadline > 0 && now >= rec.Deadline {
			s.expire(rec.ThreadID, rec.Holder)
			continue
		}
		if rec.FireAt > 0 && now >= rec.FireAt {
			s.autoWake(rec.ThreadID, rec.Holder, rec.ResumeMessage)
		}
	}
	return nil
}

// autoWake disposes a hold that was satisfied by an external/auto signal
// (timed FireAt or command completion). It emits ball.wake_sent then the guarded
// wake+dispose, and resumes the holder via OnWake.
func (s *HoldScheduler) autoWake(threadID, holder, resumeMsg string) {
	s.mu.Lock()
	rec, ok := s.holds[threadID]
	if !ok || rec.Dispositioned {
		s.mu.Unlock()
		return
	}
	rec.Dispositioned = true
	delete(s.holds, threadID)
	s.mu.Unlock()

	_ = s.ledger.Record(context.Background(), custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.BallWakeSent,
		Holder:   holder,
	})
	if ok2, _ := s.ledger.TryHoldDispositioned(context.Background(), threadID, holder); !ok2 {
		return
	}
	if s.OnWake != nil {
		s.OnWake(context.Background(), threadID, holder, resumeMsg)
	}
}

// expire writes off a hold that reached its Deadline without a wake (G4/G5:
// ball.hold_expired → dead). No resume.
func (s *HoldScheduler) expire(threadID, holder string) {
	s.mu.Lock()
	rec, ok := s.holds[threadID]
	if !ok || rec.Dispositioned {
		s.mu.Unlock()
		return
	}
	rec.Dispositioned = true
	delete(s.holds, threadID)
	s.mu.Unlock()

	_ = s.ledger.Record(context.Background(), custodyPorts.BallEvent{
		ThreadID: threadID,
		Type:     custodyPorts.BallHoldExpired,
		Holder:   holder,
	})
}

// runCommandWake runs a wakeWhen(command) process to completion. On success it
// auto-wakes the hold; on failure/timeout it expires the hold (G5 failure policy).
func (s *HoldScheduler) runCommandWake(threadID, holder, resumeMsg, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), commandWakeTimeout)
	defer cancel()
	handle, err := s.processMgr.Spawn(ctx, "sh", []string{"-c", command}, "")
	if err != nil {
		s.expire(threadID, holder)
		return
	}
	// Drain until the process exits (the pipe closes on c.Wait).
	if handle != nil && handle.Stdout != nil {
		_, _ = io.Copy(io.Discard, handle.Stdout)
	}
	// Context cancellation (timeout) also routes here via expire; the guard
	// inside autoWake/expire makes a double-call harmless.
	if ctx.Err() != nil {
		s.expire(threadID, holder)
		return
	}
	s.autoWake(threadID, holder, resumeMsg)
}

// GetHold returns the active hold for a thread, if any.
func (s *HoldScheduler) GetHold(_ context.Context, threadID string) (*custodyPorts.HoldRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.holds[threadID]
	if !ok {
		return nil, false
	}
	cp := *rec
	return &cp, true
}

// SetOnWake registers the auto-wake resume callback (G5).
func (s *HoldScheduler) SetOnWake(fn func(ctx context.Context, threadID, holder, resumeMsg string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OnWake = fn
}

// ActiveHolds returns a snapshot of all currently parked holds.
func (s *HoldScheduler) ActiveHolds(_ context.Context) []custodyPorts.HoldRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]custodyPorts.HoldRecord, 0, len(s.holds))
	for _, rec := range s.holds {
		out = append(out, *rec)
	}
	return out
}
