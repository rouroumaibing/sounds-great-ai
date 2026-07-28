package a2a

import (
	"encoding/json"
	"testing"
)

func TestTaskStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   TaskStatus
		expected string
	}{
		{"submitted", TaskStatusSubmitted, "submitted"},
		{"working", TaskStatusWorking, "working"},
		{"completed", TaskStatusCompleted, "completed"},
		{"failed", TaskStatusFailed, "failed"},
		{"input-required", TaskStatusInputRequired, "input-required"},
		{"canceled", TaskStatusCanceled, "canceled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("got %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestMessageExtractText(t *testing.T) {
	msg := Message{
		ID:         "msg-1",
		Role:       "agent",
		SenderName: "AgentA",
		Parts: []Part{
			{Type: "text", Text: "Hello "},
			{Type: "text", Text: "World"},
		},
	}
	got := msg.ExtractText()
	want := "Hello World"
	if got != want {
		t.Errorf("ExtractText() = %q, want %q", got, want)
	}
}

func TestTaskJSONRoundTrip(t *testing.T) {
	task := Task{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status:    TaskStatusWorking,
		History: []Message{
			{
				ID:         "msg-1",
				Role:       "user",
				SenderName: "AgentA",
				Parts:      []Part{{Type: "text", Text: "Hello"}},
			},
		},
		Trace: &TraceInfo{
			TraceID:      "trace-1",
			SpanID:       "span-1",
			ParentSpanID: "span-0",
		},
	}
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.ID != task.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, task.ID)
	}
	if decoded.Trace == nil || decoded.Trace.ParentSpanID != "span-0" {
		t.Errorf("Trace.ParentSpanID not preserved")
	}
	if len(decoded.History) != 1 || decoded.History[0].SenderName != "AgentA" {
		t.Errorf("History not preserved")
	}
}

func TestPartToolCallSerialization(t *testing.T) {
	part := Part{
		Type:     "tool_call",
		ToolCall: &ToolCall{ID: "tc-1", Name: "get_time", Arguments: `{}`},
	}
	data, err := json.Marshal(part)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Part
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.ToolCall == nil || decoded.ToolCall.Name != "get_time" {
		t.Errorf("ToolCall not preserved")
	}
}

func TestMethodConstants(t *testing.T) {
	if MethodTasksSend != "tasks/send" {
		t.Errorf("MethodTasksSend = %q", MethodTasksSend)
	}
	if MethodTasksGet != "tasks/get" {
		t.Errorf("MethodTasksGet = %q", MethodTasksGet)
	}
	if MethodTasksCancel != "tasks/cancel" {
		t.Errorf("MethodTasksCancel = %q", MethodTasksCancel)
	}
}
