package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	approvalPorts "sounds-great-ai/internal/domains/approval/ports"
)

// ApprovalIngress manages the approval queue: submitting items, resolving decisions,
// and registering per-feature adapters. Replaces the synchronous blocking
// RequestApproval() with an async queue pattern.
type ApprovalIngress struct {
	store    approvalPorts.IApprovalStore
	adapters map[string]approvalPorts.IApprovalAdapter
	notify   NotificationFunc
	mu       sync.RWMutex
}

// NotificationFunc is called when a new approval item is submitted.
type NotificationFunc func(item approvalPorts.ApprovalItem)

// NewApprovalIngress creates a new ApprovalIngress.
func NewApprovalIngress(store approvalPorts.IApprovalStore, notify NotificationFunc) *ApprovalIngress {
	return &ApprovalIngress{
		store:    store,
		adapters: make(map[string]approvalPorts.IApprovalAdapter),
		notify:   notify,
	}
}

// RegisterAdapter registers a per-feature approval adapter.
func (a *ApprovalIngress) RegisterAdapter(adapter approvalPorts.IApprovalAdapter) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.adapters[adapter.FeatureID()] = adapter
}

// Submit creates a new approval item in the queue and notifies.
func (a *ApprovalIngress) Submit(ctx context.Context, item approvalPorts.ApprovalItem) (string, error) {
	if item.ID == "" {
		item.ID = fmt.Sprintf("approval-%d", time.Now().UnixNano())
	}
	item.Status = "pending"
	item.CreatedAt = time.Now()

	if a.store != nil {
		if err := a.store.Save(ctx, item); err != nil {
			return "", fmt.Errorf("save approval item: %w", err)
		}
	}

	if a.notify != nil {
		go a.notify(item)
	}

	log.Printf("Approval submitted: id=%s type=%s", item.ID, item.Type)
	return item.ID, nil
}

// Resolve resolves a pending approval item with the given decision.
func (a *ApprovalIngress) Resolve(ctx context.Context, id string, decision approvalPorts.Decision) error {
	if a.store == nil {
		return fmt.Errorf("no approval store configured")
	}

	item, err := a.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get approval item: %w", err)
	}
	if item.Status != "pending" {
		return fmt.Errorf("item %s is not pending (status=%s)", id, item.Status)
	}

	now := time.Now()
	item.Status = string(decision)
	item.ResolvedAt = &now

	if err := a.store.Save(ctx, item); err != nil {
		return fmt.Errorf("save resolved item: %w", err)
	}

	a.mu.RLock()
	adapter, ok := a.adapters[item.Type]
	a.mu.RUnlock()
	if ok {
		if err := adapter.Resolve(ctx, id, decision); err != nil {
			log.Printf("warning: adapter resolve failed for %s: %v", item.Type, err)
		}
	}

	log.Printf("Approval resolved: id=%s decision=%s", id, decision)
	return nil
}

// ListPending returns all pending approval items for a user.
func (a *ApprovalIngress) ListPending(ctx context.Context, userID string) ([]approvalPorts.ApprovalItem, error) {
	if a.store == nil {
		return nil, nil
	}
	return a.store.ListPending(ctx, userID)
}

// ListSettled returns resolved approval items for a user.
func (a *ApprovalIngress) ListSettled(ctx context.Context, userID string, limit int) ([]approvalPorts.ApprovalItem, error) {
	if a.store == nil {
		return nil, nil
	}
	return a.store.ListSettled(ctx, userID, limit)
}
