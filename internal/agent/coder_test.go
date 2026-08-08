package agent

import (
	"context"
	"encoding/json"
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

func TestCoderAgentRunEmptyInput(t *testing.T) {
	agent := &CoderAgent{
		sessionID: "test-session",
	}
	ctx := context.Background()
	eventCh, err := agent.Run(ctx, "")
	if err != nil {
		t.Fatalf("Run with empty input failed: %v", err)
	}
	var events []*protocol.Event
	for ev := range eventCh {
		events = append(events, ev)
	}
	if len(events) == 0 {
		t.Error("expected at least one event even with empty input")
	}
}

func TestCoderAgentRunReturnsNilError(t *testing.T) {
	agent := &CoderAgent{
		sessionID: "test-session",
	}
	ctx := context.Background()
	_, err := agent.Run(ctx, "input")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestCoderAgentRunMultipleThinkingEvents(t *testing.T) {
	agent := &CoderAgent{
		sessionID: "test-session",
	}
	ctx := context.Background()
	eventCh, _ := agent.Run(ctx, "input")

	thinkingCount := 0
	for ev := range eventCh {
		if ev.Type == protocol.EventThinking {
			thinkingCount++
		}
	}
	if thinkingCount < 2 {
		t.Errorf("expected at least 2 THINKING events, got %d", thinkingCount)
	}
}

func TestCoderAgentRunNoModelProducesNoModelEvent(t *testing.T) {
	agent := &CoderAgent{
		sessionID: "test-session",
	}
	ctx := context.Background()
	eventCh, _ := agent.Run(ctx, "input")

	foundNoModel := false
	for ev := range eventCh {
		if ev.Type == protocol.EventThinking {
			var tp protocol.ThinkingPayload
			if json.Unmarshal(ev.Payload, &tp) == nil {
				if tp.Content == "No model configured, returning echo response." {
					foundNoModel = true
				}
			}
		}
	}
	if !foundNoModel {
		t.Error("expected no-model THINKING event when model is nil")
	}
}

func TestNewCoderAgent(t *testing.T) {
	agent := NewCoderAgent("session-1", nil, nil)
	if agent == nil {
		t.Fatal("expected non-nil agent")
	}
	if agent.sessionID != "session-1" {
		t.Errorf("expected sessionID session-1, got %s", agent.sessionID)
	}
	if agent.maxFailures != 3 {
		t.Errorf("expected default maxFailures 3, got %d", agent.maxFailures)
	}
}

func TestCoderAgentSetMaxFailures(t *testing.T) {
	agent := &CoderAgent{
		sessionID:   "test-session",
		maxFailures: 3,
	}
	agent.SetMaxFailures(10)
	if agent.maxFailures != 10 {
		t.Errorf("expected maxFailures 10, got %d", agent.maxFailures)
	}
	agent.SetMaxFailures(0)
	if agent.maxFailures != 0 {
		t.Errorf("expected maxFailures 0, got %d", agent.maxFailures)
	}
}

func TestCoderAgentRunEventsHaveSessionID(t *testing.T) {
	agent := &CoderAgent{
		sessionID: "my-session",
	}
	ctx := context.Background()
	eventCh, _ := agent.Run(ctx, "input")

	for ev := range eventCh {
		if ev.SessionID != "my-session" {
			t.Errorf("expected session ID my-session, got %s", ev.SessionID)
		}
	}
}
