package unified

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// mockExecutor is a minimal AgentExecutor for testing the interface contract.
type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, req ExecuteRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 1)
	ch <- StreamEvent{Type: "done"}
	close(ch)
	return ch, nil
}

func (m *mockExecutor) Capabilities() AgentCapabilities {
	return AgentCapabilities{SupportsMCP: true, SupportsTools: true, SupportsFileOps: true}
}

func (m *mockExecutor) Health(ctx context.Context) error {
	return nil
}

func TestAgentExecutorInterface(t *testing.T) {
	var exec AgentExecutor = &mockExecutor{}
	ch, err := exec.Execute(context.Background(), ExecuteRequest{
		Messages:     []*schema.Message{{Role: "user", Content: "hi"}},
		SystemPrompt: "test",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}
	if len(events) != 1 || !events[0].IsDone() {
		t.Fatalf("expected 1 done event, got %+v", events)
	}
	caps := exec.Capabilities()
	if !caps.SupportsMCP {
		t.Fatal("expected SupportsMCP=true")
	}
	if err := exec.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestExecuteRequestDefaults(t *testing.T) {
	req := ExecuteRequest{}
	if req.MaxTokens != 0 {
		t.Fatalf("default MaxTokens should be 0, got %d", req.MaxTokens)
	}
}
