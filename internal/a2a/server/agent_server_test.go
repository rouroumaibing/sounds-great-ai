package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"sounds-great-ai/pkg/a2a"
)

func TestAgentCardEndpoint(t *testing.T) {
	card := a2a.AgentCard{
		Name:                "AgentA",
		Description:         "Test Agent A",
		URL:                 "http://localhost:9001",
		SupportedInterfaces: []string{"tasks/send", "tasks/get", "tasks/cancel"},
	}
	server := NewAgentServer(card, nil, "test prompt", nil)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	w := httptest.NewRecorder()
	server.HandleAgentCard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got a2a.AgentCard
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Name != "AgentA" {
		t.Errorf("Name = %q, want %q", got.Name, "AgentA")
	}
}

func TestTasksSend(t *testing.T) {
	card := a2a.AgentCard{Name: "AgentA", URL: "http://localhost:9001"}
	mockModel := &mockChatModel{response: "Hello from AgentA"}
	server := NewAgentServer(card, mockModel, "You are AgentA", nil)

	task := a2a.Task{
		ID:     "task-1",
		Status: a2a.TaskStatusSubmitted,
		History: []a2a.Message{
			{ID: "msg-1", Role: "user", SenderName: "Orchestrator", Parts: []a2a.Part{{Type: "text", Text: "Say hello"}}},
		},
	}

	reqBody := a2a.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  a2a.MethodTasksSend,
		Params:  task,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleA2A(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp a2a.JSONRPCResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Result == nil {
		t.Fatal("Result is nil")
	}
	if resp.Result.Status != a2a.TaskStatusCompleted {
		t.Errorf("Status = %q, want %q", resp.Result.Status, a2a.TaskStatusCompleted)
	}
}

// mockChatModel implements model.BaseChatModel for testing
type mockChatModel struct {
	response string
}

func (m *mockChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(m.response, nil), nil
}

func (m *mockChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reader, writer := schema.Pipe[*schema.Message](1)
	go func() {
		writer.Send(schema.AssistantMessage(m.response, nil), nil)
		writer.Close()
	}()
	return reader, nil
}

func TestFormatHistoryForLLM(t *testing.T) {
	card := a2a.AgentCard{Name: "AgentA"}
	server := NewAgentServer(card, nil, "", nil)

	history := []a2a.Message{
		{ID: "m1", Role: "user", SenderName: "Orchestrator", Parts: []a2a.Part{{Type: "text", Text: "Hello"}}},
		{ID: "m2", Role: "agent", SenderName: "AgentA", Parts: []a2a.Part{{Type: "text", Text: "Hi there"}}},
		{ID: "m3", Role: "agent", SenderName: "AgentB", Parts: []a2a.Part{{Type: "text", Text: "I am B"}}},
	}

	msgs := server.formatHistoryForLLM(history)
	if len(msgs) != 3 {
		t.Fatalf("len = %d, want 3", len(msgs))
	}
	if msgs[0].Role != schema.User {
		t.Errorf("msg[0] Role = %q, want %q", msgs[0].Role, schema.User)
	}
	if msgs[1].Role != schema.Assistant {
		t.Errorf("msg[1] Role = %q, want %q", msgs[1].Role, schema.Assistant)
	}
	if msgs[2].Role != schema.User {
		t.Errorf("msg[2] Role = %q, want %q", msgs[2].Role, schema.User)
	}
	if !strings.Contains(msgs[2].Content, "[AgentB]") {
		t.Errorf("msg[2] Content should contain [AgentB], got %q", msgs[2].Content)
	}
}

func TestTasksCancel(t *testing.T) {
	card := a2a.AgentCard{Name: "AgentA"}
	mockModel := &slowMockModel{delay: 10 * time.Second}
	server := NewAgentServer(card, mockModel, "test", nil)

	// Start a task in a goroutine
	task := a2a.Task{
		Status:  a2a.TaskStatusSubmitted,
		History: []a2a.Message{{ID: "m1", Role: "user", Parts: []a2a.Part{{Type: "text", Text: "hi"}}}},
	}
	reqBody := a2a.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: a2a.MethodTasksSend, Params: task}
	body, _ := json.Marshal(reqBody)

	done := make(chan struct{})
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(body))
		w := httptest.NewRecorder()
		server.HandleA2A(w, req)
		close(done)
	}()

	// Wait for task to be stored
	var taskID string
	for i := 0; i < 100; i++ {
		server.mu.RLock()
		for id := range server.taskStore {
			taskID = id
		}
		server.mu.RUnlock()
		if taskID != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if taskID == "" {
		t.Fatal("task was never stored")
	}

	// Cancel it
	cancelReq := a2a.JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: a2a.MethodTasksCancel, Params: map[string]string{"id": taskID}}
	cancelBody, _ := json.Marshal(cancelReq)
	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(cancelBody))
	w := httptest.NewRecorder()
	server.HandleA2A(w, req)

	var resp a2a.JSONRPCResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Result.Status != a2a.TaskStatusCanceled {
		t.Errorf("Status = %q, want %q", resp.Result.Status, a2a.TaskStatusCanceled)
	}
}

type slowMockModel struct {
	delay time.Duration
}

func (m *slowMockModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	select {
	case <-time.After(m.delay):
		return schema.AssistantMessage("done", nil), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *slowMockModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, ctx.Err()
}

func TestSSEStream(t *testing.T) {
	card := a2a.AgentCard{Name: "AgentA"}
	server := NewAgentServer(card, nil, "", nil)

	// Manually store a completed task
	task := &a2a.Task{ID: "task-sse", Status: a2a.TaskStatusCompleted}
	server.mu.Lock()
	server.taskStore["task-sse"] = &TaskExecContext{Task: task, Cancel: func() {}}
	server.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/a2a/stream?task_id=task-sse", nil)
	w := httptest.NewRecorder()
	server.HandleStream(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if conn := w.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", conn)
	}
	if ac := w.Header().Get("Access-Control-Allow-Origin"); ac != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", ac)
	}
	if !strings.Contains(w.Body.String(), "final") {
		t.Errorf("response should contain 'final' event, got %q", w.Body.String())
	}
}

func TestTasksGet(t *testing.T) {
	card := a2a.AgentCard{Name: "AgentA"}
	server := NewAgentServer(card, nil, "", nil)

	// Store a task manually
	task := &a2a.Task{ID: "task-get", Status: a2a.TaskStatusCompleted}
	server.mu.Lock()
	server.taskStore["task-get"] = &TaskExecContext{Task: task, Cancel: func() {}}
	server.mu.Unlock()

	// Happy path
	reqBody := a2a.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: a2a.MethodTasksGet, Params: map[string]string{"id": "task-get"}}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleA2A(w, req)

	var resp a2a.JSONRPCResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Result == nil || resp.Result.ID != "task-get" {
		t.Errorf("expected task-get, got %+v", resp.Result)
	}

	// Not found path
	reqBody2 := a2a.JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: a2a.MethodTasksGet, Params: map[string]string{"id": "nonexistent"}}
	body2, _ := json.Marshal(reqBody2)
	req2 := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	server.HandleA2A(w2, req2)

	var resp2 a2a.JSONRPCResponse
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Error == nil {
		t.Error("expected error for nonexistent task")
	}
}
