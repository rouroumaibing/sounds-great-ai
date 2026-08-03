// internal/ragstore/eino_store.go
package ragstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// ErrUnsupported is returned by EinoStore for operations the Eino
// Indexer/Retriever interface doesn't support (Delete, ListAll, GetByID, DropAll).
var ErrUnsupported = errors.New("operation not supported by eino backend")

// EinoStore delegates storage and retrieval to Eino's indexer.Indexer and
// retriever.Retriever interfaces. It does not implement its own vector store;
// the backing index (e.g. Milvus, Redis, VolcVikingDB) is configured by the
// caller-supplied idx/ret implementations.
type EinoStore struct {
	indexer   indexer.Indexer
	retriever retriever.Retriever
}

// NewEinoStore wraps Eino Indexer/Retriever into a VectorStore. Both idx and
// ret are required; the Eino backend handles embedding internally via the
// supplied Indexer/Retriever, so no embedder is needed here.
func NewEinoStore(idx indexer.Indexer, ret retriever.Retriever) (*EinoStore, error) {
	if idx == nil {
		return nil, fmt.Errorf("eino store: indexer is required")
	}
	if ret == nil {
		return nil, fmt.Errorf("eino store: retriever is required")
	}
	return &EinoStore{indexer: idx, retriever: ret}, nil
}

// Upsert delegates to indexer.Indexer.Store. The IDs assigned by the backend
// are discarded — callers identify documents by schema.Document.ID.
func (s *EinoStore) Upsert(ctx context.Context, docs []*schema.Document) error {
	_, err := s.indexer.Store(ctx, docs)
	return err
}

// Search delegates to retriever.Retriever.Retrieve, forwarding TopK and
// ScoreThreshold via Eino options. Namespace filtering is applied in the Go
// layer because Eino's Retriever interface has no native namespace option.
func (s *EinoStore) Search(ctx context.Context, query string, opts SearchOpts) ([]*schema.Document, error) {
	einoOpts := []retriever.Option{
		retriever.WithTopK(opts.TopK),
		retriever.WithScoreThreshold(opts.Threshold),
	}
	docs, err := s.retriever.Retrieve(ctx, query, einoOpts...)
	if err != nil {
		return nil, err
	}
	if opts.Namespace == "" {
		return docs, nil
	}
	var filtered []*schema.Document
	for _, d := range docs {
		ns, _ := d.MetaData["namespace"].(string)
		if ns == opts.Namespace {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

// Delete returns ErrUnsupported — Eino's Indexer/Retriever interfaces have no
// native delete operation. Callers needing deletion should use MemoryStore or
// SQLiteStore, or wrap a backend-specific deleter outside this package.
func (s *EinoStore) Delete(ctx context.Context, ids []string) error {
	return ErrUnsupported
}

// Close is a no-op; Eino Indexer/Retriever lifecycles are managed by the
// caller that constructed them.
func (s *EinoStore) Close() error { return nil }

// ListAll returns ErrUnsupported — Eino's Indexer/Retriever interfaces have no
// native enumerate operation.
func (s *EinoStore) ListAll(ctx context.Context) ([]*schema.Document, error) {
	return nil, ErrUnsupported
}

// GetByID returns ErrUnsupported — Eino's Indexer/Retriever interfaces have no
// native get-by-id operation.
func (s *EinoStore) GetByID(ctx context.Context, id string) (*schema.Document, error) {
	return nil, ErrUnsupported
}

// DropAll returns ErrUnsupported — Eino's Indexer/Retriever interfaces have no
// native delete-all operation.
func (s *EinoStore) DropAll(ctx context.Context) error {
	return ErrUnsupported
}
