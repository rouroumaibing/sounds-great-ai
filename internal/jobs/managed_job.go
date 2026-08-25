// Package jobs implements durable, long-running job execution (roadmap P1-F).
//
// It covers: ManagedJob lifecycle with persistence so a process restart can
// recover in-flight work without loss (F261); a work-identity + three-bucket
// attribution ledger (F275); callback-auth credentials with a bounded lifetime
// and a tombstone (F298); and canonical event delivery to running agents with a
// preflight check (F300).
package jobs

import (
	"errors"
	"sync"
	"time"
)

// JobStatus is the lifecycle state of a managed job.
type JobStatus string

const (
	// JobCreated is registered but not yet started.
	JobCreated JobStatus = "created"
	// JobRunning is executing.
	JobRunning JobStatus = "running"
	// JobSucceeded finished successfully.
	JobSucceeded JobStatus = "succeeded"
	// JobFailed finished with an error (recoverable via Restart).
	JobFailed JobStatus = "failed"
)

var jobTransitions = map[JobStatus][]JobStatus{
	JobCreated:  {JobRunning},
	JobRunning:  {JobSucceeded, JobFailed, JobCreated}, // ->created = recovery restart
	JobSucceeded: {},
	JobFailed:   {JobCreated},
}

// CanTransition reports whether from->to is allowed.
func (s JobStatus) CanTransition(to JobStatus) bool {
	for _, n := range jobTransitions[s] {
		if n == to {
			return true
		}
	}
	return false
}

// ErrInvalidJobTransition is returned for an illegal status move.
var ErrInvalidJobTransition = errors.New("jobs: invalid job status transition")

// ManagedJob is a durable unit of work.
type ManagedJob struct {
	ID        string
	WorkID    string // stable work identity across attempts (F275)
	AttemptID string // unique per execution attempt
	ThreadID  string
	Payload   map[string]any

	Status     JobStatus
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	LastError  string

	// Ledger attributes resource consumption to the three buckets.
	Ledger *AttributionLedger
}

// NewManagedJob creates a Created job with a stable WorkID and a fresh attempt.
func NewManagedJob(id, workID, threadID string) *ManagedJob {
	now := time.Now()
	return &ManagedJob{
		ID:        id,
		WorkID:    workID,
		AttemptID: newAttemptID(workID, now),
		ThreadID:  threadID,
		Status:    JobCreated,
		CreatedAt: now,
		Ledger:    NewAttributionLedger(workID),
	}
}

// Transition validates and applies a status move, stamping timestamps.
func (j *ManagedJob) Transition(to JobStatus) error {
	if j.Status == to {
		return nil
	}
	if !j.Status.CanTransition(to) {
		return ErrInvalidJobTransition
	}
	now := time.Now()
	switch to {
	case JobRunning:
		j.StartedAt = now
		j.LastError = ""
		j.AttemptID = newAttemptID(j.WorkID, now)
	case JobSucceeded, JobFailed:
		j.FinishedAt = now
	}
	j.Status = to
	return nil
}

func newAttemptID(workID string, now time.Time) string {
	return workID + "-" + now.Format("20060102-150405.000")
}

// --- persistence ----------------------------------------------------------

// JobStore persists ManagedJobs. Implementations must be safe for concurrent
// use. A file-backed store should use atomic writes + tmp (per AGENTS.md, the
// persistence location is supplied by the caller, not hardcoded).
type JobStore interface {
	Save(j *ManagedJob) error
	Load(id string) (*ManagedJob, error)
	List() ([]*ManagedJob, error)
}

// InMemoryStore is a goroutine-safe JobStore for tests and single-process use.
type InMemoryStore struct {
	mu  sync.Mutex
	m   map[string]*ManagedJob
}

// NewInMemoryStore creates an empty store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{m: make(map[string]*ManagedJob)}
}

// Save persists (overwrites) a job.
func (s *InMemoryStore) Save(j *ManagedJob) error {
	if j == nil || j.ID == "" {
		return errors.New("jobs: nil/empty-id job")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *j
	if j.Ledger != nil {
		cp.Ledger = j.Ledger.clone()
	}
	s.m[j.ID] = &cp
	return nil
}

// Load returns a copy of the job, or ErrJobNotFound.
func (s *InMemoryStore) Load(id string) (*ManagedJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.m[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	cp := *j
	if j.Ledger != nil {
		cp.Ledger = j.Ledger.clone()
	}
	return &cp, nil
}

// List returns copies of all jobs.
func (s *InMemoryStore) List() ([]*ManagedJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*ManagedJob, 0, len(s.m))
	for _, j := range s.m {
		cp := *j
		out = append(out, &cp)
	}
	return out, nil
}

// ErrJobNotFound is returned when a job id is unknown.
var ErrJobNotFound = errors.New("jobs: job not found")

// Recover returns jobs that were Running at last save (interrupted by a
// restart). The caller is expected to resume each (Transition(JobCreated) then
// JobRunning) so work is not lost.
func Recover(store JobStore) ([]*ManagedJob, error) {
	all, err := store.List()
	if err != nil {
		return nil, err
	}
	var interrupted []*ManagedJob
	for _, j := range all {
		if j.Status == JobRunning {
			interrupted = append(interrupted, j)
		}
	}
	return interrupted, nil
}
