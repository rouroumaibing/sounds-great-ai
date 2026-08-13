package stores

import (
	"context"
	"sync"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
)

// MemoryBallLedgerStore is an in-memory append-only store for ball custody
// events. P0 foundation; persistence (SQLite) is added in the brief/trail phase
// (P4) so the ledger survives restarts for audit/brief use.
type MemoryBallLedgerStore struct {
	mu     sync.Mutex
	seq    int64
	events map[string][]custodyPorts.BallEvent
}

// NewMemoryBallLedgerStore creates an empty in-memory ledger store.
func NewMemoryBallLedgerStore() *MemoryBallLedgerStore {
	return &MemoryBallLedgerStore{events: make(map[string][]custodyPorts.BallEvent)}
}

// Append assigns a monotonic Seq and stores the event.
func (s *MemoryBallLedgerStore) Append(_ context.Context, ev custodyPorts.BallEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	ev.Seq = s.seq
	s.events[ev.ThreadID] = append(s.events[ev.ThreadID], ev)
	return nil
}

// GetEvents returns events for a thread in append (Seq) order.
func (s *MemoryBallLedgerStore) GetEvents(_ context.Context, threadID string) ([]custodyPorts.BallEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]custodyPorts.BallEvent, len(s.events[threadID]))
	copy(out, s.events[threadID])
	return out, nil
}

// ListThreadIDs returns every thread that has at least one ledger event.
func (s *MemoryBallLedgerStore) ListThreadIDs(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.events))
	for id := range s.events {
		ids = append(ids, id)
	}
	return ids, nil
}
