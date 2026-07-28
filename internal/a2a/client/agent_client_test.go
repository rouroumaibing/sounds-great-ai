package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sounds-great-ai/pkg/a2a"
)

func TestGetAgentCard(t *testing.T) {
	card := a2a.AgentCard{Name: "AgentB", URL: "http://localhost:9002"}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			json.NewEncoder(w).Encode(card)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	c := NewAgentClient(ts.URL, "")
	got, err := c.GetAgentCard()
	if err != nil {
		t.Fatalf("GetAgentCard: %v", err)
	}
	if got.Name != "AgentB" {
		t.Errorf("Name = %q, want %q", got.Name, "AgentB")
	}
}

func TestSendTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req a2a.JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		task := req.Params.(map[string]interface{})
		resp := a2a.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: &a2a.Task{
				ID:     task["id"].(string),
				Status: a2a.TaskStatusCompleted,
				History: []a2a.Message{{ID: "m1", Role: "agent", SenderName: "AgentB",
					Parts: []a2a.Part{{Type: "text", Text: "Hello back"}}}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := NewAgentClient(ts.URL, "")
	task := a2a.Task{ID: "task-1", Status: a2a.TaskStatusSubmitted}
	got, err := c.SendTask(task)
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}
	if got.Status != a2a.TaskStatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, a2a.TaskStatusCompleted)
	}
}

func TestSendTaskSyncPollingFallback(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/a2a" {
			callCount++
			var req a2a.JSONRPCRequest
			json.NewDecoder(r.Body).Decode(&req)

			status := a2a.TaskStatusWorking
			if callCount >= 2 { // Second call (GetTask) returns completed
				status = a2a.TaskStatusCompleted
			}
			resp := a2a.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  &a2a.Task{ID: "task-1", Status: status},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/a2a/stream" {
			// SSE returns error to force polling fallback
			http.Error(w, "SSE not available", http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	c := NewAgentClient(ts.URL, "")
	c.client.Timeout = 5 * time.Second

	task := a2a.Task{ID: "task-1", Status: a2a.TaskStatusSubmitted}
	ctx := context.Background()
	result, err := c.SendTaskSync(ctx, task, 10*time.Second)
	if err != nil {
		t.Fatalf("SendTaskSync: %v", err)
	}
	if result.Status != a2a.TaskStatusCompleted {
		t.Errorf("Status = %q, want %q", result.Status, a2a.TaskStatusCompleted)
	}
}
