package agent

import (
	"context"
	"testing"

	"sounds-great-ai/pkg/protocol"
)

func TestCoderAgentRun(t *testing.T) {
	// 创建一个简单的 CoderAgent
	agent := &CoderAgent{
		sessionID: "test-session",
	}

	ctx := context.Background()
	eventCh, err := agent.Run(ctx, "hello")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var events []*protocol.Event
	for ev := range eventCh {
		events = append(events, ev)
	}

	if len(events) == 0 {
		t.Error("expected at least one event from agent")
	}
}

func TestCoderAgentRunProducesThinkingEvent(t *testing.T) {
	agent := &CoderAgent{
		sessionID: "test-session",
	}

	ctx := context.Background()
	eventCh, _ := agent.Run(ctx, "test input")

	foundThinking := false
	for ev := range eventCh {
		if ev.Type == protocol.EventThinking {
			foundThinking = true
		}
	}

	if !foundThinking {
		t.Error("expected at least one THINKING event")
	}
}
