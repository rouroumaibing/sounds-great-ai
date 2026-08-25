package ports

import (
	"context"
	"time"
)

// Decision represents an approval decision.
type Decision string

const (
	DecisionApproved Decision = "approved"
	DecisionRejected Decision = "rejected"
	DecisionExpired  Decision = "expired"
)

// ApprovalItem represents an item pending approval.
type ApprovalItem struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Detail    string                 `json:"detail"`
	Status    string                 `json:"status"`
	Payload   map[string]any         `json:"payload,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	ResolvedAt *time.Time            `json:"resolved_at,omitempty"`

	// RequestAnchor is the audit hash captured at submit time (F246 dual-anchor).
	RequestAnchor string `json:"request_anchor,omitempty"`
	// DecisionAnchor is the audit hash captured at resolve time (F246 dual-anchor).
	DecisionAnchor string `json:"decision_anchor,omitempty"`
	// RejectionReason carries the operator's rationale back to the requester on
	// a reject decision (F281 backflow).
	RejectionReason string `json:"rejection_reason,omitempty"`
}

// IApprovalAdapter is the port for approval adapters (per-feature).
type IApprovalAdapter interface {
	FeatureID() string
	ListPending(ctx context.Context, userID string) ([]ApprovalItem, error)
	Resolve(ctx context.Context, id string, decision Decision) error
}

// IApprovalStore is the port for approval item storage.
type IApprovalStore interface {
	Save(ctx context.Context, item ApprovalItem) error
	Get(ctx context.Context, id string) (ApprovalItem, error)
	ListPending(ctx context.Context, userID string) ([]ApprovalItem, error)
	ListSettled(ctx context.Context, userID string, limit int) ([]ApprovalItem, error)
}
