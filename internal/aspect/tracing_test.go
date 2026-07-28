package aspect

import (
	"context"
	"sync"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/tool"
	"sounds-great-ai/pkg/protocol"
)

func TestTracingCallbackOnStartModel(t *testing.T) {
	var mu sync.Mutex
	var events []*protocol.Event

	cb := NewTracingCallback("session-1", func(ctx context.Context, ev *protocol.Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	// Simulate model OnStart
	info := &callbacks.RunInfo{
		Name:      "test-model",
		Type:      "OpenAI",
		Component: components.ComponentOfChatModel,
	}
	input := &tool.CallbackInput{
		ArgumentsInJSON: "",
	}

	ctx := cb.OnStart(context.Background(), info, input)

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 {
		t.Fatal("expected at least one event from model OnStart")
	}
	if events[0].Type != protocol.EventThinking {
		t.Errorf("expected THINKING event, got %s", events[0].Type)
	}
	_ = ctx
}

func TestTracingCallbackOnStartTool(t *testing.T) {
	var mu sync.Mutex
	var events []*protocol.Event

	cb := NewTracingCallback("session-1", func(ctx context.Context, ev *protocol.Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	info := &callbacks.RunInfo{
		Name:      "read_file",
		Type:      "ReadFile",
		Component: components.ComponentOfTool,
	}
	input := &tool.CallbackInput{
		ArgumentsInJSON: `{"path":"test.go"}`,
	}

	ctx := cb.OnStart(context.Background(), info, input)

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 {
		t.Fatal("expected at least one event from tool OnStart")
	}
	if events[0].Type != protocol.EventToolCall {
		t.Errorf("expected TOOL_CALL event, got %s", events[0].Type)
	}
	_ = ctx
}

func TestTracingCallbackOnEndTool(t *testing.T) {
	var mu sync.Mutex
	var events []*protocol.Event

	cb := NewTracingCallback("session-1", func(ctx context.Context, ev *protocol.Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	info := &callbacks.RunInfo{
		Name:      "read_file",
		Type:      "ReadFile",
		Component: components.ComponentOfTool,
	}
	output := &tool.CallbackOutput{
		Response: "file contents",
	}

	ctx := cb.OnEnd(context.Background(), info, output)

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 {
		t.Fatal("expected at least one event from tool OnEnd")
	}
	if events[0].Type != protocol.EventToolCall {
		t.Errorf("expected TOOL_CALL event, got %s", events[0].Type)
	}
	_ = ctx
}
