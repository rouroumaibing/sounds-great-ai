package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"sounds-great-ai/internal/aspect"
	"sounds-great-ai/pkg/a2a"
)

// TaskExecContext wraps a Task with its cancel function
type TaskExecContext struct {
	Task   *a2a.Task
	Cancel context.CancelFunc
}

// AgentServer is an A2A agent HTTP server
type AgentServer struct {
	card         a2a.AgentCard
	model        model.BaseChatModel
	systemPrompt string
	taskStore    map[string]*TaskExecContext
	taskTTL      time.Duration
	mu           sync.RWMutex
	listener     net.Listener
	commandGuard *aspect.CommandGuard
	done         chan struct{}
}

// NewAgentServer creates a new AgentServer
func NewAgentServer(card a2a.AgentCard, m model.BaseChatModel, systemPrompt string, guard *aspect.CommandGuard) *AgentServer {
	return &AgentServer{
		card:         card,
		model:        m,
		systemPrompt: systemPrompt,
		taskStore:    make(map[string]*TaskExecContext),
		taskTTL:      1 * time.Hour,
		commandGuard: guard,
		done:         make(chan struct{}),
	}
}

// HandleAgentCard returns the AgentCard JSON
func (s *AgentServer) HandleAgentCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.card)
}

// HandleA2A handles JSON-RPC requests
func (s *AgentServer) HandleA2A(w http.ResponseWriter, r *http.Request) {
	var req a2a.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, 0, -32700, "Parse error")
		return
	}

	switch req.Method {
	case a2a.MethodTasksSend:
		s.handleTasksSend(w, r, req)
	case a2a.MethodTasksGet:
		s.handleTasksGet(w, req)
	case a2a.MethodTasksCancel:
		s.handleTasksCancel(w, req)
	default:
		writeRPCError(w, req.ID, -32601, "Method not found")
	}
}

func writeRPCError(w http.ResponseWriter, id interface{}, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &a2a.RPCError{Code: code, Message: msg},
	})
}

func (s *AgentServer) handleTasksSend(w http.ResponseWriter, r *http.Request, req a2a.JSONRPCRequest) {
	paramsBytes, _ := json.Marshal(req.Params)
	var task a2a.Task
	if err := json.Unmarshal(paramsBytes, &task); err != nil {
		writeRPCError(w, req.ID, -32602, "Invalid params")
		return
	}

	// Security check
	if s.commandGuard != nil {
		for _, msg := range task.History {
			text := msg.ExtractText()
			result := s.commandGuard.GuardCommand(text)
			if result.Status == aspect.GuardStatusBlocked {
				task.Status = a2a.TaskStatusFailed
				writeRPCResult(w, req.ID, &task)
				return
			}
		}
	}

	// Build LLM messages
	llmMsgs := []*schema.Message{schema.SystemMessage(s.systemPrompt)}
	llmMsgs = append(llmMsgs, s.formatHistoryForLLM(task.History)...)

	// Create cancellable context
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	task.ID = uuid.NewString()
	task.Status = a2a.TaskStatusWorking

	// Store task
	s.mu.Lock()
	s.taskStore[task.ID] = &TaskExecContext{Task: &task, Cancel: cancel}
	s.mu.Unlock()

	// Call LLM
	resp, err := s.model.Generate(ctx, llmMsgs)
	if err != nil {
		s.mu.Lock()
		task.Status = a2a.TaskStatusFailed
		s.mu.Unlock()
		writeRPCResult(w, req.ID, &task)
		return
	}

	// Build reply message
	replyMsg := a2a.Message{
		ID:         uuid.NewString(),
		Role:       "agent",
		SenderName: s.card.Name,
		Parts:      []a2a.Part{{Type: "text", Text: resp.Content}},
	}

	s.mu.Lock()
	task.History = append(task.History, replyMsg)
	task.Status = a2a.TaskStatusCompleted
	s.mu.Unlock()

	writeRPCResult(w, req.ID, &task)
}

func writeRPCResult(w http.ResponseWriter, id interface{}, task *a2a.Task) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  task,
	})
}

// formatHistoryForLLM converts A2A History to LLM Messages from this agent's perspective
func (s *AgentServer) formatHistoryForLLM(history []a2a.Message) []*schema.Message {
	var llmMsgs []*schema.Message
	for _, msg := range history {
		if msg.Role == "system" {
			llmMsgs = append(llmMsgs, schema.SystemMessage(msg.ExtractText()))
			continue
		}
		if msg.Role == "agent" && msg.SenderName == s.card.Name {
			llmMsgs = append(llmMsgs, schema.AssistantMessage(msg.ExtractText(), nil))
		} else {
			llmMsgs = append(llmMsgs, schema.UserMessage(
				fmt.Sprintf("[%s]: %s", msg.SenderName, msg.ExtractText()),
			))
		}
	}
	return llmMsgs
}

func (s *AgentServer) handleTasksGet(w http.ResponseWriter, req a2a.JSONRPCRequest) {
	paramsBytes, _ := json.Marshal(req.Params)
	var params struct {
		ID string `json:"id"`
	}
	json.Unmarshal(paramsBytes, &params)

	s.mu.RLock()
	ctx, ok := s.taskStore[params.ID]
	s.mu.RUnlock()

	if !ok {
		writeRPCError(w, req.ID, -32602, "Task not found")
		return
	}
	writeRPCResult(w, req.ID, ctx.Task)
}

func (s *AgentServer) handleTasksCancel(w http.ResponseWriter, req a2a.JSONRPCRequest) {
	paramsBytes, _ := json.Marshal(req.Params)
	var params struct {
		ID string `json:"id"`
	}
	json.Unmarshal(paramsBytes, &params)

	s.mu.RLock()
	ctx, ok := s.taskStore[params.ID]
	s.mu.RUnlock()

	if !ok {
		writeRPCError(w, req.ID, -32602, "Task not found")
		return
	}

	ctx.Cancel() // Call outside lock to avoid deadlock

	// Snapshot task under lock before encoding
	s.mu.Lock()
	if ctx.Task.Status != a2a.TaskStatusWorking && ctx.Task.Status != a2a.TaskStatusSubmitted {
		// Task already reached terminal state, don't override
		taskCopy := *ctx.Task
		s.mu.Unlock()
		writeRPCResult(w, req.ID, &taskCopy)
		return
	}
	ctx.Task.Status = a2a.TaskStatusCanceled
	taskCopy := *ctx.Task
	s.mu.Unlock()

	writeRPCResult(w, req.ID, &taskCopy)
}

// HandleStream handles SSE streaming for a task
func (s *AgentServer) HandleStream(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		http.Error(w, "missing task_id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	for {
		s.mu.RLock()
		ctx, exists := s.taskStore[taskID]
		s.mu.RUnlock()

		if !exists {
			fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"task not found\"}\n\n")
			flusher.Flush()
			return
		}

		if ctx.Task.Status == a2a.TaskStatusCompleted || ctx.Task.Status == a2a.TaskStatusFailed || ctx.Task.Status == a2a.TaskStatusCanceled {
			taskJSON, _ := json.Marshal(ctx.Task)
			fmt.Fprintf(w, "data: {\"type\":\"final\",\"task\":%s}\n\n", taskJSON)
			flusher.Flush()
			return
		}

		fmt.Fprintf(w, "data: {\"type\":\"status\",\"status\":\"%s\"}\n\n", ctx.Task.Status)
		flusher.Flush()

		select {
		case <-r.Context().Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Start begins listening on the configured port
func (s *AgentServer) Start(port string) error {
	ln, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}
	s.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", s.HandleAgentCard)
	mux.HandleFunc("/a2a", s.HandleA2A)
	mux.HandleFunc("/a2a/stream", s.HandleStream)

	go http.Serve(ln, mux)
	go s.startTTLGC()
	return nil
}

// Stop closes the listener and stops background goroutines
func (s *AgentServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	close(s.done)
}

// startTTLGC periodically removes expired tasks
func (s *AgentServer) startTTLGC() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, ctx := range s.taskStore {
				if ctx.Task.Status == a2a.TaskStatusCompleted ||
					ctx.Task.Status == a2a.TaskStatusFailed ||
					ctx.Task.Status == a2a.TaskStatusCanceled {
					delete(s.taskStore, id)
				}
			}
			s.mu.Unlock()
		}
	}
}
