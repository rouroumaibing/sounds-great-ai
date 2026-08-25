package jobs

import (
	"errors"
	"sync"
)

// CanonicalEventKind enumerates the canonical events a running agent may
// receive mid-execution (F300): cancel / quota / pluginReady.
type CanonicalEventKind string

const (
	// EventCancel instructs the running agent to abort.
	EventCancel CanonicalEventKind = "cancel"
	// EventQuota informs the agent it hit a usage/quota boundary.
	EventQuota CanonicalEventKind = "quota"
	// EventPluginReady informs the agent a plugin became available.
	EventPluginReady CanonicalEventKind = "pluginReady"
)

// CanonicalEvent is a normalized event delivered to a running agent.
type CanonicalEvent struct {
	Kind     CanonicalEventKind
	ThreadID string
	JobID    string
	Payload  map[string]any
}

// DeliveryTarget is a running agent (or its proxy) that can receive events.
type DeliveryTarget interface {
	// Running reports whether the target is still executing and able to accept
	// events. A stopped target must report false so Preflight refuses.
	Running() bool
	// Receive handles a canonical event. Must be safe to call only after a
	// successful Preflight.
	Receive(e CanonicalEvent) error
}

// ErrNotRunning is returned when delivering to a target that is not running.
var ErrNotRunning = errors.New("jobs: delivery target not running")

// ErrNoTarget is returned when no target is registered for a job.
var ErrNoTarget = errors.New("jobs: no delivery target for job")

// Deliverer delivers canonical events to running agents with a preflight gate:
// an event is only handed to a target that is currently Running. This prevents
// delivering lifecycle signals to dead agents (F300 home-situation delivery).
type Deliverer struct {
	mu      sync.Mutex
	targets map[string]DeliveryTarget
}

// NewDeliverer creates an empty deliverer.
func NewDeliverer() *Deliverer {
	return &Deliverer{targets: make(map[string]DeliveryTarget)}
}

// Register binds a running target to a job id.
func (d *Deliverer) Register(jobID string, t DeliveryTarget) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.targets[jobID] = t
}

// Unregister removes a target (e.g. on job completion).
func (d *Deliverer) Unregister(jobID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.targets, jobID)
}

// Preflight verifies a target exists and is running.
func (d *Deliverer) Preflight(jobID string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t, ok := d.targets[jobID]
	if !ok {
		return false, ErrNoTarget
	}
	if !t.Running() {
		return false, ErrNotRunning
	}
	return true, nil
}

// Deliver runs the preflight gate then hands the event to the target.
func (d *Deliverer) Deliver(jobID string, e CanonicalEvent) error {
	ok, err := d.Preflight(jobID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotRunning
	}
	d.mu.Lock()
	t := d.targets[jobID]
	d.mu.Unlock()
	return t.Receive(e)
}
