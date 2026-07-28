package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name  string
		event *Event
	}{
		{
			name: "thinking event",
			event: NewEvent(EventThinking, "session-1", &ThinkingPayload{
				Step:      1,
				Content:   "Analyzing the request",
				Timestamp: time.Now().Unix(),
			}),
		},
		{
			name: "tool_call event",
			event: NewEvent(EventToolCall, "session-1", &ToolCallPayload{
				Tool:   "read_file",
				Params: `{"path":"main.go"}`,
				Result: "file contents here",
				Status: "success",
			}),
		},
		{
			name: "code_diff event",
			event: NewEvent(EventCodeDiff, "session-1", &CodeDiffPayload{
				File:   "main.go",
				Diff:   "--- a\n+++ b\n+hello",
				Action: "edit",
			}),
		},
		{
			name: "terminal_output event",
			event: NewEvent(EventTerminalOutput, "session-1", &TerminalOutputPayload{
				Stream: "stdout",
				Data:   "hello world\n",
			}),
		},
		{
			name: "user_input event",
			event: NewEvent(EventUserInput, "session-1", &UserInputPayload{
				Message:   "write a function",
				SessionID: "session-1",
			}),
		},
		{
			name: "hitl_approval event",
			event: NewEvent(EventHITLApproval, "session-1", &HITLApprovalPayload{
				Action:    "write .env",
				Approved:  false,
				RequestID: "req-123",
				Impact:    "Modifies environment configuration",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			var decoded Event
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			if decoded.Type != tt.event.Type {
				t.Errorf("type mismatch: got %s, want %s", decoded.Type, tt.event.Type)
			}
			if decoded.SessionID != tt.event.SessionID {
				t.Errorf("session_id mismatch: got %s, want %s", decoded.SessionID, tt.event.SessionID)
			}
			if decoded.Timestamp != tt.event.Timestamp {
				t.Errorf("timestamp mismatch: got %d, want %d", decoded.Timestamp, tt.event.Timestamp)
			}

			var originalPayload, decodedPayload map[string]interface{}
			if err := json.Unmarshal(tt.event.Payload, &originalPayload); err != nil {
				t.Fatalf("unmarshal original payload failed: %v", err)
			}
			if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
				t.Fatalf("unmarshal decoded payload failed: %v", err)
			}
			if len(originalPayload) != len(decodedPayload) {
				t.Errorf("payload field count mismatch: got %d, want %d", len(decodedPayload), len(originalPayload))
			}
		})
	}
}

func TestNewEventSetsTimestamp(t *testing.T) {
	before := time.Now().Unix()
	ev := NewEvent(EventThinking, "s1", &ThinkingPayload{Step: 1, Content: "test"})
	after := time.Now().Unix()
	if ev.Timestamp < before || ev.Timestamp > after {
		t.Errorf("timestamp %d not in range [%d, %d]", ev.Timestamp, before, after)
	}
}

func TestBarkStartEvent(t *testing.T) {
	ev := NewEvent(EventBarkStart, "session-1", &BarkStartPayload{
		Breed:     "zhonghuatianyuanquan",
		SessionID: "session-1",
		Query:     "check this command",
	})
	if ev.Type != EventBarkStart {
		t.Errorf("type = %s, want %s", ev.Type, EventBarkStart)
	}
	var p BarkStartPayload
	json.Unmarshal(ev.Payload, &p)
	if p.Breed != "zhonghuatianyuanquan" {
		t.Errorf("breed = %q", p.Breed)
	}
}

func TestBarkResultEvent(t *testing.T) {
	steps := map[string]StepResult{
		"s1": {Capability: "command_check", Approved: true, Reason: "ok"},
	}
	ev := NewEvent(EventBarkResult, "session-1", &BarkResultPayload{
		Breed:   "zhonghuatianyuanquan",
		Success: true,
		Steps:   steps,
	})
	var p BarkResultPayload
	json.Unmarshal(ev.Payload, &p)
	if !p.Success || len(p.Steps) != 1 {
		t.Errorf("unexpected payload: %+v", p)
	}
}

func TestBarkErrorEvent(t *testing.T) {
	ev := NewEvent(EventBarkError, "session-1", &BarkErrorPayload{
		Breed: "bianmu",
		Error: "capability not registered",
	})
	var p BarkErrorPayload
	json.Unmarshal(ev.Payload, &p)
	if p.Error != "capability not registered" {
		t.Errorf("error = %q", p.Error)
	}
}
