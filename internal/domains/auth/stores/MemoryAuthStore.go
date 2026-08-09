package stores

import (
	"context"
	"sync"
	"time"

	authPorts "sounds-great-ai/internal/domains/auth/ports"
)

// MemoryAuthStore is an in-memory implementation of IAuthRuleStore and IAuthAuditStore.
// Useful for development and testing.
type MemoryAuthStore struct {
	rules  map[string]authPorts.Rule
	audit []authPorts.AuditEntry
	mu    sync.RWMutex
}

// NewMemoryAuthStore creates a new MemoryAuthStore.
func NewMemoryAuthStore() *MemoryAuthStore {
	return &MemoryAuthStore{
		rules: make(map[string]authPorts.Rule),
	}
}

func (s *MemoryAuthStore) Match(ctx context.Context, action string) (authPorts.Rule, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rule := range s.rules {
		if rule.Action == action || rule.Action == "*" {
			return rule, true, nil
		}
	}
	return authPorts.Rule{}, false, nil
}

func (s *MemoryAuthStore) List(ctx context.Context) ([]authPorts.Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]authPorts.Rule, 0, len(s.rules))
	for _, r := range s.rules {
		result = append(result, r)
	}
	return result, nil
}

func (s *MemoryAuthStore) Add(ctx context.Context, rule authPorts.Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rule.CreatedAt == 0 {
		rule.CreatedAt = time.Now().Unix()
	}
	s.rules[rule.ID] = rule
	return nil
}

func (s *MemoryAuthStore) Remove(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rules, id)
	return nil
}

func (s *MemoryAuthStore) Record(ctx context.Context, entry authPorts.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audit = append(s.audit, entry)
	return nil
}

func (s *MemoryAuthStore) Query(ctx context.Context, filter authPorts.AuditFilter) ([]authPorts.AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]authPorts.AuditEntry, 0)
	for _, e := range s.audit {
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		if filter.Decision != "" && e.Decision != filter.Decision {
			continue
		}
		result = append(result, e)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}
