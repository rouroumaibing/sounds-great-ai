package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestSnapshotStore_History(t *testing.T) {
	collect := func() string { return "# sample" }
	s := NewSnapshotStore(720, 10*time.Millisecond, collect)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	hist := s.History(time.Time{})
	if len(hist) == 0 {
		t.Fatal("expected non-empty history after start")
	}
	for _, snap := range hist {
		if snap.Text != "# sample" {
			t.Fatalf("unexpected text: %q", snap.Text)
		}
	}
}

func TestSnapshotStore_Capacity(t *testing.T) {
	collect := func() string { return "x" }
	s := NewSnapshotStore(3, 5*time.Millisecond, collect)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	time.Sleep(80 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	if c := s.SnapshotCount(); c > 3 {
		t.Fatalf("expected <= 3 snapshots, got %d", c)
	}
}

func TestSnapshotStore_History_Since(t *testing.T) {
	collect := func() string { return "x" }
	s := NewSnapshotStore(720, 5*time.Millisecond, collect)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	mid := time.Now().Add(-10 * time.Millisecond)
	hist := s.History(mid)
	for _, snap := range hist {
		if snap.Timestamp.Before(mid) {
			t.Fatal("expected all snapshots >= since")
		}
	}
}
