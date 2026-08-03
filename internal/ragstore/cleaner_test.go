// internal/ragstore/cleaner_test.go
package ragstore

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestRetiredCleaner_RemovesExpired(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	oldStore := NewMemoryStore(emb, "")
	oldStore.Upsert(context.Background(), []*schema.Document{{ID: "d1", Content: "a"}})

	registry := NewStoreRegistry(oldStore, BackendMemory)
	// Switch so old goes to retirees
	registry.Switch(context.Background(), BackendMemory, StoreConfig{
		Backend: BackendMemory, Embedder: emb,
	})

	// Manually expire the retiree
	registry.mu.Lock()
	if rs, ok := registry.retirees[BackendMemory]; ok {
		rs.retireAt = time.Now().Add(-1 * time.Hour) // expired
	}
	registry.mu.Unlock()

	cleaner := NewRetiredCleaner(registry, 0) // interval=0 uses default, but we call runOnce directly
	cleaner.runOnce(context.Background())

	retirees := registry.Retirees()
	if len(retirees) != 0 {
		t.Fatalf("after clean: want 0 retirees, got %d", len(retirees))
	}
}

func TestRetiredCleaner_KeepsUnexpired(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	registry := NewStoreRegistry(NewMemoryStore(emb, ""), BackendMemory)
	registry.Switch(context.Background(), BackendMemory, StoreConfig{
		Backend: BackendMemory, Embedder: emb,
	})
	// retiree has retireAt = now+30d (not expired)

	cleaner := NewRetiredCleaner(registry, 0)
	cleaner.runOnce(context.Background())

	retirees := registry.Retirees()
	if len(retirees) != 1 {
		t.Fatalf("should keep unexpired: want 1, got %d", len(retirees))
	}
}

func TestRetiredCleaner_StartStop(t *testing.T) {
	registry := NewStoreRegistry(NewMemoryStore(&stubEmbedder{vec: []float64{1.0}}, ""), BackendMemory)
	cleaner := NewRetiredCleaner(registry, 100*time.Millisecond)
	cleaner.Start()
	time.Sleep(50 * time.Millisecond)
	cleaner.Stop()
	// Just verify no panic/deadlock
}
