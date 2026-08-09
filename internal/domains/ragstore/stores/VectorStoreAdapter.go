package stores

import (
	"context"

	"github.com/cloudwego/eino/schema"

	ragPorts "sounds-great-ai/internal/domains/ragstore/ports"
	"sounds-great-ai/internal/ragstore"
)

// VectorStoreAdapter wraps an existing ragstore.VectorStore to implement
// the domain ports.IVectorStore interface.
type VectorStoreAdapter struct {
	inner ragstore.VectorStore
}

// NewVectorStoreAdapter creates a new VectorStoreAdapter.
func NewVectorStoreAdapter(inner ragstore.VectorStore) *VectorStoreAdapter {
	return &VectorStoreAdapter{inner: inner}
}

func (a *VectorStoreAdapter) Upsert(ctx context.Context, docs []*schema.Document) error {
	return a.inner.Upsert(ctx, docs)
}

func (a *VectorStoreAdapter) Search(ctx context.Context, query string, opts ragPorts.SearchOpts) ([]*schema.Document, error) {
	return a.inner.Search(ctx, query, ragstore.SearchOpts{
		TopK:      opts.TopK,
		Threshold: opts.Threshold,
		Namespace: opts.Namespace,
	})
}

func (a *VectorStoreAdapter) Delete(ctx context.Context, ids []string) error {
	return a.inner.Delete(ctx, ids)
}

func (a *VectorStoreAdapter) Close() error {
	return a.inner.Close()
}

func (a *VectorStoreAdapter) ListAll(ctx context.Context) ([]*schema.Document, error) {
	return a.inner.ListAll(ctx)
}

func (a *VectorStoreAdapter) GetByID(ctx context.Context, id string) (*schema.Document, error) {
	return a.inner.GetByID(ctx, id)
}

func (a *VectorStoreAdapter) DropAll(ctx context.Context) error {
	return a.inner.DropAll(ctx)
}
