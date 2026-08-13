package services

import (
	"context"
	"testing"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	custodyStores "sounds-great-ai/internal/domains/custody/stores"
)

// G6: ProjectDutyBriefing classifies threads into operator-actionable buckets.
func TestProjectDutyBriefingClassification(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	ctx := context.Background()
	// Seed a representative fleet.
	ev := func(threadID string, typ custodyPorts.BallEventType, holder string, ts int64) {
		_ = store.Append(ctx, custodyPorts.BallEvent{
			ThreadID: threadID, Type: typ, Holder: holder, Timestamp: ts,
		})
	}
	// t1: parked hold awaiting a human (needs user).
	ev("t1", custodyPorts.BallHeld, "bianmu", 50)
	// t2: abandoned -> dead.
	ev("t2", custodyPorts.BallHanded, "a", 60)
	ev("t2", custodyPorts.BallAbandoned, "a", 61)
	// t3: void pass (no valid target).
	ev("t3", custodyPorts.BallVoidPass, "", 70)
	// t4a / t4b: blocked and aged (stale).
	ev("t4a", custodyPorts.TaskBlocked, "b", 100)
	ev("t4b", custodyPorts.TaskBlocked, "c", 200)
	// t5: active, not actionable.
	ev("t5", custodyPorts.BallHanded, "d", 80)

	l := NewBallLedger(store)
	brief, err := l.ProjectDutyBriefing(ctx)
	if err != nil {
		t.Fatalf("ProjectDutyBriefing: %v", err)
	}

	if len(brief.NeedsUser) != 1 || brief.NeedsUser[0].ThreadID != "t1" {
		t.Fatalf("NeedsUser = %+v, want [t1]", brief.NeedsUser)
	}
	if len(brief.DeadBalls) != 1 || brief.DeadBalls[0].ThreadID != "t2" {
		t.Fatalf("DeadBalls = %+v, want [t2]", brief.DeadBalls)
	}
	if len(brief.VoidPasses) != 1 || brief.VoidPasses[0].ThreadID != "t3" {
		t.Fatalf("VoidPasses = %+v, want [t3]", brief.VoidPasses)
	}
	if len(brief.StaleBlocked) != 2 {
		t.Fatalf("StaleBlocked len = %d, want 2", len(brief.StaleBlocked))
	}
	// StaleBlocked sorted by UpdatedAt descending: t4b(200) then t4a(100).
	if brief.StaleBlocked[0].ThreadID != "t4b" || brief.StaleBlocked[1].ThreadID != "t4a" {
		t.Fatalf("StaleBlocked order = [%s,%s], want [t4b,t4a]",
			brief.StaleBlocked[0].ThreadID, brief.StaleBlocked[1].ThreadID)
	}

	wantCounts := map[string]int{
		"parked":  1,
		"dead":    1,
		"void":    1,
		"blocked": 2,
		"active":  1,
	}
	for k, v := range wantCounts {
		if brief.Counts[k] != v {
			t.Fatalf("Counts[%s] = %d, want %d (full: %+v)", k, brief.Counts[k], v, brief.Counts)
		}
	}
}

// G6: a thread that projects to "new" (only a single no-op-ish event set) is
// excluded from the briefing.
func TestProjectDutyBriefingSkipsNew(t *testing.T) {
	store := custodyStores.NewMemoryBallLedgerStore()
	ctx := context.Background()
	// A thread with only an InvocationStarted never leaves "new" (no holder
	// handoff/held). It must not appear in any bucket.
	_ = store.Append(ctx, custodyPorts.BallEvent{
		ThreadID: "ghost", Type: custodyPorts.InvocationStarted, Holder: "a", Timestamp: 1,
	})
	l := NewBallLedger(store)
	brief, err := l.ProjectDutyBriefing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(brief.NeedsUser) != 0 || len(brief.DeadBalls) != 0 ||
		len(brief.VoidPasses) != 0 || len(brief.StaleBlocked) != 0 {
		t.Fatalf("ghost thread leaked into briefing: %+v", brief)
	}
	if brief.Counts["new"] != 0 {
		t.Fatalf("new thread should be excluded from counts, got %+v", brief.Counts)
	}
}
