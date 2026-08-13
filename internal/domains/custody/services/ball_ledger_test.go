package services

import (
	"context"
	"testing"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	custodyStores "sounds-great-ai/internal/domains/custody/stores"
)

func newTestLedger() *BallLedger {
	return NewBallLedger(custodyStores.NewMemoryBallLedgerStore())
}

func ev(t custodyPorts.BallEventType, holder, to string) custodyPorts.BallEvent {
	return custodyPorts.BallEvent{Type: t, Holder: holder, To: to, Timestamp: 1}
}

func TestProject_StateTransitions(t *testing.T) {
	cases := []struct {
		name string
		in   []custodyPorts.BallEvent
		want custodyPorts.BallState
	}{
		{"empty", nil, custodyPorts.BallStateNew},
		{"handed", []custodyPorts.BallEvent{ev(custodyPorts.BallHanded, "", "bianmu")}, custodyPorts.BallStateActive},
		{"handed+started", []custodyPorts.BallEvent{ev(custodyPorts.BallHanded, "", "bianmu"), ev(custodyPorts.InvocationStarted, "bianmu", "")}, custodyPorts.BallStateActive},
		{"handed+died", []custodyPorts.BallEvent{ev(custodyPorts.BallHanded, "", "bianmu"), ev(custodyPorts.InvocationDied, "bianmu", "")}, custodyPorts.BallStateZombie},
		{"handed+done", []custodyPorts.BallEvent{ev(custodyPorts.BallHanded, "", "bianmu"), ev(custodyPorts.TaskDone, "bianmu", "")}, custodyPorts.BallStateResolved},
		{"handed+held", []custodyPorts.BallEvent{ev(custodyPorts.BallHanded, "", "bianmu"), ev(custodyPorts.BallHeld, "bianmu", "")}, custodyPorts.BallStateParked},
		{"handed+blocked", []custodyPorts.BallEvent{ev(custodyPorts.BallHanded, "", "bianmu"), ev(custodyPorts.TaskBlocked, "bianmu", "")}, custodyPorts.BallStateBlocked},
		{"dispatch", []custodyPorts.BallEvent{ev(custodyPorts.BallHanded, "", "bianmu"), ev(custodyPorts.DispatchDispositioned, "bianmu", "xigou")}, custodyPorts.BallStateActive},
		{"held+dispositioned", []custodyPorts.BallEvent{ev(custodyPorts.BallHeld, "bianmu", ""), ev(custodyPorts.HoldDispositioned, "bianmu", "")}, custodyPorts.BallStateActive},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Project(c.in)
			if got.State != c.want {
				t.Fatalf("state = %s, want %s", got.State, c.want)
			}
		})
	}
}

func TestBallLedger_RecordAndSnapshot(t *testing.T) {
	l := newTestLedger()
	ctx := context.Background()
	const tid = "thread-1"

	if err := l.RecordHanded(ctx, tid, "", "bianmu"); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordInvocationStarted(ctx, tid, "bianmu"); err != nil {
		t.Fatal(err)
	}
	snap, err := l.Snapshot(ctx, tid)
	if err != nil {
		t.Fatal(err)
	}
	if snap.State != custodyPorts.BallStateActive || snap.Holder != "bianmu" {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.Generation != 1 {
		t.Fatalf("generation = %d, want 1", snap.Generation)
	}

	// Handoff to xigou should bump generation and move holder.
	if err := l.RecordDispatchDispositioned(ctx, tid, "bianmu", "xigou"); err != nil {
		t.Fatal(err)
	}
	snap, _ = l.Snapshot(ctx, tid)
	if snap.Generation != 2 || snap.Holder != "xigou" {
		t.Fatalf("after handoff snapshot = %+v", snap)
	}

	if err := l.RecordTaskDone(ctx, tid, "xigou"); err != nil {
		t.Fatal(err)
	}
	snap, _ = l.Snapshot(ctx, tid)
	if snap.State != custodyPorts.BallStateResolved {
		t.Fatalf("final state = %s, want resolved", snap.State)
	}
}
