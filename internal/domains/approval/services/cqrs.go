package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"

	approvalPorts "sounds-great-ai/internal/domains/approval/ports"
)

// Anchor computes a stable hash over the given value, used for the dual-anchor
// audit trail (F246: request anchor + decision anchor).
func Anchor(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// CQRSProjection is a materialized read-model of approval items, maintained
// separately from the write store (F246: CQRS). It is updated on every
// submit/resolve and supports status-filtered queries without scanning the
// write store.
type CQRSProjection struct {
	mu       sync.RWMutex
	byStatus map[string]map[string]approvalPorts.ApprovalItem
}

// NewCQRSProjection creates an empty projection.
func NewCQRSProjection() *CQRSProjection {
	return &CQRSProjection{byStatus: make(map[string]map[string]approvalPorts.ApprovalItem)}
}

// Apply upserts an item into the projection, removing it from any prior status
// bucket so each item lives in exactly one status partition.
func (p *CQRSProjection) Apply(item approvalPorts.ApprovalItem) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for st, m := range p.byStatus {
		delete(m, item.ID)
		if len(m) == 0 {
			delete(p.byStatus, st)
		}
	}
	if p.byStatus[item.Status] == nil {
		p.byStatus[item.Status] = make(map[string]approvalPorts.ApprovalItem)
	}
	p.byStatus[item.Status][item.ID] = item
}

// ByStatus returns all items currently in the given status, sorted by id.
func (p *CQRSProjection) ByStatus(status string) []approvalPorts.ApprovalItem {
	p.mu.RLock()
	defer p.mu.RUnlock()
	m := p.byStatus[status]
	out := make([]approvalPorts.ApprovalItem, 0, len(m))
	for _, it := range m {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
