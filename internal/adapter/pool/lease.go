package pool

import (
	"os"
	"sync"
	"time"
)

// Lease represents a handle to a pooled process acquired by a session.
type Lease struct {
	mu             sync.Mutex
	pool           *ProcessPool
	entry          *PoolEntry
	sessionID      string
	generation     uint64
	released       bool
}

// Process returns the underlying process handle.
func (l *Lease) Process() *PooledProcess {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.entry.process
}

// SessionID returns the session that owns this lease.
func (l *Lease) SessionID() string {
	return l.sessionID
}

// Generation returns the lease generation number.
func (l *Lease) Generation() uint64 {
	return l.generation
}

// IsReleased checks if the lease has been released.
func (l *Lease) IsReleased() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.released
}

// Release returns the process to the pool and starts the TTL timer.
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if l.released {
		l.mu.Unlock()
		return
	}
	l.released = true
	entry := l.entry
	pool := l.pool
	l.mu.Unlock()

	if pool != nil && entry != nil {
		pool.releaseEntry(entry, l.sessionID, l.generation)
	}
}

// IsStale checks if the lease is stale (zombie process or cwd deleted).
func (l *Lease) IsStale() bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.released {
		return true
	}
	return isProcessStale(l.entry)
}

// isProcessStale checks if a pooled process is stale.
func isProcessStale(entry *PoolEntry) bool {
	if entry == nil || entry.process == nil {
		return true
	}
	// Check if process is still alive
	if !entry.process.IsAlive() {
		return true
	}
	// Check if cwd still exists
	if entry.process.WorkDir() != "" {
		if _, err := os.Stat(entry.process.WorkDir()); err != nil {
			return true
		}
	}
	return false
}

// HealthCheck performs a health check on a lease.
// Returns true if healthy, false if stale.
func (l *Lease) HealthCheck() bool {
	return !l.IsStale()
}

// PooledProcess is a handle to a long-running CLI process in the pool.
type PooledProcess struct {
	mu      sync.Mutex
	pid     int
	cmd     string
	args    []string
	workDir string
	alive   bool
	startedAt time.Time
}

// NewPooledProcess creates a new PooledProcess handle.
func NewPooledProcess(pid int, cmd string, args []string, workDir string) *PooledProcess {
	return &PooledProcess{
		pid:      pid,
		cmd:      cmd,
		args:     args,
		workDir:  workDir,
		alive:    true,
		startedAt: time.Now(),
	}
}

// PID returns the process ID.
func (p *PooledProcess) PID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pid
}

// Command returns the command string.
func (p *PooledProcess) Command() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd
}

// Args returns the command arguments.
func (p *PooledProcess) Args() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.args
}

// WorkDir returns the working directory.
func (p *PooledProcess) WorkDir() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.workDir
}

// IsAlive returns whether the process is still alive.
func (p *PooledProcess) IsAlive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.alive
}

// MarkDead marks the process as dead (zombie cleanup).
func (p *PooledProcess) MarkDead() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.alive = false
}

// StartedAt returns when the process was started.
func (p *PooledProcess) StartedAt() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.startedAt
}
