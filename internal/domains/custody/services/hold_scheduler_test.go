package services

import (
	"context"
	"testing"
	"time"

	"sounds-great-ai/internal/adapter/unified"
	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	custodyStores "sounds-great-ai/internal/domains/custody/stores"
)

func newTestScheduler() (*HoldScheduler, custodyPorts.IBallLedger) {
	store := custodyStores.NewMemoryBallLedgerStore()
	ledger := NewBallLedger(store)
	return NewHoldScheduler(ledger, nil), ledger
}

func TestHoldSchedulerParkAndSnapshot(t *testing.T) {
	s, ledger := newTestScheduler()
	ctx := context.Background()

	if err := s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual}, "original ask"); err != nil {
		t.Fatalf("Hold: %v", err)
	}

	// A held thread projects to parked.
	snap, err := ledger.Snapshot(ctx, "t1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.State != custodyPorts.BallStateParked {
		t.Fatalf("state = %s, want parked", snap.State)
	}
	if snap.Holder != "bianmu" {
		t.Fatalf("holder = %s, want bianmu", snap.Holder)
	}

	// ActiveHolds reflects it.
	if holds := s.ActiveHolds(ctx); len(holds) != 1 {
		t.Fatalf("ActiveHolds len = %d, want 1", len(holds))
	}
	rec, ok := s.GetHold(ctx, "t1")
	if !ok || rec.Holder != "bianmu" {
		t.Fatalf("GetHold = %v,%v", rec, ok)
	}
}

func TestHoldSchedulerWakeManual(t *testing.T) {
	s, ledger := newTestScheduler()
	ctx := context.Background()
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual}, "ask")

	rec, err := s.Wake(ctx, "t1", custodyPorts.WakeManual, "")
	if err != nil {
		t.Fatalf("Wake: %v", err)
	}
	if rec.Holder != "bianmu" || rec.Dispositioned != true {
		t.Fatalf("returned record wrong: %+v", rec)
	}

	// No longer active.
	if _, ok := s.GetHold(ctx, "t1"); ok {
		t.Fatal("hold should be gone after wake")
	}
	if holds := s.ActiveHolds(ctx); len(holds) != 0 {
		t.Fatalf("ActiveHolds len = %d, want 0", len(holds))
	}

	// Ledger: held -> wake_condition_met -> hold_dispositioned => active.
	snap, _ := ledger.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateActive {
		t.Fatalf("state = %s, want active", snap.State)
	}
}

func TestHoldSchedulerWakeWrongKind(t *testing.T) {
	s, _ := newTestScheduler()
	ctx := context.Background()
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual}, "ask")

	if _, err := s.Wake(ctx, "t1", custodyPorts.WakeWebhook, ""); err != ErrWakeKindMismatch {
		t.Fatalf("err = %v, want ErrWakeKindMismatch", err)
	}
	// Hold still active.
	if _, ok := s.GetHold(ctx, "t1"); !ok {
		t.Fatal("hold should remain after mismatched wake")
	}
}

func TestHoldSchedulerWakeWebhookToken(t *testing.T) {
	s, _ := newTestScheduler()
	ctx := context.Background()
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeWebhook, Token: "secret-123"}, "ask")

	if _, err := s.Wake(ctx, "t1", custodyPorts.WakeWebhook, "wrong"); err != ErrInvalidToken {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
	if _, err := s.Wake(ctx, "t1", custodyPorts.WakeWebhook, "secret-123"); err != nil {
		t.Fatalf("Wake with correct token: %v", err)
	}
	if _, ok := s.GetHold(ctx, "t1"); ok {
		t.Fatal("hold should be released after correct token")
	}
}

func TestHoldSchedulerWakeNoActiveHold(t *testing.T) {
	s, _ := newTestScheduler()
	ctx := context.Background()
	if _, err := s.Wake(ctx, "nope", custodyPorts.WakeManual, ""); err != ErrNoActiveHold {
		t.Fatalf("err = %v, want ErrNoActiveHold", err)
	}
}

func TestHoldSchedulerDoubleHoldRejected(t *testing.T) {
	s, _ := newTestScheduler()
	ctx := context.Background()
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual}, "ask")
	if err := s.Hold(ctx, "t1", "xigou", custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual}, "ask2"); err == nil {
		t.Fatal("expected double-hold to be rejected")
	}
}

func TestHoldSchedulerDoubleWakeRejected(t *testing.T) {
	s, _ := newTestScheduler()
	ctx := context.Background()
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual}, "ask")
	if _, err := s.Wake(ctx, "t1", custodyPorts.WakeManual, ""); err != nil {
		t.Fatalf("first wake: %v", err)
	}
	if _, err := s.Wake(ctx, "t1", custodyPorts.WakeManual, ""); err != ErrNoActiveHold {
		t.Fatalf("second wake err = %v, want ErrNoActiveHold", err)
	}
}

// G5: Cancel revokes a parked hold without resuming it (ball.void_pass).
func TestHoldSchedulerCancel(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	ledger := NewBallLedger(store)
	s := NewHoldScheduler(ledger, nil)
	ctx := context.Background()
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual}, "ask")

	if err := s.Cancel(ctx, "t1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, ok := s.GetHold(ctx, "t1"); ok {
		t.Fatal("hold should be gone after cancel")
	}
	snap, _ := ledger.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateVoid {
		t.Fatalf("state = %s, want void (cancel -> void_pass)", snap.State)
	}
	// Canceling a non-existent hold reports ErrNoActiveHold.
	if err := s.Cancel(ctx, "t1"); err != ErrNoActiveHold {
		t.Fatalf("second cancel err = %v, want ErrNoActiveHold", err)
	}
}

// G5: Tick auto-wakes a timed hold once FireAt passes (ball.wake_sent + dispose).
func TestHoldSchedulerTickAutoWake(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	ledger := NewBallLedger(store)
	s := NewHoldScheduler(ledger, nil)
	ctx := context.Background()
	woken := make(chan string, 1)
	s.SetOnWake(func(_ context.Context, threadID, _ /*holder*/, _ /*resumeMsg*/ string) {
		woken <- threadID
	})
	// Park with a timed wake 1s out.
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual, FireAfterMs: 1000}, "ask")
	snap, _ := ledger.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateParked {
		t.Fatalf("state = %s, want parked", snap.State)
	}

	// Advance the clock comfortably past FireAt.
	if err := s.Tick(time.Now().Unix() + 10); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if _, ok := s.GetHold(ctx, "t1"); ok {
		t.Fatal("hold should be auto-released by Tick")
	}
	snap, _ = ledger.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateActive {
		t.Fatalf("state = %s, want active after auto-wake", snap.State)
	}
	select {
	case tid := <-woken:
		if tid != "t1" {
			t.Fatalf("OnWake thread = %s, want t1", tid)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnWake not invoked on auto-wake")
	}
}

// G5: Tick expires a hold that reached its Deadline without a wake
// (ball.hold_expired -> dead). processMgr is nil so no command runs.
func TestHoldSchedulerTickExpire(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	ledger := NewBallLedger(store)
	s := NewHoldScheduler(ledger, nil)
	ctx := context.Background()
	// Command hold (no FireAfterMs) => Deadline = now + 5min.
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeCommand, Command: "true"}, "ask")

	// Advance past the 5-minute deadline.
	if err := s.Tick(time.Now().Unix() + 6*60); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	snap, _ := ledger.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateDead {
		t.Fatalf("state = %s, want dead after expire", snap.State)
	}
	if _, ok := s.GetHold(ctx, "t1"); ok {
		t.Fatal("expired hold should be gone")
	}
}

// G5: a WakeCommand hold auto-disposes when the shell command completes.
func TestHoldSchedulerCommandWake(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	ledger := NewBallLedger(store)
	pm := unified.NewProcessManager()
	s := NewHoldScheduler(ledger, pm)
	ctx := context.Background()
	woken := make(chan string, 1)
	s.SetOnWake(func(_ context.Context, threadID, _ /*holder*/, _ /*resumeMsg*/ string) {
		woken <- threadID
	})
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeCommand, Command: "true"}, "ask")

	select {
	case tid := <-woken:
		if tid != "t1" {
			t.Fatalf("OnWake thread = %s, want t1", tid)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("command wake did not fire")
	}
	snap, _ := ledger.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateActive {
		t.Fatalf("state = %s, want active after command wake", snap.State)
	}
	if _, ok := s.GetHold(ctx, "t1"); ok {
		t.Fatal("hold should be released after command wake")
	}
}

// G5: Wake respects the G1 ledger guard and returns ErrDispositionRejected when
// the park was superseded (e.g. a newer handoff disposed the ball first).
func TestHoldSchedulerWakeGuardRejected(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	ledger := NewBallLedger(store)
	s := NewHoldScheduler(ledger, nil)
	ctx := context.Background()
	_ = s.Hold(ctx, "t1", "bianmu", custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual}, "ask")

	// A concurrent handoff disposes the ball to "xigou" (no longer parked).
	if err := ledger.RecordHanded(ctx, "t1", "bianmu", "xigou"); err != nil {
		t.Fatal(err)
	}
	// Wake should now be rejected by the guard.
	_, err := s.Wake(ctx, "t1", custodyPorts.WakeManual, "")
	if err != ErrDispositionRejected {
		t.Fatalf("err = %v, want ErrDispositionRejected", err)
	}
	// The stale hold is consumed (so a future legitimate park for bianmu is not
	// blocked by the double-hold guard). Crucially the BALL is NOT converged:
	// it must remain with xigou (active).
	if _, ok := s.GetHold(ctx, "t1"); ok {
		t.Fatal("stale hold should be consumed after rejected wake")
	}
	snap, _ := ledger.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateActive || snap.Holder != "xigou" {
		t.Fatalf("ball should remain with xigou, got holder=%s state=%s", snap.Holder, snap.State)
	}
}
