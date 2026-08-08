package pool

import (
	"sync"
	"sync/atomic"
	"time"
)

// PoolMetrics tracks process pool statistics.
type PoolMetrics struct {
	warmHitCount       atomic.Int64
	coldStartCount     atomic.Int64
	evictionCount      atomic.Int64
	zombieCleanupCount atomic.Int64
	leaseAcquireCount  atomic.Int64
	leaseReleaseCount  atomic.Int64
	ttlEvictionCount   atomic.Int64
	lruEvictionCount   atomic.Int64
}

// NewPoolMetrics creates a new PoolMetrics.
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{}
}

// WarmHit records a warm cache hit (reused existing process).
func (m *PoolMetrics) WarmHit() { m.warmHitCount.Add(1) }

// ColdStart records a cold start (new process spawned).
func (m *PoolMetrics) ColdStart() { m.coldStartCount.Add(1) }

// Eviction records an eviction (LRU or TTL).
func (m *PoolMetrics) Eviction() { m.evictionCount.Add(1) }

// ZombieCleanup records a zombie process cleanup.
func (m *PoolMetrics) ZombieCleanup() { m.zombieCleanupCount.Add(1) }

// LeaseAcquire records a lease acquisition.
func (m *PoolMetrics) LeaseAcquire() { m.leaseAcquireCount.Add(1) }

// LeaseRelease records a lease release.
func (m *PoolMetrics) LeaseRelease() { m.leaseReleaseCount.Add(1) }

// TTLEviction records a TTL-based eviction.
func (m *PoolMetrics) TTLEviction() {
	m.ttlEvictionCount.Add(1)
	m.Eviction()
}

// LRUEviction records an LRU-based eviction.
func (m *PoolMetrics) LRUEviction() {
	m.lruEvictionCount.Add(1)
	m.Eviction()
}

// Snapshot returns a point-in-time snapshot of all metrics.
type MetricsSnapshot struct {
	WarmHitCount       int64     `json:"warm_hit_count"`
	ColdStartCount     int64     `json:"cold_start_count"`
	EvictionCount      int64     `json:"eviction_count"`
	ZombieCleanupCount int64     `json:"zombie_cleanup_count"`
	LeaseAcquireCount  int64     `json:"lease_acquire_count"`
	LeaseReleaseCount  int64     `json:"lease_release_count"`
	TTLEvictionCount   int64     `json:"ttl_eviction_count"`
	LRUEvictionCount   int64     `json:"lru_eviction_count"`
	PoolSize           int       `json:"pool_size"`
	ActiveLeases       int       `json:"active_leases"`
	Timestamp          time.Time `json:"timestamp"`
}

// Snapshot returns a point-in-time snapshot of all metrics.
func (m *PoolMetrics) Snapshot(poolSize, activeLeases int) MetricsSnapshot {
	return MetricsSnapshot{
		WarmHitCount:       m.warmHitCount.Load(),
		ColdStartCount:     m.coldStartCount.Load(),
		EvictionCount:      m.evictionCount.Load(),
		ZombieCleanupCount: m.zombieCleanupCount.Load(),
		LeaseAcquireCount:  m.leaseAcquireCount.Load(),
		LeaseReleaseCount:  m.leaseReleaseCount.Load(),
		TTLEvictionCount:   m.ttlEvictionCount.Load(),
		LRUEvictionCount:   m.lruEvictionCount.Load(),
		PoolSize:           poolSize,
		ActiveLeases:       activeLeases,
		Timestamp:          time.Now(),
	}
}

// Reset resets all metrics to zero (for testing).
func (m *PoolMetrics) Reset() {
	m.warmHitCount.Store(0)
	m.coldStartCount.Store(0)
	m.evictionCount.Store(0)
	m.zombieCleanupCount.Store(0)
	m.leaseAcquireCount.Store(0)
	m.leaseReleaseCount.Store(0)
	m.ttlEvictionCount.Store(0)
	m.lruEvictionCount.Store(0)
}

// MetricsCollector provides thread-safe access to metrics for endpoint integration.
type MetricsCollector struct {
	mu     sync.RWMutex
	source func() MetricsSnapshot
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector(source func() MetricsSnapshot) *MetricsCollector {
	return &MetricsCollector{source: source}
}

// Collect returns the current metrics snapshot.
func (c *MetricsCollector) Collect() MetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.source == nil {
		return MetricsSnapshot{Timestamp: time.Now()}
	}
	return c.source()
}
