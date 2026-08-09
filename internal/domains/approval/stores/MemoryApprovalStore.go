package stores

import (
	"context"
	"fmt"
	"sync"

	approvalPorts "sounds-great-ai/internal/domains/approval/ports"
)

// MemoryApprovalStore is an in-memory implementation of IApprovalStore.
type MemoryApprovalStore struct {
	items map[string]approvalPorts.ApprovalItem
	mu    sync.RWMutex
}

// NewMemoryApprovalStore creates a new MemoryApprovalStore.
func NewMemoryApprovalStore() *MemoryApprovalStore {
	return &MemoryApprovalStore{
		items: make(map[string]approvalPorts.ApprovalItem),
	}
}

func (s *MemoryApprovalStore) Save(ctx context.Context, item approvalPorts.ApprovalItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *MemoryApprovalStore) Get(ctx context.Context, id string) (approvalPorts.ApprovalItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	if !ok {
		return approvalPorts.ApprovalItem{}, fmt.Errorf("not found: %s", id)
	}
	return item, nil
}

func (s *MemoryApprovalStore) ListPending(ctx context.Context, userID string) ([]approvalPorts.ApprovalItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]approvalPorts.ApprovalItem, 0)
	for _, item := range s.items {
		if item.Status == "pending" {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *MemoryApprovalStore) ListSettled(ctx context.Context, userID string, limit int) ([]approvalPorts.ApprovalItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]approvalPorts.ApprovalItem, 0)
	for _, item := range s.items {
		if item.Status != "pending" {
			result = append(result, item)
		}
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}
