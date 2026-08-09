package ports

import "context"

// HandoffRequest represents an A2A handoff request.
type HandoffRequest struct {
	FromBreed  string `json:"from_breed"`
	ToBreed    string `json:"to_breed"`
	ThreadID   string `json:"thread_id"`
	Message    string `json:"message"`
	Context    map[string]any `json:"context,omitempty"`
}

// HandoffResult represents the result of an A2A handoff.
type HandoffResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// IMentionRouter is the port for mention routing.
type IMentionRouter interface {
	Route(ctx context.Context, message string) (RoutingDecision, error)
}

// RoutingDecision represents the result of mention routing.
type RoutingDecision struct {
	TargetBreed string `json:"target_breed"`
	IsHandoff   bool   `json:"is_handoff"`
	IsParallel  bool   `json:"is_parallel"`
	Targets     []string `json:"targets,omitempty"`
}

// IA2AHub is the port for A2A handoff coordination.
type IA2AHub interface {
	Handoff(ctx context.Context, req HandoffRequest) (HandoffResult, error)
}
