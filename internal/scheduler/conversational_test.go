package scheduler

import (
	"strings"
	"testing"
	"time"

	"sounds-great-ai/internal/domains/missions"
)

func TestParseConversational_DailyAM(t *testing.T) {
	d := ParseConversational("every day at 9am remind me to stand up", "th1", "u1")
	if d.CronExpr != "daily@09:00" {
		t.Fatalf("cron = %q", d.CronExpr)
	}
	if !strings.Contains(d.Title, "stand up") {
		t.Fatalf("title = %q", d.Title)
	}
}

func TestParseConversational_PM(t *testing.T) {
	d := ParseConversational("daily at 2:30pm sync the ledger", "th1", "u1")
	if d.CronExpr != "daily@14:30" {
		t.Fatalf("cron = %q", d.CronExpr)
	}
}

func TestParseConversational_Interval(t *testing.T) {
	d := ParseConversational("every 30 minutes check the mailbox", "th1", "u1")
	if d.CronExpr != "every 30m" {
		t.Fatalf("cron = %q", d.CronExpr)
	}
}

func TestParseConversational_OneShot(t *testing.T) {
	d := ParseConversational("remind me to call mom", "th1", "u1")
	if d.CronExpr != "" {
		t.Fatalf("expected one-shot, got %q", d.CronExpr)
	}
	if !strings.Contains(d.Title, "call mom") {
		t.Fatalf("title = %q", d.Title)
	}
}

func TestScheduler_RegisterConversational_WritesPool(t *testing.T) {
	hub := missions.NewMissionHub()
	sc := New(hub)
	sc.nowFn = func() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }

	spec, err := sc.RegisterConversational("every day at 8am water the plants", "th1", "u1")
	if err != nil {
		t.Fatalf("register conversational: %v", err)
	}
	if spec.Cron != "daily@08:00" {
		t.Fatalf("cron = %q", spec.Cron)
	}
	got, gerr := hub.Get(spec.ID)
	if gerr != nil {
		t.Fatalf("task not in pool: %v", gerr)
	}
	if got.Status != missions.TaskScheduled {
		t.Fatalf("registered task should be scheduled, got %s", got.Status)
	}
}
