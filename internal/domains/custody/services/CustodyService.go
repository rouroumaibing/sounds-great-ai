package services

import (
	"context"
	"fmt"
	"time"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
)

// CustodyService manages ball custody leases for concurrent operation protection.
// Aligned with clowder-ai's ball-custody domain — prevents concurrent
// execution conflicts via compare-and-swap lease generations.
type CustodyService struct {
	store custodyPorts.ICustodyStore
	ttl   time.Duration
}

// NewCustodyService creates a new CustodyService with the given TTL.
func NewCustodyService(store custodyPorts.ICustodyStore, ttl time.Duration) *CustodyService {
	return &CustodyService{
		store: store,
		ttl:   ttl,
	}
}

// Acquire attempts to acquire a custody lease for the given subject.
func (s *CustodyService) Acquire(ctx context.Context, subject string) (custodyPorts.Lease, error) {
	now := time.Now()
	lease := custodyPorts.Lease{
		Subject:    subject,
		Holder:     subject,
		Generation: 1,
		AcquiredAt: now,
		ExpiresAt:  now.Add(s.ttl),
	}

	id, err := s.store.Acquire(ctx, lease)
	if err != nil {
		return custodyPorts.Lease{}, fmt.Errorf("acquire lease: %w", err)
	}
	lease.ID = id
	return lease, nil
}

// Release releases a custody lease.
func (s *CustodyService) Release(ctx context.Context, lease custodyPorts.Lease) error {
	if err := s.store.Release(ctx, lease.ID); err != nil {
		return fmt.Errorf("release lease: %w", err)
	}
	return nil
}

// IsStale checks if a lease has expired.
func (s *CustodyService) IsStale(ctx context.Context, lease custodyPorts.Lease) bool {
	return time.Now().After(lease.ExpiresAt)
}

// CurrentHolder returns the current lease holder for a subject.
func (s *CustodyService) CurrentHolder(ctx context.Context, subject string) (custodyPorts.Lease, error) {
	return s.store.CurrentHolder(ctx, subject)
}

// Refresh extends the lease expiration time.
func (s *CustodyService) Refresh(ctx context.Context, lease custodyPorts.Lease) error {
	newExpiry := time.Now().Add(s.ttl)
	updatedLease := lease
	updatedLease.ExpiresAt = newExpiry
	updatedLease.Generation = lease.Generation + 1
	return s.store.CAS(ctx, lease.ID, lease.Generation, updatedLease.Generation)
}
