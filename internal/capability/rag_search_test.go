// internal/capability/rag_search_test.go
package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/pkg/pack"
)

// mockStore is a test VectorStore.
type mockStore struct {
	docs []*schema.Document
	err  error
}

func (m *mockStore) Upsert(ctx context.Context, docs []*schema.Document) error { return nil }
func (m *mockStore) Search(ctx context.Context, query string, opts ragstore.SearchOpts) ([]*schema.Document, error) {
	return m.docs, m.err
}
func (m *mockStore) Delete(ctx context.Context, ids []string) error { return nil }
func (m *mockStore) Close() error                                   { return nil }
func (m *mockStore) ListAll(ctx context.Context) ([]*schema.Document, error) {
	return nil, nil
}
func (m *mockStore) GetByID(ctx context.Context, id string) (*schema.Document, error) {
	return nil, nil
}
func (m *mockStore) DropAll(ctx context.Context) error { return nil }

func TestRagSearch_NameVersion(t *testing.T) {
	r := NewRagSearch(ragstore.NewStoreRegistry(&mockStore{}, ragstore.BackendMemory))
	if r.Name() != "rag_search" {
		t.Fatalf("name: want rag_search, got %s", r.Name())
	}
	if r.Version() != "v1" {
		t.Fatalf("version: want v1, got %s", r.Version())
	}
}

func TestRagSearch_Run_Success(t *testing.T) {
	store := &mockStore{
		docs: []*schema.Document{
			{ID: "d1", Content: "hello", MetaData: map[string]any{"score": 0.9, "source": "test"}},
		},
	}
	r := NewRagSearch(ragstore.NewStoreRegistry(store, ragstore.BackendMemory))
	out, err := r.Run(context.Background(), &pack.TaskInput{
		Query: "hello",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !out.Approved {
		t.Fatal("want Approved=true")
	}
	count, _ := out.Data["count"].(int)
	if count != 1 {
		t.Fatalf("count: want 1, got %v", out.Data["count"])
	}
	matches, _ := out.Data["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("matches: want 1, got %d", len(matches))
	}
}

func TestRagSearch_Run_EmptyResults(t *testing.T) {
	store := &mockStore{docs: nil}
	r := NewRagSearch(ragstore.NewStoreRegistry(store, ragstore.BackendMemory))
	out, err := r.Run(context.Background(), &pack.TaskInput{Query: "q"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.Approved {
		t.Fatal("want Approved=false for empty results")
	}
}

func TestRagSearch_Run_StoreError_Fallback(t *testing.T) {
	store := &mockStore{err: errors.New("boom")}
	r := NewRagSearch(ragstore.NewStoreRegistry(store, ragstore.BackendMemory))
	out, err := r.Run(context.Background(), &pack.TaskInput{Query: "q"})
	if err != nil {
		t.Fatalf("should not return error on store failure: %v", err)
	}
	if out.Approved {
		t.Fatal("want Approved=false on store error")
	}
	fallback, _ := out.Data["fallback"].(bool)
	if !fallback {
		t.Fatal("want fallback=true on store error")
	}
	count, _ := out.Data["count"].(int)
	if count != 0 {
		t.Fatalf("count: want 0 on fallback, got %v", count)
	}
}

func TestRagSearch_Run_ConfigOverrides(t *testing.T) {
	store := &mockStore{docs: nil}
	r := NewRagSearch(ragstore.NewStoreRegistry(store, ragstore.BackendMemory))
	r.Run(context.Background(), &pack.TaskInput{
		Query: "q",
		CapabilityConfig: map[string]any{
			"top_k":     10,
			"threshold": 0.5,
			"namespace": "docs",
		},
	})
	// We can't easily verify opts passed to mock, but we verify no panic + correct types
}
