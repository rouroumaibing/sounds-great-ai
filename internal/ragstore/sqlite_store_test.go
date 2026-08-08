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

func TestSQLiteStore_FTS5Available(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store, _ := NewSQLiteStore(emb, filepath.Join(t.TempDir(), "test.db"))
	defer store.Close()
	if !store.ftsAvailable {
		t.Fatal("FTS5 should be available with modernc.org/sqlite")
	}
}

func TestSQLiteStore_HybridSearch_BM25Vector(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0, 0.0}}
	store, _ := NewSQLiteStore(emb, filepath.Join(t.TempDir(), "test.db"))
	defer store.Close()

	docs := []*schema.Document{
		{ID: "d1", Content: "the quick brown fox jumps", MetaData: map[string]any{"title": "fox story"}},
		{ID: "d2", Content: "lazy dog sleeps all day", MetaData: map[string]any{"title": "dog story"}},
		{ID: "d3", Content: "fox and dog are friends", MetaData: map[string]any{"title": "friends"}},
	}
	if err := store.Upsert(context.Background(), docs); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Search for "fox" — should match d1 and d3 via BM25, all via vector
	results, err := store.Search(context.Background(), "fox", SearchOpts{TopK: 5, Threshold: 0.0})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("hybrid search returned no results")
	}

	// Verify RRF fusion: docs matching both BM25 and vector should rank higher
	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.ID] = true
	}
	// d1 and d3 contain "fox" → should be in results
	if !ids["d1"] {
		t.Fatal("d1 (contains 'fox') missing from hybrid results")
	}
	if !ids["d3"] {
		t.Fatal("d3 (contains 'fox') missing from hybrid results")
	}
}

func TestSQLiteStore_HybridSearch_FTS5ErrorFallback(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store, _ := NewSQLiteStore(emb, filepath.Join(t.TempDir(), "test.db"))
	defer store.Close()

	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "hello world"},
	})

	// Invalid FTS5 query syntax (unmatched quote) → should fall back to vector-only
	results, err := store.Search(context.Background(), `hello "`, SearchOpts{TopK: 5, Threshold: 0.0})
	if err != nil {
		t.Fatalf("search with bad FTS5 query: %v", err)
	}
	// Should still get vector results
	if len(results) == 0 {
		t.Fatal("fallback to vector-only returned no results")
	}
}

func TestSQLiteStore_HybridSearch_RRFOrdering(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store, _ := NewSQLiteStore(emb, filepath.Join(t.TempDir(), "test.db"))
	defer store.Close()

	// d1 matches BM25 query exactly; d2 does not
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "golang testing best practices", MetaData: map[string]any{"title": "go"}},
		{ID: "d2", Content: "python recipes cookbook", MetaData: map[string]any{"title": "py"}},
	})

	results, err := store.Search(context.Background(), "golang", SearchOpts{TopK: 2, Threshold: 0.0})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}
	// d1 should rank first (matches BM25 + vector)
	if results[0].ID != "d1" {
		t.Fatalf("RRF: want d1 first, got %s", results[0].ID)
	}
}

func TestSQLiteStore_FTS5SyncOnUpsert(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store, _ := NewSQLiteStore(emb, filepath.Join(t.TempDir(), "test.db"))
	defer store.Close()

	// Upsert
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "original content", MetaData: map[string]any{"title": "orig"}},
	})
	// Re-upsert with new content
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "updated content", MetaData: map[string]any{"title": "updated"}},
	})

	// Search for "updated" — should find the re-upserted content
	results, _ := store.Search(context.Background(), "updated", SearchOpts{TopK: 5, Threshold: 0.0})
	found := false
	for _, r := range results {
		if r.ID == "d1" {
			found = true
		}
	}
	if !found {
		t.Fatal("FTS5 sync on re-upsert failed: d1 not found for 'updated'")
	}
}
