package unified

import (
	"sync"
	"time"
)

type ProcessStatus string

const (
	StatusRunning ProcessStatus = "running"
	StatusExited  ProcessStatus = "exited"
	StatusKilled  ProcessStatus = "killed"
	StatusTimeout ProcessStatus = "timeout"
)

type ProcessRecord struct {
	PID       int
	Command   string
	CLIType   string
	StartedAt time.Time
	EndedAt   *time.Time
	ExitCode  *int
	Signal    string
	Status    ProcessStatus
}

type ProcessRegistry struct {
	mu    sync.RWMutex
	procs map[int]*ProcessRecord
}

func NewProcessRegistry() *ProcessRegistry {
	return &ProcessRegistry{procs: make(map[int]*ProcessRecord)}
}

func (r *ProcessRegistry) Register(pid int, command, cliType string) *ProcessRecord {
	rec := &ProcessRecord{
		PID:       pid,
		Command:   command,
		CLIType:   cliType,
		StartedAt: time.Now(),
		Status:    StatusRunning,
	}
	r.mu.Lock()
	r.procs[pid] = rec
	r.mu.Unlock()
	return rec
}

func (r *ProcessRegistry) UpdateExit(pid int, exitCode *int, signal string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.procs[pid]
	if !ok {
		return
	}
	now := time.Now()
	rec.EndedAt = &now
	rec.ExitCode = exitCode
	rec.Signal = signal
	if signal != "" {
		rec.Status = StatusKilled
	} else {
		rec.Status = StatusExited
	}
}

func (r *ProcessRegistry) UpdateTimeout(pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.procs[pid]
	if !ok {
		return
	}
	now := time.Now()
	rec.EndedAt = &now
	rec.Status = StatusTimeout
}

func (r *ProcessRegistry) Get(pid int) *ProcessRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.procs[pid]
}

func (r *ProcessRegistry) List() []*ProcessRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*ProcessRecord, 0, len(r.procs))
	for _, rec := range r.procs {
		list = append(list, rec)
	}
	return list
}

func (r *ProcessRegistry) Remove(pid int) {
	r.mu.Lock()
	delete(r.procs, pid)
	r.mu.Unlock()
}
