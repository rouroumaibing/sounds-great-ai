package eval

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSchedulerIsDueFailOpenWhenRedisNil(t *testing.T) {
	s := &Scheduler{redis: nil}
	d := EvalDomain{DomainID: "eval:test", Frequency: "daily"}
	if !s.isDue(context.Background(), d) {
		t.Fatal("expected fail-open (true) when redis is nil")
	}
}

func TestSchedulerIsDueFailOpenOnRedisError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mr.Close() // kill redis → errors

	s := &Scheduler{redis: client}
	d := EvalDomain{DomainID: "eval:test", Frequency: "daily"}
	if !s.isDue(context.Background(), d) {
		t.Fatal("expected fail-open (true) on redis error")
	}
}

func TestSchedulerIsDueFalseWhenRecentlyDispatched(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	d := EvalDomain{DomainID: "eval:test", Frequency: "daily"}
	key := "eval-nday-last-dispatch:eval:test"
	client.Set(ctx, key, time.Now().UnixMilli(), 0)

	s := &Scheduler{redis: client}
	if s.isDue(ctx, d) {
		t.Fatal("expected not due (false) — just dispatched")
	}
}

func TestSchedulerEffectiveBreedOverride(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	d := EvalDomain{DomainID: "eval:a2a", EvalBreed: "bianmu"}
	client.HSet(ctx, "eval:cat-override:eval:a2a", "breedId", "demu")

	s := &Scheduler{redis: client}
	got := s.effectiveBreed(ctx, d)
	if got != "demu" {
		t.Errorf("effectiveBreed = %q, want demu", got)
	}
}

func TestSchedulerEffectiveBreedFallbackToYAML(t *testing.T) {
	s := &Scheduler{redis: nil}
	d := EvalDomain{DomainID: "eval:a2a", EvalBreed: "bianmu"}
	got := s.effectiveBreed(context.Background(), d)
	if got != "bianmu" {
		t.Errorf("effectiveBreed = %q, want bianmu (YAML default)", got)
	}
}
