// internal/ragstore/factory_test.go
package ragstore

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
)

// nilEmbedder satisfies embedding.Embedder for factory tests.
type nilEmbedder struct{}

func (n *nilEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return make([][]float64, len(texts)), nil
}

func TestNewStore_Memory(t *testing.T) {
	store, err := NewStore(StoreConfig{
		Backend:  BackendMemory,
		Embedder: &nilEmbedder{},
	})
	if err != nil {
		t.Fatalf("memory: %v", err)
	}
	if store == nil {
		t.Fatal("nil store")
	}
	store.Close()
}

func TestNewStore_UnknownBackend(t *testing.T) {
	_, err := NewStore(StoreConfig{
		Backend:  BackendType("unknown"),
		Embedder: &nilEmbedder{},
	})
	if err == nil {
		t.Fatal("want error for unknown backend")
	}
}

func TestNewStore_MemoryWithPersist(t *testing.T) {
	tmp := t.TempDir() + "/rag.json"
	store, err := NewStore(StoreConfig{
		Backend:     BackendMemory,
		Embedder:    &nilEmbedder{},
		PersistPath: tmp,
	})
	if err != nil {
		t.Fatalf("memory persist: %v", err)
	}
	if store == nil {
		t.Fatal("nil store")
	}
	store.Close()
}
