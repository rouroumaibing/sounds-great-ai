package eval

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupMiniredis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return client, mr
}

func TestRedisClosureAppendAndState(t *testing.T) {
	client, _ := setupMiniredis(t)
	svc := NewClosureService(client)
	ctx := context.Background()

	verdictID := "v-1"
	ev := ClosureEvent{
		ID:        "e-1",
		Type:      "verdict_opened",
		Timestamp: time.Now(),
		Actor:     "system",
	}
	if err := svc.AppendEvent(ctx, verdictID, ev); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	state, err := svc.CurrentState(ctx, verdictID)
	if err != nil {
		t.Fatalf("CurrentState: %v", err)
	}
	if state != StateOpen {
		t.Errorf("state = %q, want open", state)
	}
}

func TestRedisClosureIdempotentEvent(t *testing.T) {
	client, _ := setupMiniredis(t)
	svc := NewClosureService(client)
	ctx := context.Background()

	verdictID := "v-2"
	ev := ClosureEvent{ID: "e-dup", Type: "verdict_opened", Timestamp: time.Now(), Actor: "system"}
	_ = svc.AppendEvent(ctx, verdictID, ev)
	err := svc.AppendEvent(ctx, verdictID, ev) // same eventID
	if err != ErrDuplicateEvent {
		t.Fatalf("expected ErrDuplicateEvent, got %v", err)
	}
}

func TestRedisClosureStateTransitions(t *testing.T) {
	client, _ := setupMiniredis(t)
	svc := NewClosureService(client)
	ctx := context.Background()
	verdictID := "v-3"

	events := []ClosureEvent{
		{ID: "1", Type: "verdict_opened", Timestamp: time.Now(), Actor: "system"},
		{ID: "2", Type: "owner_acknowledged", Timestamp: time.Now(), Actor: "user"},
		{ID: "3", Type: "action_planned", Timestamp: time.Now(), Actor: "user"},
		{ID: "4", Type: "fix_recorded", Timestamp: time.Now(), Actor: "dev"},
		{ID: "5", Type: "reeval_requested", Timestamp: time.Now(), Actor: "system"},
		{ID: "6", Type: "reeval_passed", Timestamp: time.Now(), Actor: "system"},
	}
	for _, ev := range events {
		if err := svc.AppendEvent(ctx, verdictID, ev); err != nil {
			t.Fatalf("AppendEvent %s: %v", ev.Type, err)
		}
	}
	state, _ := svc.CurrentState(ctx, verdictID)
	if state != StateResolved {
		t.Errorf("state = %q, want resolved", state)
	}
}

func TestMemoryClosureMatchesRedis(t *testing.T) {
	svc := NewClosureService(nil) // nil redis → memory fallback
	ctx := context.Background()
	verdictID := "v-mem"

	ev := ClosureEvent{ID: "m-1", Type: "verdict_opened", Timestamp: time.Now(), Actor: "system"}
	if err := svc.AppendEvent(ctx, verdictID, ev); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	state, _ := svc.CurrentState(ctx, verdictID)
	if state != StateOpen {
		t.Errorf("memory state = %q, want open", state)
	}
}

func TestNewClosureServiceReturnsMemoryWhenNil(t *testing.T) {
	svc := NewClosureService(nil)
	if _, ok := svc.(*MemoryClosureService); !ok {
		t.Fatal("expected MemoryClosureService for nil redis")
	}
}
