package services

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/adapter/unified"
	agentsPorts "sounds-great-ai/internal/domains/agents/ports"
)

func newMockMap() map[string]unified.AgentExecutor {
	return map[string]unified.AgentExecutor{
		"claude":   &mockAdapter{name: "claude"},
		"codex":    &mockAdapter{name: "codex"},
		"gemini":   &mockAdapter{name: "gemini"},
		"opencode": &mockAdapter{name: "opencode"},
	}
}

type mockAdapter struct {
	name  string
	cap   unified.AgentCapabilities
	execd bool
}

func (m *mockAdapter) Execute(ctx context.Context, req unified.ExecuteRequest) (<-chan unified.StreamEvent, error) {
	m.execd = true
	out := make(chan unified.StreamEvent, 1)
	out <- unified.StreamEvent{Type: "text", Content: "hi from " + m.name}
	close(out)
	return out, nil
}

func (m *mockAdapter) Capabilities() unified.AgentCapabilities { return m.cap }
func (m *mockAdapter) Health(ctx context.Context) error        { return nil }

func TestAgentExecutorServiceCount(t *testing.T) {
	svc := NewAgentExecutor(newMockMap())
	if svc.Count() != 4 {
		t.Fatalf("count = %d, want 4", svc.Count())
	}
}

func TestAgentExecutorServiceRoutesByClientID(t *testing.T) {
	mocks := newMockMap()
	svc := NewAgentExecutor(mocks)

	req := agentsPorts.ExecuteRequest{
		ClientID: "codex",
		Messages: []*schema.Message{schema.UserMessage("hi")},
		Model:    "gpt-4",
	}
	ch, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	ev, ok := <-ch
	if !ok {
		t.Fatal("expected a stream event")
	}
	if ev.Content != "hi from codex" {
		t.Fatalf("content = %q, want codex", ev.Content)
	}
	if !mocks["codex"].(*mockAdapter).execd {
		t.Fatal("codex adapter should have executed")
	}
	if mocks["claude"].(*mockAdapter).execd {
		t.Fatal("claude adapter should NOT have executed")
	}
}

func TestAgentExecutorServiceUnknownClientID(t *testing.T) {
	svc := NewAgentExecutor(newMockMap())
	_, err := svc.Execute(context.Background(), agentsPorts.ExecuteRequest{ClientID: "nope"})
	if err == nil {
		t.Fatal("expected error for unknown CLI")
	}
	if _, err := svc.Get("nope"); err == nil {
		t.Fatal("Get: expected error for unknown CLI")
	}
}

func TestAgentExecutorServiceDelegatesCapabilitiesAndHealth(t *testing.T) {
	svc := NewAgentExecutor(newMockMap())
	cap := svc.Capabilities("gemini")
	_ = cap // just ensure no panic / no error path
	if err := svc.Health(context.Background(), "gemini"); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if err := svc.Health(context.Background(), "unknown"); err == nil {
		t.Fatal("Health: expected error for unknown CLI")
	}
}

// Ensure the service satisfies the port.
var _ agentsPorts.IAgentExecutor = (*AgentExecutorService)(nil)
