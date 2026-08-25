package jobs

import (
	"sync"
	"time"
)

// CallbackAuth is a credential handed to a long-running job so it can report
// progress over a callback channel (F298). It is bound to a single job and has
// a bounded lifetime. A tombstone permanently invalidates it (fail-closed):
// once tombstoned, even a non-expired token must be rejected.
type CallbackAuth struct {
	mu sync.Mutex

	Token     string
	BoundJobID string
	ExpiresAt time.Time
	Tombstoned bool
}

// NewCallbackAuth creates a callback credential bound to jobID, valid until
// expiresAt.
func NewCallbackAuth(token, jobID string, expiresAt time.Time) *CallbackAuth {
	return &CallbackAuth{Token: token, BoundJobID: jobID, ExpiresAt: expiresAt}
}

// Valid reports whether the credential may be used at now. It is fail-closed:
// a tombstoned or expired credential is always invalid.
func (c *CallbackAuth) Valid(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Tombstoned {
		return false
	}
	return !now.After(c.ExpiresAt)
}

// Tombstone permanently revokes the credential. Idempotent.
func (c *CallbackAuth) Tombstone() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Tombstoned = true
}

// IsTombstoned reports the tombstone state.
func (c *CallbackAuth) IsTombstoned() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Tombstoned
}

// CallbackAuthStore holds callback credentials keyed by job id. It is
// goroutine-safe and supports the "callback-auth lifetime" gate: a credential
// is looked up only while valid.
type CallbackAuthStore struct {
	mu      sync.Mutex
	byJobID map[string]*CallbackAuth
}

// NewCallbackAuthStore creates an empty store.
func NewCallbackAuthStore() *CallbackAuthStore {
	return &CallbackAuthStore{byJobID: make(map[string]*CallbackAuth)}
}

// Put registers a credential for a job.
func (s *CallbackAuthStore) Put(c *CallbackAuth) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byJobID[c.BoundJobID] = c
}

// Lookup returns the credential for a job only if it is currently valid at now.
// A tombstoned/expired credential returns (nil, false) — never a usable token.
func (s *CallbackAuthStore) Lookup(jobID string, now time.Time) (*CallbackAuth, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.byJobID[jobID]
	if !ok {
		return nil, false
	}
	if !c.Valid(now) {
		return nil, false
	}
	return c, true
}
