package cue

import (
	"context"
	"strings"
	"testing"
)

func TestCuePlaneInject(t *testing.T) {
	catalog := NewCatalog()
	resolvers := NewResolverRegistry()
	resolvers.Register(NewStaticResolver("entity", []StaticEntry{
		{ID: "e1", Content: "UserService handles auth"},
	}))
	resolvers.Register(NewStaticResolver("decision", []StaticEntry{
		{ID: "d1", Content: "Use SQLite for local dev"},
	}))
	builder := NewEnvelopeBuilder()
	ledger := NewConsumptionLedger()
	plane := NewCuePlane(catalog, resolvers, builder, ledger)

	input := CatalogInput{
		SubjectMentions:       []string{"UserService"},
		KnownSubjects:         []string{"UserService"},
		DeliveryDecisionPending: true,
	}
	result, err := plane.Inject(context.Background(), input, "session-1")
	if err != nil {
		t.Fatalf("inject error: %v", err)
	}
	if result.Injected == 0 {
		t.Fatal("expected at least 1 injected cue")
	}
	if result.Prompt == "" {
		t.Error("expected non-empty prompt")
	}
	if !strings.Contains(result.Prompt, "<cue-plane>") {
		t.Error("prompt should contain <cue-plane> tag")
	}
}

func TestCuePlaneInjectEmpty(t *testing.T) {
	catalog := NewCatalog()
	resolvers := NewResolverRegistry()
	builder := NewEnvelopeBuilder()
	ledger := NewConsumptionLedger()
	plane := NewCuePlane(catalog, resolvers, builder, ledger)

	input := CatalogInput{}
	result, err := plane.Inject(context.Background(), input, "session-1")
	if err != nil {
		t.Fatalf("inject error: %v", err)
	}
	if result.Injected != 0 {
		t.Errorf("expected 0 injected, got %d", result.Injected)
	}
	if result.Prompt != "" {
		t.Error("expected empty prompt for no cues")
	}
}

func TestCuePlaneSourceInvalidationFailClosed(t *testing.T) {
	catalog := NewCatalog()
	resolvers := NewResolverRegistry()
	resolvers.Register(NewStaticResolver("entity", []StaticEntry{
		{ID: "e1", Content: "UserService handles auth"},
	}))
	builder := NewEnvelopeBuilder()
	ledger := NewConsumptionLedger()
	plane := NewCuePlane(catalog, resolvers, builder, ledger)

	// Invalidate the source before injection
	plane.InvalidateSource("e1", "corrected")

	input := CatalogInput{
		SubjectMentions: []string{"UserService"},
		KnownSubjects:   []string{"UserService"},
	}
	result, err := plane.Inject(context.Background(), input, "session-1")
	if err != nil {
		t.Fatalf("inject error: %v", err)
	}
	if result.Injected != 0 {
		t.Errorf("expected 0 injected (fail-closed), got %d", result.Injected)
	}
	if result.Skipped == 0 {
		t.Error("expected at least 1 skipped due to invalidation")
	}
}

func TestCuePlaneRecordAgentAction(t *testing.T) {
	catalog := NewCatalog()
	resolvers := NewResolverRegistry()
	builder := NewEnvelopeBuilder()
	ledger := NewConsumptionLedger()
	plane := NewCuePlane(catalog, resolvers, builder, ledger)

	plane.RecordAgentAction("env-1", "person", "session-1", EpisodeApplied)
	plane.RecordAgentAction("env-1", "person", "session-1", EpisodeDismissed)
	if ledger.EpisodeCount() != 2 {
		t.Errorf("expected 2 episodes, got %d", ledger.EpisodeCount())
	}
}

func TestCuePlaneReset(t *testing.T) {
	catalog := NewCatalog()
	resolvers := NewResolverRegistry()
	resolvers.Register(NewStaticResolver("entity", []StaticEntry{
		{ID: "e1", Content: "UserService"},
	}))
	builder := NewEnvelopeBuilder()
	ledger := NewConsumptionLedger()
	plane := NewCuePlane(catalog, resolvers, builder, ledger)

	input := CatalogInput{
		SubjectMentions: []string{"UserService"},
		KnownSubjects:   []string{"UserService"},
	}
	// First injection
	r1, _ := plane.Inject(context.Background(), input, "session-1")
	if r1.Injected == 0 {
		t.Fatal("first injection should have cues")
	}
	// Reset for new invocation
	plane.Reset()
	// Second injection after reset should work again
	r2, _ := plane.Inject(context.Background(), input, "session-1")
	if r2.Injected == 0 {
		t.Error("second injection after reset should have cues")
	}
}

func TestCuePlaneAssemblePrompt(t *testing.T) {
	catalog := NewCatalog()
	resolvers := NewResolverRegistry()
	builder := NewEnvelopeBuilder()
	ledger := NewConsumptionLedger()
	plane := NewCuePlane(catalog, resolvers, builder, ledger)

	envs := []*CueEnvelope{
		{ID: "e1", Lane: "person", WhyNow: "subject seen", Summary: "User is Alice", DrillHandle: "drill:person:e1"},
	}
	prompt := plane.AssemblePrompt(envs)
	if !strings.Contains(prompt, "Alice") {
		t.Error("prompt should contain content")
	}
	if !strings.Contains(prompt, "drill:person:e1") {
		t.Error("prompt should contain drill handle")
	}
}
