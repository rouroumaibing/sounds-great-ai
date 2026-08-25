package auth

import (
	"errors"
	"sync"
	"time"
)

// AgentKey is a persistent credential bound to an agent (dog). It supports
// rotation (issue a new key deriving from the old) and revocation (fail-closed
// deny). Persistence is via AgentKeyStore; production backs this with Redis
// (per roadmap P1-D, F178) but the interface keeps it interchangeable.
type AgentKey struct {
	Key        string
	AgentID    string
	CreatedAt  time.Time
	Revoked    bool
	RotatedFrom string
	// Metadata carries the thread/Memory back-reference written back across
	// invocations (F178): e.g. {"thread_id": "...", "memory_ref": "..."}.
	Metadata map[string]string
}

// AgentKeyStore persists agent keys. Implementations must be safe for
// concurrent use. A Redis-backed store satisfies this interface in production.
type AgentKeyStore interface {
	Save(k AgentKey) error
	Load(key string) (AgentKey, error)
	ListForAgent(agentID string) ([]AgentKey, error)
}

// ErrAgentKeyNotFound is returned when a key is unknown.
var ErrAgentKeyNotFound = errors.New("auth: agent key not found")

// ErrAgentKeyRevoked is returned when a key has been revoked (fail-closed).
var ErrAgentKeyRevoked = errors.New("auth: agent key revoked (fail-closed)")

// InMemoryAgentKeyStore is a goroutine-safe AgentKeyStore for tests/dev.
type InMemoryAgentKeyStore struct {
	mu    sync.Mutex
	byKey map[string]AgentKey
	byAgent map[string][]string
}

// NewInMemoryAgentKeyStore creates an empty store.
func NewInMemoryAgentKeyStore() *InMemoryAgentKeyStore {
	return &InMemoryAgentKeyStore{
		byKey:   make(map[string]AgentKey),
		byAgent: make(map[string][]string),
	}
}

// Save persists a key.
func (s *InMemoryAgentKeyStore) Save(k AgentKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byKey[k.Key] = k
	s.byAgent[k.AgentID] = appendUnique(s.byAgent[k.AgentID], k.Key)
	return nil
}

// Load returns a key by its string value.
func (s *InMemoryAgentKeyStore) Load(key string) (AgentKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.byKey[key]
	if !ok {
		return AgentKey{}, ErrAgentKeyNotFound
	}
	return k, nil
}

// ListForAgent returns all keys (including revoked) for an agent.
func (s *InMemoryAgentKeyStore) ListForAgent(agentID string) ([]AgentKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []AgentKey
	for _, key := range s.byAgent[agentID] {
		out = append(out, s.byKey[key])
	}
	return out, nil
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// AgentKeyRegistry issues, rotates, and revokes agent keys, persisting them via
// the configured store.
type AgentKeyRegistry struct {
	store AgentKeyStore
	mu    sync.Mutex
}

// NewAgentKeyRegistry creates a registry over a store.
func NewAgentKeyRegistry(store AgentKeyStore) *AgentKeyRegistry {
	return &AgentKeyRegistry{store: store}
}

// Issue mints a fresh key for an agent and persists it (F178 write-back).
func (r *AgentKeyRegistry) Issue(agentID, key string) (*AgentKey, error) {
	k := AgentKey{Key: key, AgentID: agentID, CreatedAt: time.Now()}
	if err := r.store.Save(k); err != nil {
		return nil, err
	}
	return &k, nil
}

// Rotate revokes the old key and issues a new one derived from it. The new key
// records RotatedFrom for audit lineage.
func (r *AgentKeyRegistry) Rotate(oldKey, newKey string) (*AgentKey, error) {
	old, err := r.store.Load(oldKey)
	if err != nil {
		return nil, err
	}
	// revoke old
	old.Revoked = true
	if err := r.store.Save(old); err != nil {
		return nil, err
	}
	nu := AgentKey{Key: newKey, AgentID: old.AgentID, CreatedAt: time.Now(), RotatedFrom: oldKey}
	if err := r.store.Save(nu); err != nil {
		return nil, err
	}
	return &nu, nil
}

// Revoke invalidates a key (fail-closed: subsequent Validate denies).
func (r *AgentKeyRegistry) Revoke(key string) error {
	k, err := r.store.Load(key)
	if err != nil {
		return err
	}
	k.Revoked = true
	return r.store.Save(k)
}

// Validate reports whether a key is usable. Fail-closed: unknown or revoked
// keys deny.
func (r *AgentKeyRegistry) Validate(key string) (bool, error) {
	k, err := r.store.Load(key)
	if err != nil {
		return false, ErrAgentKeyNotFound
	}
	if k.Revoked {
		return false, ErrAgentKeyRevoked
	}
	return true, nil
}
