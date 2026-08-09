package ports

import (
	"context"
	"io"
)

// ExecuteRequest represents a request to execute an agent.
type ExecuteRequest struct {
	BreedID   string
	ThreadID  string
	Message   string
	Stream    bool
	Context   map[string]any
}

// StreamEvent represents a streaming event from agent execution.
type StreamEvent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// IAgentExecutor is the port for agent execution.
type IAgentExecutor interface {
	Execute(ctx context.Context, req ExecuteRequest) (<-chan StreamEvent, error)
}

// IProcessManager is the port for managing agent processes.
type IProcessManager interface {
	Start(ctx context.Context, breedID string, args []string) (io.Closer, error)
	Stop(breedID string) error
	IsRunning(breedID string) bool
}
