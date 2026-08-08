package cue

import (
	"context"
	"testing"
	"time"
)

func TestResolverRegistryFailClosed(t *testing.T) {
	reg := NewResolverRegistry()
	// No resolver registered for "person"
	result, err := reg.Resolve(context.Background(), "person", "subject", "reason", 300)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result for unregistered lane (fail-closed)")
	}
}

func TestStaticResolver(t *testing.T) {
	entries := []StaticEntry{
		{ID: "e1", Content: "User prefers Go"},
		{ID: "e2", Content: "User likes dark mode"},
	}
	resolver := NewStaticResolver("taste", entries)
	reg := NewResolverRegistry()
	reg.Register(resolver)

	result, err := reg.Resolve(context.Background(), "taste", "user prefs", "subject seen", 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if !result[0].IsRecalled {
		t.Error("expected IsRecalled=true")
	}
}

func TestTimeoutResolver(t *testing.T) {
	entries := []StaticEntry{{ID: "e1", Content: "test"}}
	inner := NewStaticResolver("entity", entries)
	resolver := NewTimeoutResolver(inner, 100*time.Millisecond)
	reg := NewResolverRegistry()
	reg.Register(resolver)

	result, err := reg.Resolve(context.Background(), "entity", "subject", "reason", 420)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
}

func TestResolverRegistryHasResolver(t *testing.T) {
	reg := NewResolverRegistry()
	if reg.HasResolver("person") {
		t.Error("should not have resolver for person")
	}
	reg.Register(NewStaticResolver("person", nil))
	if !reg.HasResolver("person") {
		t.Error("should have resolver for person")
	}
}

func TestResolverRegistryRegisteredLanes(t *testing.T) {
	reg := NewResolverRegistry()
	reg.Register(NewStaticResolver("person", nil))
	reg.Register(NewStaticResolver("taste", nil))
	lanes := reg.RegisteredLanes()
	if len(lanes) != 2 {
		t.Fatalf("expected 2 lanes, got %d", len(lanes))
	}
}
