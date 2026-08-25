package services

import (
	"context"
	"testing"

	agentsPorts "sounds-great-ai/internal/domains/agents/ports"
	"sounds-great-ai/internal/adapter/unified"
)

// fakeExecutor is a minimal unified.AgentExecutor for availability tests.
type fakeExecutor struct{ id string }

func (f *fakeExecutor) Execute(ctx context.Context, req unified.ExecuteRequest) (<-chan unified.StreamEvent, error) {
	return nil, nil
}
func (f *fakeExecutor) Capabilities() unified.AgentCapabilities { return unified.AgentCapabilities{} }
func (f *fakeExecutor) Health(ctx context.Context) error        { return nil }

func TestMemberAvailability_DisableImpact(t *testing.T) {
	av := NewMemberAvailability()
	av.SetEnabled("dog-x", false)
	if av.Enabled("dog-x") {
		t.Fatal("disabled member must report disabled")
	}
	if !av.Enabled("unknown-dog") {
		t.Fatal("unknown dog defaults to enabled")
	}
}

func TestAgentExecutor_DisabledFailsClosed(t *testing.T) {
	svc := NewAgentExecutor(map[string]unified.AgentExecutor{"dog-x": &fakeExecutor{id: "dog-x"}})
	av := NewMemberAvailability()
	av.SetEnabled("dog-x", false)
	svc.SetAvailabilityChecker(av)

	_, err := svc.Execute(context.Background(), agentsPorts.ExecuteRequest{ClientID: "dog-x"})
	if err == nil {
		t.Fatal("disabled member execution must fail closed")
	}
	cre, ok := err.(*CatRoutingError)
	if !ok {
		t.Fatalf("expected *CatRoutingError, got %T", err)
	}
	if cre.Reason != "member disabled" {
		t.Fatalf("wrong reason: %q", cre.Reason)
	}

	// Re-enabling lets the member execute.
	av.SetEnabled("dog-x", true)
	if _, err := svc.Execute(context.Background(), agentsPorts.ExecuteRequest{ClientID: "dog-x"}); err != nil {
		t.Fatalf("enabled member should pass availability gate: %v", err)
	}
}
