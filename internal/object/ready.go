// Package object models object-driven experience readiness signals (F283):
// backend objects expose a readiness state consumed by the UI runtime.
package object

import (
	"sync"
	"time"
)

// Readiness is the readiness signal of a backend object.
type Readiness struct {
	ID        string
	Kind      string
	Ready     bool
	Reason    string
	UpdatedAt time.Time
}

// Registry tracks object readiness signals (F283). Single source of truth.
type Registry struct {
	mu      sync.RWMutex
	objects map[string]Readiness
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry { return &Registry{objects: make(map[string]Readiness)} }

// Set records/replaces an object's readiness signal.
func (r *Registry) Set(o Readiness) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o.UpdatedAt = time.Now()
	r.objects[o.ID] = o
}

// Ready reports whether an object is ready. Unknown objects are NOT ready
// (fail-closed: an absent readiness must never be assumed ready).
func (r *Registry) Ready(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.objects[id]
	return ok && o.Ready
}

// Signal returns the full readiness signal for an object.
func (r *Registry) Signal(id string) (Readiness, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.objects[id]
	return o, ok
}
