package scheduler

import (
	"errors"
	"sync"
	"time"

	"sounds-great-ai/internal/domains/missions"
)

// Handler executes a fired task. Returning nil marks the task done; a non-nil
// error marks it failed. The scheduler never swallows the error.
type Handler func(t *missions.TaskSpec) error

// ErrNoHandler is returned when firing without a registered handler.
var ErrNoHandler = errors.New("scheduler: no handler registered")

// Scheduler drives due TaskSpecs. It computes each cron task's next fire time
// and, on Tick, transitions scheduled -> running -> done/failed, rescheduling
// recurring tasks.
type Scheduler struct {
	mu       sync.Mutex
	hub      *missions.MissionHub
	handler  Handler
	nextFire map[string]time.Time
	// nowFn allows injecting a clock in tests; defaults to time.Now.
	nowFn func() time.Time
}

// New creates a scheduler over the given hub. handler may be set later via
// SetHandler.
func New(hub *missions.MissionHub) *Scheduler {
	return &Scheduler{
		hub:      hub,
		nextFire: make(map[string]time.Time),
		nowFn:    time.Now,
	}
}

// SetHandler installs the execution callback.
func (s *Scheduler) SetHandler(h Handler) { s.handler = h }

// ScheduleCron validates expr, registers the task in the hub (status scheduled),
// and records its next fire time. One-shot tasks (empty Cron) are scheduled
// immediately (nextFire = now).
func (s *Scheduler) ScheduleCron(t *missions.TaskSpec, expr string) error {
	if expr != "" {
		if _, err := Parse(expr); err != nil {
			return err
		}
		t.Cron = expr
	}
	if err := s.hub.Register(t); err != nil {
		return err
	}
	now := s.nowFn()
	if err := t.Transition(missions.TaskScheduled); err != nil {
		return err
	}
	s.mu.Lock()
	if expr == "" {
		s.nextFire[t.ID] = now
	} else {
		s.nextFire[t.ID] = ParseOrNow(expr, now)
	}
	s.mu.Unlock()
	return nil
}

// ParseOrNow parses expr and returns its next fire, falling back to now on a
// parse error (so a malformed expr never blocks firing).
func ParseOrNow(expr string, now time.Time) time.Time {
	if sp, err := Parse(expr); err == nil {
		return sp.Next(now)
	}
	return now
}

// Tick finds tasks due at or before now, fires each via the handler, and
// applies lifecycle transitions. It returns the count of tasks fired. A missing
// handler yields ErrNoHandler without firing anything.
func (s *Scheduler) Tick() (int, error) {
	if s.handler == nil {
		return 0, ErrNoHandler
	}
	now := s.nowFn()
	s.mu.Lock()
	var due []string
	for id, nf := range s.nextFire {
		if !nf.After(now) {
			due = append(due, id)
		}
	}
	s.mu.Unlock()

	fired := 0
	for _, id := range due {
		t, err := s.hub.Get(id)
		if err != nil {
			continue
		}
		if t.Status != missions.TaskScheduled {
			continue
		}
		if err := t.Transition(missions.TaskRunning); err != nil {
			continue
		}
		runErr := s.handler(t)
		if runErr != nil {
			t.LastError = runErr.Error()
			_ = t.Transition(missions.TaskFailed)
		} else {
			_ = t.Transition(missions.TaskDone)
		}
		fired++

		// Reschedule recurring tasks; one-shot tasks stay done.
		s.mu.Lock()
		if t.Cron != "" {
			s.nextFire[id] = ParseOrNow(t.Cron, now)
			_ = t.Transition(missions.TaskScheduled)
		} else {
			delete(s.nextFire, id)
		}
		s.mu.Unlock()
	}
	return fired, nil
}

// NextFire returns the recorded next fire time for a task id (zero, false if
// unknown).
func (s *Scheduler) NextFire(id string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nf, ok := s.nextFire[id]
	return nf, ok
}
