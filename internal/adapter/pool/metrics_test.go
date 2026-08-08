package pool

import "testing"

func TestPoolMetrics_Counters(t *testing.T) {
	m := NewPoolMetrics()
	m.WarmHit()
	m.WarmHit()
	m.ColdStart()
	m.LeaseAcquire()
	m.LeaseRelease()
	snap := m.Snapshot(5, 2)
	if snap.WarmHitCount != 2 {
		t.Errorf("WarmHitCount = %d, want 2", snap.WarmHitCount)
	}
	if snap.ColdStartCount != 1 {
		t.Errorf("ColdStartCount = %d, want 1", snap.ColdStartCount)
	}
	if snap.PoolSize != 5 {
		t.Errorf("PoolSize = %d, want 5", snap.PoolSize)
	}
	if snap.ActiveLeases != 2 {
		t.Errorf("ActiveLeases = %d, want 2", snap.ActiveLeases)
	}
}

func TestPoolMetrics_TTLAndLRU(t *testing.T) {
	m := NewPoolMetrics()
	m.TTLEviction()
	m.LRUEviction()
	snap := m.Snapshot(0, 0)
	if snap.TTLEvictionCount != 1 {
		t.Errorf("TTLEvictionCount = %d, want 1", snap.TTLEvictionCount)
	}
	if snap.LRUEvictionCount != 1 {
		t.Errorf("LRUEvictionCount = %d, want 1", snap.LRUEvictionCount)
	}
	if snap.EvictionCount != 2 {
		t.Errorf("EvictionCount = %d, want 2", snap.EvictionCount)
	}
}

func TestPoolMetrics_Reset(t *testing.T) {
	m := NewPoolMetrics()
	m.WarmHit()
	m.ColdStart()
	m.Reset()
	snap := m.Snapshot(0, 0)
	if snap.WarmHitCount != 0 || snap.ColdStartCount != 0 {
		t.Error("Reset did not zero counters")
	}
}

func TestMetricsCollector_WithSource(t *testing.T) {
	m := NewPoolMetrics()
	m.WarmHit()
	c := NewMetricsCollector(func() MetricsSnapshot {
		return m.Snapshot(1, 0)
	})
	snap := c.Collect()
	if snap.WarmHitCount != 1 {
		t.Errorf("Collect WarmHitCount = %d, want 1", snap.WarmHitCount)
	}
}

func TestMetricsCollector_NilSource(t *testing.T) {
	c := NewMetricsCollector(nil)
	snap := c.Collect()
	if snap.WarmHitCount != 0 {
		t.Errorf("nil source WarmHitCount = %d, want 0", snap.WarmHitCount)
	}
}
