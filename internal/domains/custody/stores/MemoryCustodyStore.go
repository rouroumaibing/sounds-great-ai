package stores

import (
	"context"
	"fmt"
	"sync"
	"time"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
)

// MemoryCustodyStore is an in-memory implementation of ICustodyStore.
type MemoryCustodyStore struct {
	leases map[custodyPorts.LeaseID]custodyPorts.Lease
	bySubject map[string]custodyPorts.LeaseID
	mu     sync.RWMutex
}

// NewMemoryCustodyStore creates a new MemoryCustodyStore.
func NewMemoryCustodyStore() *MemoryCustodyStore {
	return &MemoryCustodyStore{
		leases:   make(map[custodyPorts.LeaseID]custodyPorts.Lease),
		bySubject: make(map[string]custodyPorts.LeaseID),
	}
}

func (s *MemoryCustodyStore) Acquire(ctx context.Context, lease custodyPorts.Lease) (custodyPorts.LeaseID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, ok := s.bySubject[lease.Subject]; ok {
		existing := s.leases[existingID]
		if time.Now().Before(existing.ExpiresAt) {
			return "", fmt.Errorf("subject %s already has active lease %s", lease.Subject, existingID)
		}
		delete(s.leases, existingID)
	}

	id := custodyPorts.LeaseID(fmt.Sprintf("lease-%d", time.Now().UnixNano()))
	lease.ID = id
	s.leases[id] = lease
	s.bySubject[lease.Subject] = id
	return id, nil
}

func (s *MemoryCustodyStore) Release(ctx context.Context, id custodyPorts.LeaseID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lease, ok := s.leases[id]
	if !ok {
		return fmt.Errorf("lease not found: %s", id)
	}
	delete(s.leases, id)
	delete(s.bySubject, lease.Subject)
	return nil
}

func (s *MemoryCustodyStore) CurrentHolder(ctx context.Context, subject string) (custodyPorts.Lease, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.bySubject[subject]
	if !ok {
		return custodyPorts.Lease{}, fmt.Errorf("no lease for subject: %s", subject)
	}
	return s.leases[id], nil
}

func (s *MemoryCustodyStore) CAS(ctx context.Context, id custodyPorts.LeaseID, expectedGen int, newGen int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lease, ok := s.leases[id]
	if !ok {
		return fmt.Errorf("lease not found: %s", id)
	}
	if lease.Generation != expectedGen {
		return fmt.Errorf("generation mismatch: expected %d, got %d", expectedGen, lease.Generation)
	}
	lease.Generation = newGen
	s.leases[id] = lease
	return nil
}
