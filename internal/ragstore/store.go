// internal/ragstore/store.go
package ragstore

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// VectorStore is the interface all backends must satisfy.
// Three implementations: MemoryStore / SQLiteStore / EinoStore.
type VectorStore interface {
	// Upsert inserts or updates documents (deduped by ID).
	// Implementation calls Embedder to generate vectors and persists them.
	Upsert(ctx context.Context, docs []*schema.Document) error

	// Search retrieves top_k relevant documents by query text.
	// Implementation calls Embedder to generate query vector and searches.
	Search(ctx context.Context, query string, opts SearchOpts) ([]*schema.Document, error)

	// Delete removes documents by ID.
	Delete(ctx context.Context, ids []string) error

	// Close releases backend resources.
	Close() error

	// ListAll returns all documents (for migration; excludes vectors to save memory)
	ListAll(ctx context.Context) ([]*schema.Document, error)

	// GetByID retrieves a single document by ID (for migration resumption)
	GetByID(ctx context.Context, id string) (*schema.Document, error)

	// DropAll deletes all data (for 30-day cleanup)
	DropAll(ctx context.Context) error
}

// SearchOpts controls search behavior.
type SearchOpts struct {
	TopK      int     // default 5
	Threshold float64 // similarity floor, default 0.0 (no filter)
	Namespace string  // namespace filter, empty = all
}

// BackendType identifies a VectorStore backend.
type BackendType string

const (
	BackendMemory BackendType = "memory"
	BackendSQLite BackendType = "sqlite"
	BackendEino   BackendType = "eino"
)
