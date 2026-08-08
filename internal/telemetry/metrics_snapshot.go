package telemetry

import (
	"context"
	"sync"
	"time"
)

// MetricsSnapshot is a point-in-time Prometheus text format snapshot.
type MetricsSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Text      string    `json:"text"`
}

// SnapshotStore periodically samples Prometheus metrics, retaining the last maxSize snapshots.
type SnapshotStore struct {
	snapshots []MetricsSnapshot
	maxSize   int
	interval  time.Duration
	mu        sync.RWMutex
	collect   func() string
}

// NewSnapshotStore creates a SnapshotStore. collect returns current Prometheus text.
func NewSnapshotStore(maxSize int, interval time.Duration, collect func() string) *SnapshotStore {
	if maxSize <= 0 {
		maxSize = 720
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if collect == nil {
		collect = func() string { return "" }
	}
	return &SnapshotStore{snapshots: make([]MetricsSnapshot, 0, maxSize), maxSize: maxSize, interval: interval, collect: collect}
}

// Start launches a background goroutine that samples every interval. Stops on ctx cancel.
func (s *SnapshotStore) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snap := MetricsSnapshot{Timestamp: time.Now(), Text: s.collect()}
				s.mu.Lock()
				if len(s.snapshots) >= s.maxSize {
					s.snapshots = s.snapshots[1:]
				}
				s.snapshots = append(s.snapshots, snap)
				s.mu.Unlock()
			}
		}
	}()
}

// History returns all snapshots after since, in chronological order. Zero since returns all.
func (s *SnapshotStore) History(since time.Time) []MetricsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]MetricsSnapshot, 0, len(s.snapshots))
	for _, snap := range s.snapshots {
		if since.IsZero() || !snap.Timestamp.Before(since) {
			result = append(result, snap)
		}
	}
	return result
}

// SnapshotCount returns the current number of snapshots (for testing).
func (s *SnapshotStore) SnapshotCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshots)
}
