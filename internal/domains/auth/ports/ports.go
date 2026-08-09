package ports

import (
	"context"
	"net/http"
	"time"
)

// Rule represents an authorization rule.
type Rule struct {
	ID        string   `json:"id"`
	Action    string   `json:"action"`
	Methods   []string `json:"methods"`
	Paths     []string `json:"paths"`
	Decision  string   `json:"decision"`
	Priority  int      `json:"priority"`
	CreatedAt int64    `json:"created_at"`
}

// AuditEntry represents an audit log entry.
type AuditEntry struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Decision  string    `json:"decision"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// AuditFilter filters audit log queries.
type AuditFilter struct {
	Action   string
	Decision string
	From     time.Time
	To       time.Time
	Limit    int
}

// PermissionRequest represents a permission check request.
type PermissionRequest struct {
	Action string
	Method string
	Path   string
}

// PermissionResponse represents the result of a permission check.
type PermissionResponse struct {
	Granted bool
	Reason  string
	Rule    *Rule
}

// IAuthRuleStore is the port for authorization rule storage.
type IAuthRuleStore interface {
	Match(ctx context.Context, action string) (Rule, bool, error)
	List(ctx context.Context) ([]Rule, error)
	Add(ctx context.Context, rule Rule) error
	Remove(ctx context.Context, id string) error
}

// IAuthAuditStore is the port for audit log storage.
type IAuthAuditStore interface {
	Record(ctx context.Context, entry AuditEntry) error
	Query(ctx context.Context, filter AuditFilter) ([]AuditEntry, error)
}

// IPendingRequestStore is the port for pending permission requests.
type IPendingRequestStore interface {
	Save(ctx context.Context, req PendingRequest) error
	Get(ctx context.Context, id string) (PendingRequest, error)
	Resolve(ctx context.Context, id string, response string) error
}

// PendingRequest represents a permission request awaiting operator decision.
type PendingRequest struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	Context   string    `json:"context"`
	CreatedAt time.Time `json:"created_at"`
}

// IAuthorizationManager is the port for the authorization manager.
type IAuthorizationManager interface {
	Check(ctx context.Context, req PermissionRequest) (PermissionResponse, error)
	Middleware(next http.Handler) http.Handler
}
