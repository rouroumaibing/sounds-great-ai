// internal/ragstore/memory_store_test.go
package ragstore

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// stubEmbedder is a test-only embedding.Embedder returning fixed vectors.
type stubEmbedder struct {
	vec []float64
}

// EmbedStrings satisfies embedding.Embedder (NOT opts ...any).
// The signature must match the real interface so this stub can be reused by
// Tasks 5, 11, 13, 15, 16, 17 tests.
func (s *stubEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	// Return one vector per text, each vector is s.vec (or a per-text variation)
	vecs := make([][]float64, len(texts))
	for i := range texts {
		// Different texts get different vectors so similarity varies
		v := make([]float64, len(s.vec))
		copy(v, s.vec)
		if i > 0 {
			v[0] += float64(i) * 0.01 // slight variation
		}
		vecs[i] = v
	}
	return vecs, nil
}

func TestMemoryStore_UpsertSearch(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0, 0.0, 0.0}}
	store := NewMemoryStore(emb, "")

	docs := []*schema.Document{
		{ID: "d1", Content: "hello world", MetaData: map[string]any{"namespace": "docs"}},
		{ID: "d2", Content: "foo bar", MetaData: map[string]any{"namespace": "docs"}},
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
}

func TestMemoryStore_SearchNamespaceFilter(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0, 0.0}}
	store := NewMemoryStore(emb, "")

	docs := []*schema.Document{
		{ID: "d1", Content: "a", MetaData: map[string]any{"namespace": "ns1"}},
		{ID: "d2", Content: "b", MetaData: map[string]any{"namespace": "ns2"}},
	}
	store.Upsert(context.Background(), docs)

	results, err := store.Search(context.Background(), "q", SearchOpts{TopK: 10, Namespace: "ns1"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("namespace filter: want 1, got %d", len(results))
	}
	if results[0].ID != "d1" {
		t.Fatalf("want d1, got %s", results[0].ID)
	}
}

func TestMemoryStore_SearchThreshold(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0, 0.0}}
	store := NewMemoryStore(emb, "")
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "a"},
	})

	// Threshold = 2.0 (impossible) should filter everything
	results, _ := store.Search(context.Background(), "q", SearchOpts{TopK: 10, Threshold: 2.0})
	if len(results) != 0 {
		t.Fatalf("threshold 2.0: want 0, got %d", len(results))
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store := NewMemoryStore(emb, "")
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "a"},
		{ID: "d2", Content: "b"},
	})

	store.Delete(context.Background(), []string{"d1"})
	results, _ := store.Search(context.Background(), "q", SearchOpts{TopK: 10})
	if len(results) != 1 {
		t.Fatalf("after delete: want 1, got %d", len(results))
	}
	if results[0].ID != "d2" {
		t.Fatalf("want d2, got %s", results[0].ID)
	}
}

func TestMemoryStore_Search_MetaDataDeepCopy(t *testing.T) {
	// Verify Search returns a deep-copied MetaData map, so callers mutating
	// the returned "score" don't trigger concurrent map writes on the stored doc.
	emb := &stubEmbedder{vec: []float64{1.0}}
	store := NewMemoryStore(emb, "")
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "a", MetaData: map[string]any{"source": "test"}},
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results, _ := store.Search(context.Background(), "q", SearchOpts{TopK: 1})
			for _, r := range results {
				r.MetaData["score"] = 0.99 // mutate returned copy
			}
		}()
	}
	wg.Wait()
	// If shallow copy, this would race-fail under -race
}

func TestMemoryStore_PersistRoundTrip(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0, 0.5}}
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "rag.json")

	store1 := NewMemoryStore(emb, path)
	store1.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "hello", MetaData: map[string]any{"ns": "test"}},
	})
	// Poll for async persist (saveToDisk runs in a goroutine, lock-free I/O).
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("persist file not created within 1s")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// New store loads from same path
	store2 := NewMemoryStore(emb, path)
	results, _ := store2.Search(context.Background(), "q", SearchOpts{TopK: 10})
	if len(results) != 1 {
		t.Fatalf("after reload: want 1, got %d", len(results))
	}
	if results[0].ID != "d1" {
		t.Fatalf("want d1, got %s", results[0].ID)
	}
}

func TestMemoryStore_ConcurrentUpsertSearch_NoRace(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store := NewMemoryStore(emb, "")

	var wg sync.WaitGroup
	// Concurrent upserts
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.Upsert(context.Background(), []*schema.Document{
				{ID: string(rune('a' + n%26)), Content: "x"},
			})
		}(i)
	}
	// Concurrent searches
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Search(context.Background(), "q", SearchOpts{TopK: 5})
		}()
	}
	wg.Wait()
}
