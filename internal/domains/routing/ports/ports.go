package ports

import (
	"context"

	"sounds-great-ai/internal/a2a"
)

// IMentionRouter is the port for mention routing.
type IMentionRouter interface {
	Route(ctx context.Context, message string) (RoutingDecision, error)
}

// RoutingDecision represents the result of mention routing.
// Field set mirrors the runtime implementation that previously lived in
// internal/platform/router.go (kept identical during the D4-2 migration so the
// transport layer consumes the same shape).
type RoutingDecision struct {
	TargetBreeds []string `json:"target_breeds"`
	Strategy     string   `json:"strategy"` // "single" | "serial" | "parallel"
	HasMentions  bool     `json:"has_mentions"`
	Warnings     []string `json:"warnings,omitempty"`
}

// IA2AHub is the port for A2A handoff coordination. It is satisfied by an
// adapter wrapping the flat internal/a2a.A2AHub (D4-2b migration); the method
// signatures mirror a2a.A2AHub so the transport/sop layers keep consuming the
// same *a2a.Thread shape they already depend on.
type IA2AHub interface {
	GetThread(ctx context.Context, id string) *a2a.Thread
	CreateThread(ctx context.Context, task string, participants []string) *a2a.Thread
	Handoff(ctx context.Context, thread *a2a.Thread, hf a2a.Handoff) (*a2a.Thread, error)
}
