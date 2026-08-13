package stores

import (
	"context"

	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/domains/routing/ports"
)

// A2AHubAdapter adapts the flat internal/a2a.A2AHub to the ports.IA2AHub port.
// It is the D4-2b migration shim: the transport/sop layers depend on the port
// (and on *a2a.Thread), while the concrete hub implementation stays in
// internal/a2a. The ctx parameters are accepted for port symmetry and ignored
// because the hub is an in-memory structure with no blocking IO.
type A2AHubAdapter struct {
	hub *a2a.A2AHub
}

// NewA2AHubAdapter wraps an *a2a.A2AHub behind the IA2AHub port.
func NewA2AHubAdapter(hub *a2a.A2AHub) *A2AHubAdapter {
	return &A2AHubAdapter{hub: hub}
}

// GetThread returns the a2a thread for an id (nil if absent).
func (a *A2AHubAdapter) GetThread(_ context.Context, id string) *a2a.Thread {
	return a.hub.GetThread(id)
}

// CreateThread starts a new a2a thread with the given participants.
func (a *A2AHubAdapter) CreateThread(_ context.Context, task string, participants []string) *a2a.Thread {
	return a.hub.CreateThread(task, participants)
}

// Handoff records a handoff on the thread and returns the updated thread.
func (a *A2AHubAdapter) Handoff(_ context.Context, thread *a2a.Thread, hf a2a.Handoff) (*a2a.Thread, error) {
	return a.hub.Handoff(thread, hf)
}

// Ensure A2AHubAdapter satisfies the port at compile time.
var _ ports.IA2AHub = (*A2AHubAdapter)(nil)
