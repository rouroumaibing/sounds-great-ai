package pool

import (
	"testing"
	"time"
)

// testFactory creates a factory that returns a simple PooledProcess.
// Uses empty workDir to skip the cwd stat check in tests.
func testFactory(key PoolKey) (*PooledProcess, error) {
	return NewPooledProcess(1234, "test-cli", []string{"--flag"}, ""), nil
}

func TestProcessPoolAcquireColdStart(t *testing.T) {
	config := PoolConfig{MaxCapacity: 4, TTL: time.Minute, HealthInterval: time.Hour}
	pool := NewProcessPool(config, testFactory)
	defer pool.Close()

	key := PoolKey{ProjectPath: "/tmp/project", ProviderProfile: "claude"}
	lease, err := pool.Acquire(key, "session-1")
	if err != nil {
		t.Fatalf("acquire error: %v", err)
	}
	if lease == nil {
		t.Fatal("expected non-nil lease")
	}
	if lease.Generation() != 1 {
		t.Errorf("expected generation 1, got %d", lease.Generation())
	}

	snap := pool.MetricsSnapshot()
	if snap.ColdStartCount != 1 {
		t.Errorf("expected 1 cold start, got %d", snap.ColdStartCount)
	}
}

func TestProcessPoolAcquireWarmHit(t *testing.T) {
	config := PoolConfig{MaxCapacity: 4, TTL: time.Minute, HealthInterval: time.Hour}
	pool := NewProcessPool(config, testFactory)
	defer pool.Close()

	key := PoolKey{ProjectPath: "/tmp/project", ProviderProfile: "claude"}
	lease1, _ := pool.Acquire(key, "session-1")
	lease1.Release()

	lease2, err := pool.Acquire(key, "session-2")
	if err != nil {
		t.Fatalf("acquire error: %v", err)
	}
	defer lease2.Release()

	snap := pool.MetricsSnapshot()
	if snap.WarmHitCount != 1 {
		t.Errorf("expected 1 warm hit, got %d", snap.WarmHitCount)
	}
	if snap.ColdStartCount != 1 {
		t.Errorf("expected 1 cold start, got %d", snap.ColdStartCount)
	}
}

func TestProcessPoolLRUEviction(t *testing.T) {
	config := PoolConfig{MaxCapacity: 2, TTL: time.Minute, HealthInterval: time.Hour}
	pool := NewProcessPool(config, testFactory)
	defer pool.Close()

	key1 := PoolKey{ProjectPath: "/tmp/p1", ProviderProfile: "claude"}
	key2 := PoolKey{ProjectPath: "/tmp/p2", ProviderProfile: "claude"}
	key3 := PoolKey{ProjectPath: "/tmp/p3", ProviderProfile: "claude"}

	lease1, _ := pool.Acquire(key1, "s1")
	lease1.Release()
	lease2, _ := pool.Acquire(key2, "s2")
	lease2.Release()

	// Acquire key3 should evict key1 (LRU)
	lease3, err := pool.Acquire(key3, "s3")
	if err != nil {
		t.Fatalf("acquire error: %v", err)
	}
	lease3.Release()

	snap := pool.MetricsSnapshot()
	if snap.LRUEvictionCount == 0 {
		t.Error("expected at least 1 LRU eviction")
	}
	if pool.Size() > 2 {
		t.Errorf("pool size = %d, should be <= 2", pool.Size())
	}
}

func TestProcessPoolLeaseRelease(t *testing.T) {
	config := PoolConfig{MaxCapacity: 4, TTL: time.Minute, HealthInterval: time.Hour}
	pool := NewProcessPool(config, testFactory)
	defer pool.Close()

	key := PoolKey{ProjectPath: "/tmp/project", ProviderProfile: "claude"}
	lease, _ := pool.Acquire(key, "session-1")
	if lease.IsReleased() {
		t.Error("lease should not be released initially")
	}
	lease.Release()
	if !lease.IsReleased() {
		t.Error("lease should be released after Release()")
	}
	// Double release should be safe
	lease.Release()
}

func TestProcessPoolMetrics(t *testing.T) {
	config := PoolConfig{MaxCapacity: 4, TTL: time.Minute, HealthInterval: time.Hour}
	pool := NewProcessPool(config, testFactory)
	defer pool.Close()

	key := PoolKey{ProjectPath: "/tmp/project", ProviderProfile: "claude"}
	lease, _ := pool.Acquire(key, "session-1")
	lease.Release()

	snap := pool.MetricsSnapshot()
	if snap.LeaseAcquireCount != 1 {
		t.Errorf("expected 1 acquire, got %d", snap.LeaseAcquireCount)
	}
	if snap.LeaseReleaseCount != 1 {
		t.Errorf("expected 1 release, got %d", snap.LeaseReleaseCount)
	}
}

func TestProcessPoolSize(t *testing.T) {
	config := PoolConfig{MaxCapacity: 4, TTL: time.Minute, HealthInterval: time.Hour}
	pool := NewProcessPool(config, testFactory)
	defer pool.Close()

	if pool.Size() != 0 {
		t.Errorf("expected size 0, got %d", pool.Size())
	}

	key := PoolKey{ProjectPath: "/tmp/project", ProviderProfile: "claude"}
	lease, _ := pool.Acquire(key, "session-1")
	defer lease.Release()

	if pool.Size() != 1 {
		t.Errorf("expected size 1, got %d", pool.Size())
	}
}

func TestProcessPoolActiveLeasesForSession(t *testing.T) {
	config := PoolConfig{MaxCapacity: 4, TTL: time.Minute, HealthInterval: time.Hour}
	pool := NewProcessPool(config, testFactory)
	defer pool.Close()

	key := PoolKey{ProjectPath: "/tmp/project", ProviderProfile: "claude"}
	lease, _ := pool.Acquire(key, "session-1")
	defer lease.Release()

	if pool.ActiveLeasesForSession("session-1") != 1 {
		t.Error("expected 1 active lease for session-1")
	}
	if pool.ActiveLeasesForSession("session-2") != 0 {
		t.Error("expected 0 active leases for session-2")
	}
}

func TestPooledProcess(t *testing.T) {
	p := NewPooledProcess(42, "claude", []string{"--flag"}, "/tmp")
	if p.PID() != 42 {
		t.Errorf("expected PID 42, got %d", p.PID())
	}
	if p.Command() != "claude" {
		t.Errorf("expected claude, got %s", p.Command())
	}
	if !p.IsAlive() {
		t.Error("process should be alive")
	}
	p.MarkDead()
	if p.IsAlive() {
		t.Error("process should be dead after MarkDead")
	}
}

func TestPoolKeyString(t *testing.T) {
	k := PoolKey{ProjectPath: "/tmp/project", ProviderProfile: "claude"}
	if k.String() != "/tmp/project|claude" {
		t.Errorf("unexpected key string: %s", k.String())
	}
}

func TestMetricsSnapshot(t *testing.T) {
	m := NewPoolMetrics()
	m.WarmHit()
	m.ColdStart()
	m.TTLEviction()
	m.LRUEviction()
	m.ZombieCleanup()

	snap := m.Snapshot(2, 1)
	if snap.WarmHitCount != 1 {
		t.Errorf("expected 1 warm hit, got %d", snap.WarmHitCount)
	}
	if snap.ColdStartCount != 1 {
		t.Errorf("expected 1 cold start, got %d", snap.ColdStartCount)
	}
	if snap.TTLEvictionCount != 1 {
		t.Errorf("expected 1 TTL eviction, got %d", snap.TTLEvictionCount)
	}
	if snap.LRUEvictionCount != 1 {
		t.Errorf("expected 1 LRU eviction, got %d", snap.LRUEvictionCount)
	}
	if snap.EvictionCount != 2 {
		t.Errorf("expected 2 total evictions, got %d", snap.EvictionCount)
	}
	if snap.ZombieCleanupCount != 1 {
		t.Errorf("expected 1 zombie cleanup, got %d", snap.ZombieCleanupCount)
	}
	if snap.PoolSize != 2 {
		t.Errorf("expected pool size 2, got %d", snap.PoolSize)
	}
	if snap.ActiveLeases != 1 {
		t.Errorf("expected 1 active lease, got %d", snap.ActiveLeases)
	}
}

func TestMetricsCollector(t *testing.T) {
	m := NewPoolMetrics()
	m.WarmHit()
	collector := NewMetricsCollector(func() MetricsSnapshot {
		return m.Snapshot(1, 0)
	})
	snap := collector.Collect()
	if snap.WarmHitCount != 1 {
		t.Errorf("expected 1 warm hit, got %d", snap.WarmHitCount)
	}
}
