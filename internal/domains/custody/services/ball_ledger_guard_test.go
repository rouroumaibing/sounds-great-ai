package services

import (
	"context"
	"testing"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	custodyStores "sounds-great-ai/internal/domains/custody/stores"
)

// newGuardTestLedger builds a store+ledger pair so tests can assert both the
// projected Snapshot and the raw append-only event log (for audit events).
func newGuardTestLedger() (*BallLedger, *custodyStores.MemoryBallLedgerStore) {
	store := custodyStores.NewMemoryBallLedgerStore()
	return NewBallLedger(store), store
}

func lastEventType(t *testing.T, store *custodyStores.MemoryBallLedgerStore, threadID string) custodyPorts.BallEventType {
	t.Helper()
	evs, err := store.GetEvents(context.Background(), threadID)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(evs) == 0 {
		t.Fatalf("no events for %s", threadID)
	}
	return evs[len(evs)-1].Type
}

// G1: TryDispatchDispositioned converges only when the caller is still the holder.
func TestGuardDispatchStaleCallbackRejected(t *testing.T) {
	l, store := newGuardTestLedger()
	ctx := context.Background()

	// Ball handed to "a" -> active, holder a.
	if err := l.RecordHanded(ctx, "t1", "", "a"); err != nil {
		t.Fatal(err)
	}
	ok, err := l.TryDispatchDispositioned(ctx, "t1", "a", "b")
	if err != nil || !ok {
		t.Fatalf("first dispatch: ok=%v err=%v", ok, err)
	}
	snap, _ := l.Snapshot(ctx, "t1")
	if snap.Holder != "b" || snap.State != custodyPorts.BallStateActive {
		t.Fatalf("after dispatch holder=%s state=%s", snap.Holder, snap.State)
	}

	// A duplicate/stale callback from "a" (superseded by b) must be rejected.
	ok, err = l.TryDispatchDispositioned(ctx, "t1", "a", "c")
	if err != nil {
		t.Fatalf("stale dispatch err: %v", err)
	}
	if ok {
		t.Fatal("stale dispatch should be rejected")
	}
	// State must NOT change; an audit event must be written.
	snap, _ = l.Snapshot(ctx, "t1")
	if snap.Holder != "b" {
		t.Fatalf("stale dispatch mutated holder to %s", snap.Holder)
	}
	if lastEventType(t, store, "t1") != custodyPorts.BallDispositionRejected {
		t.Fatalf("expected ball.disposition_rejected audit event")
	}
}

// G1: TryHoldDispositioned converges only when parked & held by the waker.
func TestGuardHoldDispositionOnlyWhenParked(t *testing.T) {
	l, store := newGuardTestLedger()
	ctx := context.Background()

	if err := l.RecordHeld(ctx, "t1", "bianmu"); err != nil {
		t.Fatal(err)
	}
	ok, err := l.TryHoldDispositioned(ctx, "t1", "bianmu")
	if err != nil || !ok {
		t.Fatalf("hold disposition: ok=%v err=%v", ok, err)
	}
	snap, _ := l.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateActive {
		t.Fatalf("expected active after wake, got %s", snap.State)
	}

	// The ball is already active; a second hold-disposition must be rejected.
	ok, err = l.TryHoldDispositioned(ctx, "t1", "bianmu")
	if err != nil {
		t.Fatalf("second hold disposition err: %v", err)
	}
	if ok {
		t.Fatal("second hold disposition should be rejected")
	}
	if lastEventType(t, store, "t1") != custodyPorts.BallDispositionRejected {
		t.Fatalf("expected ball.disposition_rejected audit event")
	}
}

// G1: a hold-disposition is rejected when a newer handoff superseded the park.
func TestGuardHoldDispositionSupersededByHandoff(t *testing.T) {
	l, _ := newGuardTestLedger()
	ctx := context.Background()

	// Park under "a".
	if err := l.RecordHeld(ctx, "t1", "a"); err != nil {
		t.Fatal(err)
	}
	// A concurrent handoff disposes the ball to "b" (active).
	if err := l.RecordHanded(ctx, "t1", "a", "b"); err != nil {
		t.Fatal(err)
	}
	// Now "a" tries to dispose its hold: the ball is no longer parked/held by a.
	ok, err := l.TryHoldDispositioned(ctx, "t1", "a")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatal("hold disposition should be rejected after superseding handoff")
	}
	snap, _ := l.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateActive || snap.Holder != "b" {
		t.Fatalf("ball should remain with b, got holder=%s state=%s", snap.Holder, snap.State)
	}
}
