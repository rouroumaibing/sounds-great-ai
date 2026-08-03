// internal/ragstore/store_test.go
package ragstore

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// fakeStore is a minimal implementation to verify the interface compiles.
type fakeStore struct{}

func (f *fakeStore) Upsert(ctx context.Context, docs []*schema.Document) error { return nil }
func (f *fakeStore) Search(ctx context.Context, query string, opts SearchOpts) ([]*schema.Document, error) {
	return nil, nil
}
func (f *fakeStore) Delete(ctx context.Context, ids []string) error { return nil }
func (f *fakeStore) Close() error                                   { return nil }
func (f *fakeStore) ListAll(ctx context.Context) ([]*schema.Document, error) {
	return nil, nil
}
func (f *fakeStore) GetByID(ctx context.Context, id string) (*schema.Document, error) {
	return nil, nil
}
func (f *fakeStore) DropAll(ctx context.Context) error { return nil }

func TestVectorStore_InterfaceConformance(t *testing.T) {
	var _ VectorStore = (*fakeStore)(nil)
}

func TestBackendType_Constants(t *testing.T) {
	if BackendMemory != "memory" {
		t.Fatalf("BackendMemory: want 'memory', got %q", BackendMemory)
	}
	if BackendSQLite != "sqlite" {
		t.Fatalf("BackendSQLite: want 'sqlite', got %q", BackendSQLite)
	}
	if BackendEino != "eino" {
		t.Fatalf("BackendEino: want 'eino', got %q", BackendEino)
	}
}
