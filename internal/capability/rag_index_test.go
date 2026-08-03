package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/pkg/pack"
)

// captureStore captures Upsert calls for inspection.
type captureStore struct {
	upserted []*schema.Document
	err      error
}

func (c *captureStore) Upsert(ctx context.Context, docs []*schema.Document) error {
	c.upserted = docs
	return c.err
}
func (c *captureStore) Search(ctx context.Context, query string, opts ragstore.SearchOpts) ([]*schema.Document, error) {
	return nil, nil
}
func (c *captureStore) Delete(ctx context.Context, ids []string) error { return nil }
func (c *captureStore) Close() error                                   { return nil }
func (c *captureStore) ListAll(ctx context.Context) ([]*schema.Document, error) {
	return nil, nil
}
func (c *captureStore) GetByID(ctx context.Context, id string) (*schema.Document, error) {
	return nil, nil
}
func (c *captureStore) DropAll(ctx context.Context) error { return nil }

func TestRagIndex_NameVersion(t *testing.T) {
	r := NewRagIndex(ragstore.NewStoreRegistry(&captureStore{}, ragstore.BackendMemory))
	if r.Name() != "rag_index" {
		t.Fatalf("name: want rag_index, got %s", r.Name())
	}
}

func TestRagIndex_Run_SingleDoc(t *testing.T) {
	store := &captureStore{}
	r := NewRagIndex(ragstore.NewStoreRegistry(store, ragstore.BackendMemory))
	out, err := r.Run(context.Background(), &pack.TaskInput{
		Query: "hello world",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !out.Approved {
		t.Fatal("want Approved=true")
	}
	if len(store.upserted) != 1 {
		t.Fatalf("upserted: want 1, got %d", len(store.upserted))
	}
	if store.upserted[0].Content != "hello world" {
		t.Fatalf("content: want 'hello world', got %q", store.upserted[0].Content)
	}
}

func TestRagIndex_Run_StoreError(t *testing.T) {
	store := &captureStore{err: errors.New("boom")}
	r := NewRagIndex(ragstore.NewStoreRegistry(store, ragstore.BackendMemory))
	out, err := r.Run(context.Background(), &pack.TaskInput{Query: "q"})
	if err != nil {
		t.Fatalf("should not return error: %v", err)
	}
	if out.Approved {
		t.Fatal("want Approved=false on store error")
	}
	indexed, _ := out.Data["indexed"].(int)
	if indexed != 0 {
		t.Fatalf("indexed: want 0, got %v", indexed)
	}
}

func TestRagIndex_Run_NamespaceConfig(t *testing.T) {
	store := &captureStore{}
	r := NewRagIndex(ragstore.NewStoreRegistry(store, ragstore.BackendMemory))
	r.Run(context.Background(), &pack.TaskInput{
		Query: "q",
		CapabilityConfig: map[string]any{
			"namespace": "custom-ns",
			"source":    "test",
		},
	})
	if len(store.upserted) != 1 {
		t.Fatalf("want 1 doc, got %d", len(store.upserted))
	}
	ns, _ := store.upserted[0].MetaData["namespace"].(string)
	if ns != "custom-ns" {
		t.Fatalf("namespace: want custom-ns, got %q", ns)
	}
}
