package jobs

import (
	"errors"
	"testing"
	"time"
)

func TestManagedJob_Lifecycle(t *testing.T) {
	j := NewManagedJob("j1", "w1", "th1")
	if j.Status != JobCreated {
		t.Fatal("new job must be created")
	}
	if j.WorkID != "w1" || j.AttemptID == "" {
		t.Fatal("work identity / attempt not set")
	}
	if err := j.Transition(JobRunning); err != nil {
		t.Fatal(err)
	}
	if j.StartedAt.IsZero() {
		t.Fatal("started at not set")
	}
	if err := j.Transition(JobSucceeded); err != nil {
		t.Fatal(err)
	}
	if j.FinishedAt.IsZero() {
		t.Fatal("finished at not set")
	}
	// terminal
	if err := j.Transition(JobRunning); err == nil {
		t.Fatal("succeeded->running must be invalid")
	}
}

func TestManagedJob_RecoveryAfterRestart(t *testing.T) {
	store := NewInMemoryStore()
	j := NewManagedJob("j1", "w1", "th1")
	_ = j.Transition(JobRunning)
	_ = store.Save(j) // "crash" while running

	// simulate restart: load and recover
	interrupted, err := Recover(store)
	if err != nil || len(interrupted) != 1 {
		t.Fatalf("recover: %v len=%d", err, len(interrupted))
	}
	if interrupted[0].Status != JobRunning {
		t.Fatal("recovered job should still be running")
	}
	// resume: restart attempt then succeed (work not lost)
	_ = interrupted[0].Transition(JobCreated)
	_ = interrupted[0].Transition(JobRunning)
	_ = interrupted[0].Transition(JobSucceeded)
	_ = store.Save(interrupted[0])

	reloaded, _ := store.Load("j1")
	if reloaded.Status != JobSucceeded {
		t.Fatal("recovered job should finish succeeded")
	}
	if reloaded.WorkID != "w1" {
		t.Fatal("work identity must survive restart")
	}
}

func TestManagedJob_InvalidTransition(t *testing.T) {
	j := NewManagedJob("j", "w", "t")
	if err := j.Transition(JobSucceeded); err == nil {
		t.Fatal("created->succeeded must be invalid")
	}
}

func TestAttributionLedger_ThreeBuckets(t *testing.T) {
	l := NewAttributionLedger("w1")
	l.Add(BucketInput, 10)
	l.Add(BucketCompute, 25)
	l.Add(BucketOutput, 5)
	l.Add(BucketCompute, -3) // refund
	if l.Get(BucketInput) != 10 || l.Get(BucketCompute) != 22 || l.Get(BucketOutput) != 5 {
		t.Fatalf("bucket balances wrong: %+v", l.Buckets())
	}
	if l.Total() != 37 {
		t.Fatalf("total = %d, want 37", l.Total())
	}
	// immutable snapshot
	snap := l.Buckets()
	snap[BucketInput] = 999
	if l.Get(BucketInput) != 10 {
		t.Fatal("Buckets() must return a copy")
	}
}

func TestCallbackAuth_TombstoneFailClosed(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewCallbackAuth("tok", "j1", now.Add(time.Hour))
	if !c.Valid(now) {
		t.Fatal("fresh credential should be valid")
	}
	c.Tombstone()
	if c.Valid(now) {
		t.Fatal("tombstoned credential must be invalid even before expiry")
	}
	if !c.IsTombstoned() {
		t.Fatal("tombstone state")
	}
}

func TestCallbackAuth_Expiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewCallbackAuth("tok", "j1", now.Add(-time.Hour)) // already expired
	if c.Valid(now) {
		t.Fatal("expired credential must be invalid")
	}
}

func TestCallbackAuthStore_RejectsDead(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewCallbackAuthStore()
	c := NewCallbackAuth("tok", "j1", now.Add(time.Hour))
	store.Put(c)
	if _, ok := store.Lookup("j1", now); !ok {
		t.Fatal("valid credential must be returned")
	}
	c.Tombstone()
	if _, ok := store.Lookup("j1", now); ok {
		t.Fatal("tombstoned credential must not be returned")
	}
	if _, ok := store.Lookup("missing", now); ok {
		t.Fatal("missing job must not return a credential")
	}
}

// fakeTarget is a DeliveryTarget for delivery tests.
type fakeTarget struct {
	running bool
	got     []CanonicalEvent
}

func (f *fakeTarget) Running() bool { return f.running }
func (f *fakeTarget) Receive(e CanonicalEvent) error {
	f.got = append(f.got, e)
	return nil
}

func TestDeliverer_PreflightAndDeliver(t *testing.T) {
	d := NewDeliverer()
	ft := &fakeTarget{running: true}
	d.Register("j1", ft)

	if ok, err := d.Preflight("j1"); !ok || err != nil {
		t.Fatalf("preflight should pass for running target: %v", err)
	}
	e := CanonicalEvent{Kind: EventCancel, JobID: "j1", ThreadID: "th1"}
	if err := d.Deliver("j1", e); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if len(ft.got) != 1 || ft.got[0].Kind != EventCancel {
		t.Fatal("event not delivered")
	}
}

func TestDeliverer_RefusesDeadTarget(t *testing.T) {
	d := NewDeliverer()
	d.Register("j1", &fakeTarget{running: false})
	if ok, err := d.Preflight("j1"); ok || !errors.Is(err, ErrNotRunning) {
		t.Fatalf("preflight must refuse dead target: ok=%v err=%v", ok, err)
	}
	if err := d.Deliver("j1", CanonicalEvent{Kind: EventCancel, JobID: "j1"}); !errors.Is(err, ErrNotRunning) {
		t.Fatal("deliver must refuse dead target")
	}
	if _, err := d.Preflight("missing"); !errors.Is(err, ErrNoTarget) {
		t.Fatal("missing target must error")
	}
}
