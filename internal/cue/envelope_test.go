package cue

import (
	"testing"
	"time"
)

func TestEnvelopeIsExpired(t *testing.T) {
	env := &CueEnvelope{
		Expiry: time.Now().Add(-1 * time.Minute).UnixMilli(),
	}
	if !env.IsExpired() {
		t.Error("envelope should be expired")
	}

	env2 := &CueEnvelope{
		Expiry: time.Now().Add(1 * time.Minute).UnixMilli(),
	}
	if env2.IsExpired() {
		t.Error("envelope should not be expired")
	}

	env3 := &CueEnvelope{
		Expiry: 0, // no expiry
	}
	if env3.IsExpired() {
		t.Error("envelope with no expiry should not be expired")
	}
}

func TestBudgetForLane(t *testing.T) {
	if BudgetForLane("person") != 300 {
		t.Errorf("person budget = %d, want 300", BudgetForLane("person"))
	}
	if BudgetForLane("taste") != 300 {
		t.Errorf("taste budget = %d, want 300", BudgetForLane("taste"))
	}
	if BudgetForLane("entity") != 420 {
		t.Errorf("entity budget = %d, want 420", BudgetForLane("entity"))
	}
	if BudgetForLane("decision") != 420 {
		t.Errorf("decision budget = %d, want 420", BudgetForLane("decision"))
	}
	if BudgetForLane("unknown") != 300 {
		t.Errorf("unknown lane budget = %d, want 300 (default)", BudgetForLane("unknown"))
	}
}

func TestEnvelopeBuilderDedup(t *testing.T) {
	b := NewEnvelopeBuilder()
	e1 := b.Build("id-1", "person", "why", "summary", "src", "drill", ScopeSession, 0, "")
	if e1 == nil {
		t.Fatal("first build should succeed")
	}
	e2 := b.Build("id-1", "person", "why", "summary", "src", "drill", ScopeSession, 0, "")
	if e2 != nil {
		t.Error("duplicate build should return nil")
	}
	e3 := b.Build("id-2", "person", "why", "summary", "src", "drill", ScopeSession, 0, "")
	if e3 == nil {
		t.Fatal("different ID should succeed")
	}
	if b.SeenCount() != 2 {
		t.Errorf("seen count = %d, want 2", b.SeenCount())
	}
}

func TestEnvelopeBuilderReset(t *testing.T) {
	b := NewEnvelopeBuilder()
	b.Build("id-1", "person", "why", "summary", "src", "drill", ScopeSession, 0, "")
	b.Reset()
	if b.SeenCount() != 0 {
		t.Error("after reset, seen count should be 0")
	}
}

func TestEnvelopeBuilderSetsBudget(t *testing.T) {
	b := NewEnvelopeBuilder()
	e := b.Build("id-1", "person", "why", "summary", "src", "drill", ScopeSession, 0, "")
	if e.TokenBudget != 300 {
		t.Errorf("expected budget 300, got %d", e.TokenBudget)
	}
}
