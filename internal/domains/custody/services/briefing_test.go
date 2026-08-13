package services

import (
	"context"
	"testing"

	custodyStores "sounds-great-ai/internal/domains/custody/stores"
)

func TestProjectTrail(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	l := NewBallLedger(store)
	ctx := context.Background()
	tid := "thread-1"

	if err := l.RecordHanded(ctx, tid, "", "bianmu"); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordInvocationStarted(ctx, tid, "bianmu"); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordDispatchDispositioned(ctx, tid, "bianmu", "xigou"); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordInvocationStarted(ctx, tid, "xigou"); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordHeld(ctx, tid, "xigou"); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordTaskDone(ctx, tid, "xigou"); err != nil {
		t.Fatal(err)
	}

	brief, err := l.ProjectTrail(ctx, tid)
	if err != nil {
		t.Fatal(err)
	}
	if brief.ThreadID != tid {
		t.Fatalf("ThreadID = %q, want %q", brief.ThreadID, tid)
	}
	if brief.State != "resolved" {
		t.Fatalf("State = %q, want resolved", brief.State)
	}
	if brief.Holder != "xigou" {
		t.Fatalf("Holder = %q, want xigou", brief.Holder)
	}
	if brief.Turns != 3 {
		t.Fatalf("Turns = %d, want 3", brief.Turns)
	}
	if brief.Handoffs != 1 {
		t.Fatalf("Handoffs = %d, want 1", brief.Handoffs)
	}
	if brief.Holds != 1 {
		t.Fatalf("Holds = %d, want 1", brief.Holds)
	}
	if len(brief.Trail) != 6 {
		t.Fatalf("Trail len = %d, want 6", len(brief.Trail))
	}
	// Trail must be in append order (seq ascending).
	for i := 1; i < len(brief.Trail); i++ {
		if brief.Trail[i].Seq < brief.Trail[i-1].Seq {
			t.Fatalf("trail not ordered at index %d", i)
		}
	}
}

func TestProjectTrailEmpty(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	l := NewBallLedger(store)
	brief, err := l.ProjectTrail(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if brief.State != "new" {
		t.Fatalf("State = %q, want new for empty thread", brief.State)
	}
	if len(brief.Trail) != 0 {
		t.Fatalf("Trail len = %d, want 0", len(brief.Trail))
	}
}
