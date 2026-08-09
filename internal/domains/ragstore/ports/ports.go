package ports

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// SearchOpts controls search behavior.
type SearchOpts struct {
	TopK      int
	Threshold float64
	Namespace string
}

// BackendType identifies a VectorStore backend.
type BackendType string

const (
	BackendMemory BackendType = "memory"
	BackendSQLite BackendType = "sqlite"
	BackendEino   BackendType = "eino"
)

// IVectorStore is the port for vector storage backends.
type IVectorStore interface {
	Upsert(ctx context.Context, docs []*schema.Document) error
	Search(ctx context.Context, query string, opts SearchOpts) ([]*schema.Document, error)
	Delete(ctx context.Context, ids []string) error
	Close() error
	ListAll(ctx context.Context) ([]*schema.Document, error)
	GetByID(ctx context.Context, id string) (*schema.Document, error)
	DropAll(ctx context.Context) error
}

// IRAGService is the port for the RAG service.
type IRAGService interface {
	Index(ctx context.Context, docs []*schema.Document) error
	Retrieve(ctx context.Context, query string, opts SearchOpts) ([]*schema.Document, error)
	SwitchBackend(ctx context.Context, backend BackendType) error
	ActiveBackend() BackendType
}
