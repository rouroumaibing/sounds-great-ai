// internal/ragstore/sqlite_store_test.go
package ragstore

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestSQLiteStore_UpsertSearchDelete(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0, 0.0, 0.0}}
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(emb, dbPath)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer store.Close()

	docs := []*schema.Document{
		{ID: "d1", Content: "hello", MetaData: map[string]any{"namespace": "docs"}},
		{ID: "d2", Content: "world", MetaData: map[string]any{"namespace": "docs"}},
	}
	if err := store.Upsert(context.Background(), docs); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	results, err := store.Search(context.Background(), "hello", SearchOpts{TopK: 2})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}

	// Delete d1
	store.Delete(context.Background(), []string{"d1"})
	results, _ = store.Search(context.Background(), "hello", SearchOpts{TopK: 10})
	if len(results) != 1 {
		t.Fatalf("after delete: want 1, got %d", len(results))
	}
	if results[0].ID != "d2" {
		t.Fatalf("want d2, got %s", results[0].ID)
	}
}

func TestSQLiteStore_RestartPersistence(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	dbPath := filepath.Join(t.TempDir(), "test.db")

	store1, _ := NewSQLiteStore(emb, dbPath)
	store1.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "persisted"},
	})
	store1.Close()

	// New store loads from same db
	store2, _ := NewSQLiteStore(emb, dbPath)
	defer store2.Close()
	results, _ := store2.Search(context.Background(), "q", SearchOpts{TopK: 10})
	if len(results) != 1 {
		t.Fatalf("after restart: want 1, got %d", len(results))
	}
	if results[0].ID != "d1" {
		t.Fatalf("want d1, got %s", results[0].ID)
	}
}

func TestSQLiteStore_NamespaceFilter(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store, _ := NewSQLiteStore(emb, filepath.Join(t.TempDir(), "test.db"))
	defer store.Close()
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "a", MetaData: map[string]any{"namespace": "ns1"}},
		{ID: "d2", Content: "b", MetaData: map[string]any{"namespace": "ns2"}},
	})
	results, _ := store.Search(context.Background(), "q", SearchOpts{TopK: 10, Namespace: "ns1"})
	if len(results) != 1 || results[0].ID != "d1" {
		t.Fatalf("namespace filter failed: %v", results)
	}
}

func TestSQLiteStore_ConcurrentAccess_NoRace(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store, _ := NewSQLiteStore(emb, filepath.Join(t.TempDir(), "test.db"))
	defer store.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.Upsert(context.Background(), []*schema.Document{
				{ID: string(rune('a' + n%26)), Content: "x"},
			})
		}(i)
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Search(context.Background(), "q", SearchOpts{TopK: 5})
		}()
	}
	wg.Wait()
}
