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

func TestHitlResponseEvent(t *testing.T) {
	ev := NewEvent(EventHitlResponse, "session-1", &HitlResponsePayload{
		RequestID: "req-456",
		Approved:  true,
		Reason:    "looks good",
	})
	if ev.Type != EventHitlResponse {
		t.Errorf("type = %s, want %s", ev.Type, EventHitlResponse)
	}
	var p HitlResponsePayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.RequestID != "req-456" || !p.Approved || p.Reason != "looks good" {
		t.Errorf("unexpected payload: %+v", p)
	}
}

func TestSystemNoticeEvent(t *testing.T) {
	ev := NewEvent(EventSystemNotice, "session-1", &SystemNoticePayload{
		Severity:  "warning",
		Title:     "High CPU",
		Message:   "CPU usage above 90%",
		Timestamp: "2026-01-01T00:00:00Z",
	})
	if ev.Type != EventSystemNotice {
		t.Errorf("type = %s, want %s", ev.Type, EventSystemNotice)
	}
	var p SystemNoticePayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Severity != "warning" || p.Title != "High CPU" {
		t.Errorf("unexpected payload: %+v", p)
	}
}

func TestNewEventNilPayload(t *testing.T) {
	ev := NewEvent(EventThinking, "s1", nil)
	if ev.Type != EventThinking {
		t.Errorf("type = %s, want %s", ev.Type, EventThinking)
	}
	if string(ev.Payload) != "null" {
		t.Errorf("payload = %q, want %q", ev.Payload, "null")
	}
}

func TestNewEventEmptySessionID(t *testing.T) {
	ev := NewEvent(EventThinking, "", &ThinkingPayload{Step: 1, Content: "test"})
	if ev.SessionID != "" {
		t.Errorf("SessionID = %q, want empty", ev.SessionID)
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SessionID != "" {
		t.Errorf("decoded SessionID = %q, want empty", decoded.SessionID)
	}
}

func TestNewEventLargePayload(t *testing.T) {
	largeContent := make([]byte, 10000)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	ev := NewEvent(EventThinking, "s1", &ThinkingPayload{
		Step:    1,
		Content: string(largeContent),
	})
	if len(ev.Payload) < 10000 {
		t.Errorf("payload too small: %d bytes", len(ev.Payload))
	}
	var p ThinkingPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Content) != 10000 {
		t.Errorf("content len = %d, want 10000", len(p.Content))
	}
}

func TestEventSeqOmitEmpty(t *testing.T) {
	ev := &Event{
		Type:      EventThinking,
		SessionID: "s1",
		Timestamp: 1000,
		Payload:   json.RawMessage(`{}`),
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Seq=0 should be omitted (omitempty)
	if string(data) != `{"type":"THINKING","session_id":"s1","timestamp":1000,"payload":{}}` {
		t.Errorf("unexpected JSON: %s", data)
	}

	ev.Seq = 42
	data, err = json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal with seq: %v", err)
	}
	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Seq != 42 {
		t.Errorf("Seq = %d, want 42", decoded.Seq)
	}
}

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		typ      EventType
		expected string
	}{
		{"EventThinking", EventThinking, "THINKING"},
		{"EventToolCall", EventToolCall, "TOOL_CALL"},
		{"EventCodeDiff", EventCodeDiff, "CODE_DIFF"},
		{"EventTerminalOutput", EventTerminalOutput, "TERMINAL_OUTPUT"},
		{"EventUserInput", EventUserInput, "USER_INPUT"},
		{"EventHITLApproval", EventHITLApproval, "HITL_APPROVAL"},
		{"EventHitlResponse", EventHitlResponse, "HITL_RESPONSE"},
		{"EventBarkStart", EventBarkStart, "BARK_START"},
		{"EventBarkResult", EventBarkResult, "BARK_RESULT"},
		{"EventBarkError", EventBarkError, "BARK_ERROR"},
		{"EventSystemNotice", EventSystemNotice, "SYSTEM_NOTICE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.typ) != tt.expected {
				t.Errorf("got %q, want %q", tt.typ, tt.expected)
			}
		})
	}
}

func TestEventUnmarshalDirect(t *testing.T) {
	raw := `{"type":"TOOL_CALL","session_id":"sess-42","timestamp":1700000000,"seq":7,"payload":{"tool":"grep","params":"foo","result":"bar","status":"ok"}}`
	var ev Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Type != EventToolCall {
		t.Errorf("type = %s, want %s", ev.Type, EventToolCall)
	}
	if ev.SessionID != "sess-42" {
		t.Errorf("session_id = %q", ev.SessionID)
	}
	if ev.Timestamp != 1700000000 {
		t.Errorf("timestamp = %d", ev.Timestamp)
	}
	if ev.Seq != 7 {
		t.Errorf("seq = %d", ev.Seq)
	}
	var p ToolCallPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.Tool != "grep" || p.Status != "ok" {
		t.Errorf("unexpected payload: %+v", p)
	}
}

func TestNewEventPreservesTypeAndSession(t *testing.T) {
	tests := []struct {
		name      string
		typ       EventType
		sessionID string
		payload   interface{}
	}{
		{"thinking", EventThinking, "s1", &ThinkingPayload{Step: 1, Content: "hi"}},
		{"tool_call", EventToolCall, "s2", &ToolCallPayload{Tool: "t"}},
		{"code_diff", EventCodeDiff, "s3", &CodeDiffPayload{File: "f.go"}},
		{"terminal", EventTerminalOutput, "s4", &TerminalOutputPayload{Stream: "stdout"}},
		{"user_input", EventUserInput, "s5", &UserInputPayload{Message: "m"}},
		{"hitl_approval", EventHITLApproval, "s6", &HITLApprovalPayload{RequestID: "r"}},
		{"hitl_response", EventHitlResponse, "s7", &HitlResponsePayload{RequestID: "r"}},
		{"bark_start", EventBarkStart, "s8", &BarkStartPayload{Breed: "b"}},
		{"bark_result", EventBarkResult, "s9", &BarkResultPayload{Breed: "b"}},
		{"bark_error", EventBarkError, "s10", &BarkErrorPayload{Breed: "b"}},
		{"system_notice", EventSystemNotice, "s11", &SystemNoticePayload{Title: "t"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := NewEvent(tt.typ, tt.sessionID, tt.payload)
			if ev.Type != tt.typ {
				t.Errorf("type = %s, want %s", ev.Type, tt.typ)
			}
			if ev.SessionID != tt.sessionID {
				t.Errorf("session_id = %q, want %q", ev.SessionID, tt.sessionID)
			}
			if ev.Timestamp == 0 {
				t.Error("timestamp should be non-zero")
			}
			if len(ev.Payload) == 0 {
				t.Error("payload should not be empty")
			}
		})
	}
}

func TestAllPayloadsJSONRoundTrip(t *testing.T) {
	t.Run("thinking", func(t *testing.T) {
		in := ThinkingPayload{Step: 5, Content: "step 5", Timestamp: 123}
		data, _ := json.Marshal(in)
		var out ThinkingPayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("tool_call", func(t *testing.T) {
		in := ToolCallPayload{Tool: "read", Params: `{}`, Result: "ok", Status: "success"}
		data, _ := json.Marshal(in)
		var out ToolCallPayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("code_diff", func(t *testing.T) {
		in := CodeDiffPayload{File: "main.go", Diff: "+line", Action: "add"}
		data, _ := json.Marshal(in)
		var out CodeDiffPayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("terminal_output", func(t *testing.T) {
		in := TerminalOutputPayload{Stream: "stderr", Data: "error\n"}
		data, _ := json.Marshal(in)
		var out TerminalOutputPayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("user_input", func(t *testing.T) {
		in := UserInputPayload{Message: "hello", SessionID: "s1"}
		data, _ := json.Marshal(in)
		var out UserInputPayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("hitl_approval", func(t *testing.T) {
		in := HITLApprovalPayload{Action: "write", Approved: true, RequestID: "r1", Impact: "low"}
		data, _ := json.Marshal(in)
		var out HITLApprovalPayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("hitl_response", func(t *testing.T) {
		in := HitlResponsePayload{RequestID: "r1", Approved: false, Reason: "no"}
		data, _ := json.Marshal(in)
		var out HitlResponsePayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("bark_start", func(t *testing.T) {
		in := BarkStartPayload{Breed: "bianmu", SessionID: "s1", Query: "q"}
		data, _ := json.Marshal(in)
		var out BarkStartPayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("bark_result", func(t *testing.T) {
		in := BarkResultPayload{
			Breed:   "xigou",
			Success: true,
			Steps:   map[string]StepResult{"s1": {Capability: "search", Approved: true, Reason: "ok"}},
		}
		data, _ := json.Marshal(in)
		var out BarkResultPayload
		json.Unmarshal(data, &out)
		if out.Breed != in.Breed || out.Success != in.Success || len(out.Steps) != 1 {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("bark_error", func(t *testing.T) {
		in := BarkErrorPayload{Breed: "demu", Error: "timeout"}
		data, _ := json.Marshal(in)
		var out BarkErrorPayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
	t.Run("system_notice", func(t *testing.T) {
		in := SystemNoticePayload{Severity: "critical", Title: "Down", Message: "service unavailable", Timestamp: "2026-01-01T00:00:00Z"}
		data, _ := json.Marshal(in)
		var out SystemNoticePayload
		json.Unmarshal(data, &out)
		if out != in {
			t.Errorf("mismatch: %+v vs %+v", out, in)
		}
	})
}
