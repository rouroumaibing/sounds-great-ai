// Package missions implements the Mission Hub task pool and TaskSpec lifecycle
// (roadmap P0-5). It is the backend task center: tasks are registered (often
// conversationally), scheduled (cron or one-shot), run, and completed. The
// scheduler package consumes TaskSpecs to fire due work.
package missions

import (
	"errors"
	"time"
)

// TaskStatus is the lifecycle state of a task.
type TaskStatus string

const (
	// TaskOpen is the initial registered state, not yet scheduled.
	TaskOpen TaskStatus = "open"
	// TaskScheduled has a due time / cron and is waiting to run.
	TaskScheduled TaskStatus = "scheduled"
	// TaskRunning is currently executing.
	TaskRunning TaskStatus = "running"
	// TaskDone finished successfully.
	TaskDone TaskStatus = "done"
	// TaskCancelled is a terminal, non-executing state.
	TaskCancelled TaskStatus = "cancelled"
	// TaskFailed finished with an error (terminal but re-runnable via reschedule).
	TaskFailed TaskStatus = "failed"
)

var taskTransitions = map[TaskStatus][]TaskStatus{
	TaskOpen:     {TaskScheduled, TaskCancelled},
	TaskScheduled: {TaskRunning, TaskCancelled, TaskOpen},
	TaskRunning:  {TaskDone, TaskFailed, TaskScheduled},
	TaskDone:     {TaskScheduled}, // re-schedule a recurring task
	TaskFailed:   {TaskScheduled, TaskCancelled},
	TaskCancelled: {},
}

// CanTransition reports whether from->to is permitted.
func (s TaskStatus) CanTransition(to TaskStatus) bool {
	for _, n := range taskTransitions[s] {
		if n == to {
			return true
		}
	}
	return false
}

// ErrInvalidTaskTransition is returned for an illegal status move.
var ErrInvalidTaskTransition = errors.New("missions: invalid task status transition")

// TaskSpec is a single mission-hub task.
type TaskSpec struct {
	ID          string
	Title       string
	Description string
	ThreadID    string
	CreatedBy   string
	// Cron is an optional schedule expression (see scheduler.Spec). Empty means
	// a one-shot task fired by explicit scheduling.
	Cron string
	// Payload is free-form task input (JSON-serializable by the caller).
	Payload map[string]any

	Status     TaskStatus
	CreatedAt  time.Time
	ScheduledAt time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	LastError  string
}

// NewTaskSpec creates an Open task with a generated id and timestamps.
func NewTaskSpec(id, title, createdBy string) *TaskSpec {
	now := time.Now()
	return &TaskSpec{
		ID:        id,
		Title:     title,
		CreatedBy: createdBy,
		Status:    TaskOpen,
		CreatedAt: now,
	}
}

// Transition validates and applies a status move, updating the relevant
// timestamp. It returns ErrInvalidTaskTransition on an illegal move and leaves
// the spec unchanged.
func (t *TaskSpec) Transition(to TaskStatus) error {
	if t.Status == to {
		return nil
	}
	if !t.Status.CanTransition(to) {
		return ErrInvalidTaskTransition
	}
	now := time.Now()
	switch to {
	case TaskScheduled:
		t.ScheduledAt = now
	case TaskRunning:
		t.StartedAt = now
		t.LastError = ""
	case TaskDone, TaskFailed:
		t.FinishedAt = now
	}
	t.Status = to
	return nil
}
