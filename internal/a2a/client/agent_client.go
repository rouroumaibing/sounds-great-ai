package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sounds-great-ai/pkg/a2a"
)

// AgentClient is an A2A HTTP client
type AgentClient struct {
	baseURL string
	client  *http.Client
}

// NewAgentClient creates a new AgentClient
func NewAgentClient(baseURL string, apiKey string) *AgentClient {
	return &AgentClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// GetAgentCard fetches the agent card
func (c *AgentClient) GetAgentCard() (*a2a.AgentCard, error) {
	resp, err := c.client.Get(c.baseURL + "/.well-known/agent-card.json")
	if err != nil {
		return nil, fmt.Errorf("GetAgentCard: %w", err)
	}
	defer resp.Body.Close()
	var card a2a.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decode agent card: %w", err)
	}
	return &card, nil
}

// SendTask sends a task via JSON-RPC tasks/send
func (c *AgentClient) SendTask(task a2a.Task) (*a2a.Task, error) {
	req := a2a.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  a2a.MethodTasksSend,
		Params:  task,
	}
	return c.doJSONRPC(req)
}

// GetTask queries a task via JSON-RPC tasks/get
func (c *AgentClient) GetTask(taskID string) (*a2a.Task, error) {
	req := a2a.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  a2a.MethodTasksGet,
		Params:  map[string]string{"id": taskID},
	}
	return c.doJSONRPC(req)
}

// CancelTask cancels a task via JSON-RPC tasks/cancel
func (c *AgentClient) CancelTask(taskID string) (*a2a.Task, error) {
	req := a2a.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  a2a.MethodTasksCancel,
		Params:  map[string]string{"id": taskID},
	}
	return c.doJSONRPC(req)
}

func (c *AgentClient) doJSONRPC(req a2a.JSONRPCRequest) (*a2a.Task, error) {
	body, _ := json.Marshal(req)
	resp, err := c.client.Post(c.baseURL+"/a2a", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("doJSONRPC: %w", err)
	}
	defer resp.Body.Close()
	var rpcResp a2a.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// StreamTask connects to the SSE stream for a task and calls onEvent for each
// "data:" line. It returns nil when the stream ends normally.
func (c *AgentClient) StreamTask(ctx context.Context, taskID string, onEvent func(eventType string, data string)) error {
	url := fmt.Sprintf("%s/a2a/stream?task_id=%s", c.baseURL, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("StreamTask: %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			onEvent("data", data)
		}
	}
	return scanner.Err()
}

// SendTaskSync sends a task and waits for completion. It tries SSE first and
// falls back to polling with exponential backoff when SSE is unavailable.
func (c *AgentClient) SendTaskSync(ctx context.Context, task a2a.Task, timeout time.Duration) (*a2a.Task, error) {
	result, err := c.SendTask(task)
	if err != nil {
		return nil, err
	}
	if result.Status == a2a.TaskStatusCompleted ||
		result.Status == a2a.TaskStatusFailed ||
		result.Status == a2a.TaskStatusCanceled ||
		result.Status == a2a.TaskStatusInputRequired {
		return result, nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sseDone := make(chan *a2a.Task, 1)
	go func() {
		var finalTask *a2a.Task
		err := c.StreamTask(ctx, result.ID, func(eventType, data string) {
			if strings.Contains(data, "\"type\":\"final\"") {
				var wrapper struct {
					Type string   `json:"type"`
					Task a2a.Task `json:"task"`
				}
				if json.Unmarshal([]byte(data), &wrapper) == nil {
					finalTask = &wrapper.Task
				}
			}
		})
		if err == nil && finalTask != nil {
			sseDone <- finalTask
		} else {
			sseDone <- nil
		}
	}()

	select {
	case task := <-sseDone:
		if task != nil {
			return task, nil
		}
		// SSE failed, fall through to polling
	case <-ctx.Done():
		return result, ctx.Err()
	}

	return c.pollUntilDone(ctx, result.ID)
}

func (c *AgentClient) pollUntilDone(ctx context.Context, taskID string) (*a2a.Task, error) {
	backoff := 500 * time.Millisecond
	maxBackoff := 5 * time.Second
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		task, err := c.GetTask(taskID)
		if err != nil {
			return nil, err
		}
		if task.Status == a2a.TaskStatusCompleted ||
			task.Status == a2a.TaskStatusFailed ||
			task.Status == a2a.TaskStatusCanceled ||
			task.Status == a2a.TaskStatusInputRequired {
			return task, nil
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
