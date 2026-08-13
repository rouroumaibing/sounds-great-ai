package services

import (
	"context"
	"testing"
	"time"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	custodyStores "sounds-great-ai/internal/domains/custody/stores"
)

func TestReconcileZombiesHealsStaleInFlight(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	l := NewBallLedger(store)
	ctx := context.Background()

	old := time.Now().Add(-10 * time.Minute).Unix()
	_ = store.Append(ctx, custodyPorts.BallEvent{
		ThreadID: "t1", Type: custodyPorts.InvocationStarted, Holder: "bianmu", Timestamp: old,
	})

	n, err := l.ReconcileZombies(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("ReconcileZombies error: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 healed zombie, got %d", n)
	}
	snap, _ := l.Snapshot(ctx, "t1")
	if snap.State != custodyPorts.BallStateZombie {
		t.Fatalf("want zombie state after heal, got %s", snap.State)
	}
}

func TestReconcileZombiesSkipsRecentWithHeartbeat(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	l := NewBallLedger(store)
	ctx := context.Background()
	now := time.Now().Unix()
	_ = store.Append(ctx, custodyPorts.BallEvent{ThreadID: "t2", Type: custodyPorts.InvocationStarted, Holder: "xigou", Timestamp: now})
	_ = store.Append(ctx, custodyPorts.BallEvent{ThreadID: "t2", Type: custodyPorts.InvocationHeartbeat, Holder: "xigou", Timestamp: now})

	n, _ := l.ReconcileZombies(ctx, 5*time.Minute)
	if n != 0 {
		t.Fatalf("want 0 healed (recent + heartbeat), got %d", n)
	}
}

func TestReconcileZombiesSkipsCompleted(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	l := NewBallLedger(store)
	ctx := context.Background()
	old := time.Now().Add(-10 * time.Minute).Unix()
	_ = store.Append(ctx, custodyPorts.BallEvent{ThreadID: "t3", Type: custodyPorts.InvocationStarted, Holder: "jinmao", Timestamp: old})
	_ = store.Append(ctx, custodyPorts.BallEvent{ThreadID: "t3", Type: custodyPorts.TaskDone, Holder: "jinmao", Timestamp: old + 1})

	n, _ := l.ReconcileZombies(ctx, 5*time.Minute)
	if n != 0 {
		t.Fatalf("want 0 healed (completed), got %d", n)
	}
}

func TestReconcileZombiesSkipsRecentNoHeartbeat(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	l := NewBallLedger(store)
	ctx := context.Background()
	// Started just now with no heartbeat yet: still within timeout -> not healed.
	_ = store.Append(ctx, custodyPorts.BallEvent{ThreadID: "t4", Type: custodyPorts.InvocationStarted, Holder: "demu", Timestamp: time.Now().Unix()})

	n, _ := l.ReconcileZombies(ctx, 5*time.Minute)
	if n != 0 {
		t.Fatalf("want 0 healed (started recently, no heartbeat), got %d", n)
	}
}

func TestReconcileZombiesHealsOnlyLastInFlight(t *testing.T) {
	// Two invocations; the first completed, the second is stale -> only 1 healed.
	store := custodyStores.NewMemoryBallLedgerStore()
	l := NewBallLedger(store)
	ctx := context.Background()
	base := time.Now().Add(-20 * time.Minute).Unix()
	_ = store.Append(ctx, custodyPorts.BallEvent{ThreadID: "t5", Type: custodyPorts.InvocationStarted, Holder: "a", Timestamp: base})
	_ = store.Append(ctx, custodyPorts.BallEvent{ThreadID: "t5", Type: custodyPorts.TaskDone, Holder: "a", Timestamp: base + 1})
	_ = store.Append(ctx, custodyPorts.BallEvent{ThreadID: "t5", Type: custodyPorts.InvocationStarted, Holder: "b", Timestamp: base + 2})

	n, _ := l.ReconcileZombies(ctx, 5*time.Minute)
	if n != 1 {
		t.Fatalf("want 1 healed (only last in-flight), got %d", n)
	}
}
