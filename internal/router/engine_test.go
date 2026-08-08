package router

import (
	"context"
	"testing"
	"sounds-great-ai/internal/config"
)

func TestRoutingEngine_NoRules(t *testing.T) {
	e := NewEngine(nil, nil)
	decision, err := e.Route(context.Background(), "do something")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if len(decision.Plan) != 1 {
		t.Fatalf("got %d steps, want 1", len(decision.Plan))
	}
	if decision.Reason == "" {
		t.Error("Reason is empty")
	}
}

func TestRoutingEngine_WithRoster(t *testing.T) {
	roster := map[string]*config.BreedConfig{
		"bianmu": {ID: "bianmu"},
	}
	e := NewEngine(nil, roster)
	decision, err := e.Route(context.Background(), "test task")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if decision == nil {
		t.Fatal("decision is nil")
	}
}
