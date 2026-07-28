package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"sounds-great-ai/internal/a2a/client"
	"sounds-great-ai/pkg/a2a"
)

func TestWorklistLoopDetection(t *testing.T) {
	wl := NewWorklist()

	// Normal adds should succeed
	err := wl.Add(WorklistEntry{ToAgent: "AgentA", VisitedChain: []string{}})
	if err != nil {
		t.Fatalf("first add: %v", err)
	}
	err = wl.Add(WorklistEntry{ToAgent: "AgentB", VisitedChain: []string{"AgentA"}})
	if err != nil {
		t.Fatalf("second add: %v", err)
	}

	// Loop: AgentA visited 2 times already -> should fail
	err = wl.Add(WorklistEntry{ToAgent: "AgentA", VisitedChain: []string{"AgentA", "AgentB", "AgentA"}})
	if err == nil {
		t.Error("expected loop detection error, got nil")
	}
}

func TestParseMentions(t *testing.T) {
	mentions := parseMentions("Hello @AgentB please help")
	if len(mentions) != 1 || mentions[0] != "AgentB" {
		t.Errorf("mentions = %v, want [AgentB]", mentions)
	}

	mentions = parseMentions("no mentions here")
	if len(mentions) != 0 {
		t.Errorf("mentions = %v, want empty", mentions)
	}
}

func TestOrchestratorRun(t *testing.T) {
	// Create mock HTTP servers for AgentA and AgentB
	agentA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req a2a.JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		paramsBytes, _ := json.Marshal(req.Params)
		var task a2a.Task
		json.Unmarshal(paramsBytes, &task)

		replyText := "Hello from AgentA"
		if len(task.History) > 0 {
			lastMsg := task.History[len(task.History)-1]
			if strings.Contains(lastMsg.ExtractText(), "几点") {
				replyText = "现在是 2026-07-30 15:30:45"
			} else {
				replyText = "我是 glm-4.7-flash-free"
			}
		}

		task.Status = a2a.TaskStatusCompleted
		task.History = append(task.History, a2a.Message{
			ID:         uuid.NewString(),
			Role:       "agent",
			SenderName: "AgentA",
			Parts:      []a2a.Part{{Type: "text", Text: replyText}},
		})
		json.NewEncoder(w).Encode(a2a.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: &task})
	}))
	defer agentA.Close()

	agentB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req a2a.JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		paramsBytes, _ := json.Marshal(req.Params)
		var task a2a.Task
		json.Unmarshal(paramsBytes, &task)

		task.Status = a2a.TaskStatusCompleted
		task.History = append(task.History, a2a.Message{
			ID:         uuid.NewString(),
			Role:       "agent",
			SenderName: "AgentB",
			Parts:      []a2a.Part{{Type: "text", Text: "你好！我是 glm-5.2。请问你是什么大模型？"}},
		})
		json.NewEncoder(w).Encode(a2a.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: &task})
	}))
	defer agentB.Close()

	clients := map[string]*client.AgentClient{
		"AgentA": client.NewAgentClient(agentA.URL, ""),
		"AgentB": client.NewAgentClient(agentB.URL, ""),
	}
	urls := map[string]string{"AgentA": agentA.URL, "AgentB": agentB.URL}

	script := &Script{
		Turns: []Turn{
			{ToAgent: "AgentA", Prompt: "向 Agent B 问好"},
			{FromAgent: "AgentA", ToAgent: "AgentB"},
			{FromAgent: "AgentB", ToAgent: "AgentA"},
			{FromAgent: "AgentA", ToAgent: "AgentB"},
		},
	}

	orch := NewOrchestrator(nil, clients, urls, script)
	err := orch.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}
