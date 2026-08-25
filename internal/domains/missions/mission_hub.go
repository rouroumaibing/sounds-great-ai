package missions

import (
	"errors"
	"sync"
)

// ErrTaskNotFound is returned when a task id is unknown.
var ErrTaskNotFound = errors.New("missions: task not found")

// ErrTaskExists is returned when registering a duplicate id.
var ErrTaskExists = errors.New("missions: task already exists")

// MissionHub is a goroutine-safe registry of TaskSpecs. It is the in-memory
// task pool backing the task board API. Persistence (if desired) wraps this.
type MissionHub struct {
	mu    sync.RWMutex
	tasks map[string]*TaskSpec
}

// NewMissionHub creates an empty hub.
func NewMissionHub() *MissionHub {
	return &MissionHub{tasks: make(map[string]*TaskSpec)}
}

// Register inserts a new task. Returns ErrTaskExists on duplicate id.
func (h *MissionHub) Register(t *TaskSpec) error {
	if t == nil || t.ID == "" {
		return errors.New("missions: nil or empty-id task")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.tasks[t.ID]; ok {
		return ErrTaskExists
	}
	h.tasks[t.ID] = t
	return nil
}

// Get returns a task by id (nil, ErrTaskNotFound if absent).
func (h *MissionHub) Get(id string) (*TaskSpec, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	t, ok := h.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return t, nil
}

// List returns all tasks (caller gets a shallow snapshot; specs are the live
// pointers, so callers must not mutate status outside Transition).
func (h *MissionHub) List() []*TaskSpec {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*TaskSpec, 0, len(h.tasks))
	for _, t := range h.tasks {
		out = append(out, t)
	}
	return out
}

// ListForThread returns tasks belonging to a thread.
func (h *MissionHub) ListForThread(threadID string) []*TaskSpec {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*TaskSpec
	for _, t := range h.tasks {
		if t.ThreadID == threadID {
			out = append(out, t)
		}
	}
	return out
}

// UpdateStatus applies a validated transition to a registered task.
func (h *MissionHub) UpdateStatus(id string, to TaskStatus) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}
	return t.Transition(to)
}
