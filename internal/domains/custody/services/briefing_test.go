package services

import (
	"context"
	"testing"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
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

// G14: MergeUnifiedTrail folds custody + repo events into one time-ordered axis,
// filtering repo events to the thread's [first, last] custody timestamp window.
func TestMergeUnifiedTrail(t *testing.T) {
	b := custodyPorts.Briefing{
		Trail: []custodyPorts.TrailEntry{
			{Seq: 1, Type: "ball.handed", Holder: "a", Timestamp: 100},
			{Seq: 2, Type: "ball.handed", Holder: "b", Timestamp: 300},
		},
	}
	repo := []custodyStores.RepoEvent{
		{Kind: "branch_pushed", Branch: "main", HeadSHA: "m1", At: 200}, // inside window
		{Kind: "branch_pushed", Branch: "dev", HeadSHA: "d1", At: 50},  // before window -> dropped
		{Kind: "branch_updated", Branch: "feat", HeadSHA: "f1", At: 400}, // after window -> dropped
	}
	out := MergeUnifiedTrail(b, repo)
	if len(out) != 3 {
		t.Fatalf("unified len = %d, want 3 (2 custody + 1 in-window repo)", len(out))
	}
	// time-ordered: 100 custody, 200 repo(main), 300 custody
	if out[0].Timestamp != 100 || out[0].Source != "custody" {
		t.Fatalf("out[0] = %+v", out[0])
	}
	if out[1].Timestamp != 200 || out[1].Source != "repo" || out[1].Branch != "main" {
		t.Fatalf("out[1] = %+v", out[1])
	}
	if out[2].Timestamp != 300 || out[2].Source != "custody" {
		t.Fatalf("out[2] = %+v", out[2])
	}
}

// G14: with no repo events the unified timeline equals the custody trail.
func TestMergeUnifiedTrailEmptyRepo(t *testing.T) {
	b := custodyPorts.Briefing{
		Trail: []custodyPorts.TrailEntry{
			{Seq: 1, Type: "ball.handed", Holder: "a", Timestamp: 100},
		},
	}
	out := MergeUnifiedTrail(b, nil)
	if len(out) != 1 || out[0].Source != "custody" {
		t.Fatalf("unified should be just custody trail, got %+v", out)
	}
}
