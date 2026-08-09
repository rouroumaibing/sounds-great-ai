package ports

import (
	"context"
	"time"
)

// LeaseID uniquely identifies a custody lease.
type LeaseID string

// Lease represents a ball custody lease.
type Lease struct {
	ID         LeaseID    `json:"id"`
	Subject    string     `json:"subject"`
	Holder     string     `json:"holder"`
	Generation int        `json:"generation"`
	AcquiredAt time.Time  `json:"acquired_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

// ICustodyStore is the port for custody lease storage.
type ICustodyStore interface {
	Acquire(ctx context.Context, lease Lease) (LeaseID, error)
	Release(ctx context.Context, id LeaseID) error
	CurrentHolder(ctx context.Context, subject string) (Lease, error)
	CAS(ctx context.Context, id LeaseID, expectedGen int, newGen int) error
}

// ICustodyService is the port for the custody service.
type ICustodyService interface {
	Acquire(ctx context.Context, subject string) (Lease, error)
	Release(ctx context.Context, lease Lease) error
	IsStale(ctx context.Context, lease Lease) bool
}
