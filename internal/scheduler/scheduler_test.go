package scheduler

import (
	"errors"
	"testing"
	"time"

	"sounds-great-ai/internal/domains/missions"
)

func TestParse_Interval(t *testing.T) {
	sp, err := Parse("every 30m")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if got := sp.Next(base); !got.Equal(base.Add(30 * time.Minute)) {
		t.Fatalf("interval next = %v", got)
	}
	sp2, _ := Parse("every 2h")
	if got := sp2.Next(base); !got.Equal(base.Add(2 * time.Hour)) {
		t.Fatalf("2h next = %v", got)
	}
}

func TestParse_Daily(t *testing.T) {
	sp, err := Parse("daily@09:30")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	got := sp.Next(base)
	if !got.Equal(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("daily earlier same day = %v", got)
	}
	// past the time today -> next day
	base2 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	got2 := sp.Next(base2)
	if !got2.Equal(time.Date(2026, 1, 2, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("daily next day = %v", got2)
	}
}

func TestParse_Cron(t *testing.T) {
	sp, err := Parse("0 9 * * *")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) // Thursday
	got := sp.Next(base)
	if !got.Equal(time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("cron daily 9am = %v", got)
	}
	// range + step
	sp2, err := Parse("*/15 0 * * *")
	if err != nil {
		t.Fatal(err)
	}
	// at 00:00, next should be 00:15 same day
	b2 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !sp2.Next(b2).Equal(time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)) {
		t.Fatal("cron */15 at midnight")
	}
}

func TestParse_Invalid(t *testing.T) {
	for _, bad := range []string{"", "every", "banana", "0 9 * *", "99 9 * * *"} {
		if _, err := Parse(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

// TestScheduler_TickFiresDue verifies a due task fires and a recurring task
// reschedules.
func TestScheduler_TickFiresDue(t *testing.T) {
	hub := missions.NewMissionHub()
	sc := New(hub)
	fixed := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	sc.nowFn = func() time.Time { return fixed }

	var ran int
	sc.SetHandler(func(t *missions.TaskSpec) error {
		ran++
		return nil
	})

	// one-shot due now
	oneshot := missions.NewTaskSpec("os", "remind", "u1")
	if err := sc.ScheduleCron(oneshot, ""); err != nil {
		t.Fatal(err)
	}
	n, err := sc.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || ran != 1 {
		t.Fatalf("expected 1 fired, got %d ran=%d", n, ran)
	}
	got, _ := hub.Get("os")
	if got.Status != missions.TaskDone {
		t.Fatalf("one-shot should be done, got %s", got.Status)
	}

	// recurring due now -> fired then rescheduled
	rec := missions.NewTaskSpec("rec", "stand", "u1")
	if err := sc.ScheduleCron(rec, "every 1h"); err != nil {
		t.Fatal(err)
	}
	nf, ok := sc.NextFire("rec")
	if !ok || nf.Before(fixed) {
		t.Fatalf("recurring next fire should be in future: %v", nf)
	}
	if _, err := sc.Tick(); err != nil {
		t.Fatal(err)
	}
	got2, _ := hub.Get("rec")
	if got2.Status != missions.TaskScheduled {
		t.Fatalf("recurring should reschedule to scheduled, got %s", got2.Status)
	}
}

func TestScheduler_TickNoHandler(t *testing.T) {
	hub := missions.NewMissionHub()
	sc := New(hub)
	sc.nowFn = func() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }
	_ = sc.ScheduleCron(missions.NewTaskSpec("x", "y", "u1"), "")
	if _, err := sc.Tick(); !errors.Is(err, ErrNoHandler) {
		t.Fatal("missing handler must error")
	}
}

func TestScheduler_HandlerErrorMarksFailed(t *testing.T) {
	hub := missions.NewMissionHub()
	sc := New(hub)
	sc.nowFn = func() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }
	sc.SetHandler(func(t *missions.TaskSpec) error { return errors.New("boom") })
	_ = sc.ScheduleCron(missions.NewTaskSpec("x", "y", "u1"), "")
	_, _ = sc.Tick()
	got, _ := hub.Get("x")
	if got.Status != missions.TaskFailed {
		t.Fatalf("handler error must mark failed, got %s", got.Status)
	}
	if got.LastError != "boom" {
		t.Fatalf("last error not recorded: %q", got.LastError)
	}
}
