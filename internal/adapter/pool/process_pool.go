package pool

import (
	"fmt"
	"sync"
	"time"
)

// PoolKey is the composite key for pool entries: (projectPath, providerProfile).
type PoolKey struct {
	ProjectPath    string
	ProviderProfile string
}

// String returns a string representation of the key.
func (k PoolKey) String() string {
	return fmt.Sprintf("%s|%s", k.ProjectPath, k.ProviderProfile)
}

// PoolEntry is a single entry in the process pool.
type PoolEntry struct {
	key            PoolKey
	process        *PooledProcess
	lastUsed       time.Time
	sessionOwners  map[string]bool // sessionIDs currently holding a lease
	leaseGeneration uint64
	createdAt      time.Time
}

// ProcessPool manages long-running CLI processes keyed by (projectPath, providerProfile).
// It provides LRU eviction and TTL-based idle eviction.
type ProcessPool struct {
	mu          sync.Mutex
	entries     map[string]*PoolEntry
	maxCapacity int
	ttl         time.Duration
	metrics     *PoolMetrics
	processFactory ProcessFactory
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// ProcessFactory creates a new pooled process. This is the spawn abstraction.
// The pool uses this factory for cold starts; existing one-shot spawn is the fallback.
type ProcessFactory func(key PoolKey) (*PooledProcess, error)

// DefaultProcessFactory is the default factory using exec.Command.
func DefaultProcessFactory(key PoolKey) (*PooledProcess, error) {
	// This is a placeholder — the real factory is injected by the adapter layer
	// to avoid duplicating spawn logic. The pool doesn't own spawn details.
	return NewPooledProcess(0, "", nil, key.ProjectPath), nil
}

// PoolConfig holds configuration for the process pool.
type PoolConfig struct {
	MaxCapacity    int           // max number of pooled processes (default 8)
	TTL            time.Duration // idle TTL before eviction (default 30 min)
	HealthInterval time.Duration // zombie check interval (default 5 min)
}

// DefaultPoolConfig returns sensible defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxCapacity:    8,
		TTL:            30 * time.Minute,
		HealthInterval: 5 * time.Minute,
	}
}

// NewProcessPool creates a new ProcessPool with the given config and factory.
func NewProcessPool(config PoolConfig, factory ProcessFactory) *ProcessPool {
	if config.MaxCapacity <= 0 {
		config = DefaultPoolConfig()
	}
	if factory == nil {
		factory = DefaultProcessFactory
	}
	p := &ProcessPool{
		entries:        make(map[string]*PoolEntry),
		maxCapacity:    config.MaxCapacity,
		ttl:            config.TTL,
		metrics:        NewPoolMetrics(),
		processFactory: factory,
		stopCh:         make(chan struct{}),
	}
	// Start background health check and TTL eviction
	p.wg.Add(1)
	go p.backgroundLoop(config.HealthInterval)
	return p
}

// Acquire gets or creates a process for the given key and returns a Lease.
// If a warm process exists, it's reused (warm hit). Otherwise a new one is spawned (cold start).
func (p *ProcessPool) Acquire(key PoolKey, sessionID string) (*Lease, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.metrics.LeaseAcquire()
	k := key.String()
	entry, exists := p.entries[k]

	if exists && !isProcessStale(entry) {
		// Warm hit: reuse existing process
		p.metrics.WarmHit()
		entry.lastUsed = time.Now()
		entry.leaseGeneration++
		entry.sessionOwners[sessionID] = true
		return &Lease{
			pool:       p,
			entry:      entry,
			sessionID:  sessionID,
			generation: entry.leaseGeneration,
		}, nil
	}

	// Cold start: need to create a new process
	if exists && isProcessStale(entry) {
		// Stale entry: clean up and remove
		p.metrics.ZombieCleanup()
		delete(p.entries, k)
	}

	// Check capacity and evict LRU if needed
	if len(p.entries) >= p.maxCapacity {
		p.evictLRU()
	}

	// Spawn new process
	process, err := p.processFactory(key)
	if err != nil {
		return nil, fmt.Errorf("pool spawn failed: %w", err)
	}
	p.metrics.ColdStart()

	entry = &PoolEntry{
		key:           key,
		process:       process,
		lastUsed:      time.Now(),
		sessionOwners: map[string]bool{sessionID: true},
		leaseGeneration: 1,
		createdAt:     time.Now(),
	}
	p.entries[k] = entry

	return &Lease{
		pool:       p,
		entry:      entry,
		sessionID:  sessionID,
		generation: 1,
	}, nil
}

// releaseEntry returns a process to the pool (called by Lease.Release).
func (p *ProcessPool) releaseEntry(entry *PoolEntry, sessionID string, generation uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.metrics.LeaseRelease()

	// Only release if the generation matches (not a stale lease)
	if entry.leaseGeneration != generation {
		return
	}

	// Remove this session from the owners
	delete(entry.sessionOwners, sessionID)
	entry.lastUsed = time.Now()

	// If the process is stale, clean it up
	if isProcessStale(entry) {
		p.metrics.ZombieCleanup()
		delete(p.entries, entry.key.String())
	}
}

// evictLRU evicts the least recently used entry. Caller must hold the lock.
func (p *ProcessPool) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	for k, entry := range p.entries {
		// Don't evict entries with active leases
		if len(entry.sessionOwners) > 0 {
			continue
		}
		if oldestKey == "" || entry.lastUsed.Before(oldestTime) {
			oldestKey = k
			oldestTime = entry.lastUsed
		}
	}
	if oldestKey != "" {
		entry := p.entries[oldestKey]
		if entry.process != nil {
			entry.process.MarkDead()
		}
		delete(p.entries, oldestKey)
		p.metrics.LRUEviction()
	}
}

// backgroundLoop runs periodic health checks and TTL evictions.
func (p *ProcessPool) backgroundLoop(interval time.Duration) {
	defer p.wg.Done()
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.healthCheck()
			p.evictExpired()
		}
	}
}

// healthCheck scans for zombie processes and cleans them up.
func (p *ProcessPool) healthCheck() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, entry := range p.entries {
		if len(entry.sessionOwners) > 0 {
			continue // don't touch entries with active leases
		}
		if isProcessStale(entry) {
			if entry.process != nil {
				entry.process.MarkDead()
			}
			delete(p.entries, k)
			p.metrics.ZombieCleanup()
		}
	}
}

// evictExpired evicts entries that have been idle longer than TTL.
func (p *ProcessPool) evictExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	for k, entry := range p.entries {
		if len(entry.sessionOwners) > 0 {
			continue // don't evict entries with active leases
		}
		if now.Sub(entry.lastUsed) > p.ttl {
			if entry.process != nil {
				entry.process.MarkDead()
			}
			delete(p.entries, k)
			p.metrics.TTLEviction()
		}
	}
}

// Close shuts down the pool and all background goroutines.
func (p *ProcessPool) Close() {
	close(p.stopCh)
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, entry := range p.entries {
		if entry.process != nil {
			entry.process.MarkDead()
		}
	}
	p.entries = make(map[string]*PoolEntry)
}

// Size returns the current number of pooled processes.
func (p *ProcessPool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}

// Metrics returns the pool metrics.
func (p *ProcessPool) Metrics() *PoolMetrics {
	return p.metrics
}

// MetricsSnapshot returns a point-in-time metrics snapshot.
func (p *ProcessPool) MetricsSnapshot() MetricsSnapshot {
	p.mu.Lock()
	activeLeases := 0
	for _, entry := range p.entries {
		activeLeases += len(entry.sessionOwners)
	}
	poolSize := len(p.entries)
	p.mu.Unlock()
	return p.metrics.Snapshot(poolSize, activeLeases)
}

// ActiveLeasesForSession returns the number of active leases for a session.
func (p *ProcessPool) ActiveLeasesForSession(sessionID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, entry := range p.entries {
		if entry.sessionOwners[sessionID] {
			count++
		}
	}
	return count
}
