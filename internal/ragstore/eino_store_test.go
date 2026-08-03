// internal/ragstore/eino_store_test.go
package ragstore

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// mockIndexer satisfies indexer.Indexer.
// Note: the real Eino v0.9.13 signature is
//
//	Store(ctx, docs, opts...) (ids []string, err error)
//
// — it returns assigned IDs, unlike the brief's assumed `error`-only return.
type mockIndexer struct {
	stored []*schema.Document
	ids    []string
	err    error
}

func (m *mockIndexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) {
	m.stored = docs
	if m.err != nil {
		return nil, m.err
	}
	if m.ids != nil {
		return m.ids, nil
	}
	ids := make([]string, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	return ids, nil
}

// mockRetriever satisfies retriever.Retriever.
type mockRetriever struct {
	docs []*schema.Document
	err  error
}

func (m *mockRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	return m.docs, m.err
}

func TestEinoStore_Upsert(t *testing.T) {
	idx := &mockIndexer{}
	ret := &mockRetriever{}
	store, err := NewEinoStore(idx, ret)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	docs := []*schema.Document{{ID: "d1", Content: "hello"}}
	if err := store.Upsert(context.Background(), docs); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if len(idx.stored) != 1 {
		t.Fatalf("want 1 stored, got %d", len(idx.stored))
	}
}

func TestEinoStore_Search(t *testing.T) {
	idx := &mockIndexer{}
	ret := &mockRetriever{
		docs: []*schema.Document{{ID: "d1", Content: "hello"}},
	}
	store, _ := NewEinoStore(idx, ret)
	results, err := store.Search(context.Background(), "q", SearchOpts{TopK: 5})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1, got %d", len(results))
	}
}

func TestEinoStore_Search_Error(t *testing.T) {
	ret := &mockRetriever{err: errors.New("retrieve failed")}
	store, _ := NewEinoStore(&mockIndexer{}, ret)
	_, err := store.Search(context.Background(), "q", SearchOpts{})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestEinoStore_NilIndexer(t *testing.T) {
	_, err := NewEinoStore(nil, &mockRetriever{})
	if err == nil {
		t.Fatal("want error for nil indexer")
	}
}

func TestEinoStore_NilRetriever(t *testing.T) {
	_, err := NewEinoStore(&mockIndexer{}, nil)
	if err == nil {
		t.Fatal("want error for nil retriever")
	}
}

func TestEinoStore_Delete_Unsupported(t *testing.T) {
	store, _ := NewEinoStore(&mockIndexer{}, &mockRetriever{})
	err := store.Delete(context.Background(), []string{"d1"})
	if err == nil {
		t.Fatal("want ErrUnsupported for Delete")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}
