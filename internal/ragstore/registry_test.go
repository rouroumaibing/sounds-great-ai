// internal/ragstore/registry_test.go
package ragstore

import (
	"context"
	"testing"
	"time"
)

func TestStoreRegistry_Initial(t *testing.T) {
	store := NewMemoryStore(&stubEmbedder{vec: []float64{1.0}}, "")
	r := NewStoreRegistry(store, BackendMemory)
	active, bk := r.Active()
	if active == nil {
		t.Fatal("nil active")
	}
	if bk != BackendMemory {
		t.Fatalf("bk: want memory, got %s", bk)
	}
}

func TestStoreRegistry_Switch(t *testing.T) {
	memStore := NewMemoryStore(&stubEmbedder{vec: []float64{1.0}}, "")
	r := NewStoreRegistry(memStore, BackendMemory)

	// Switch to a new memory store (simulate SQLite)
	newStore := NewMemoryStore(&stubEmbedder{vec: []float64{1.0}}, "")
	err := r.Switch(context.Background(), BackendSQLite, StoreConfig{
		Backend:  BackendMemory, // will be overridden by Switch
		Embedder: &stubEmbedder{vec: []float64{1.0}},
	})
	_ = newStore // Switch creates its own store from cfg
	if err != nil {
		t.Fatalf("switch: %v", err)
	}
	active, bk := r.Active()
	if bk != BackendSQLite {
		t.Fatalf("bk: want sqlite, got %s", bk)
	}
	if active == memStore {
		t.Fatal("active should be new store, not old")
	}

	// Old backend should be in retirees
	retirees := r.Retirees()
	if len(retirees) != 1 {
		t.Fatalf("retirees: want 1, got %d", len(retirees))
	}
	if retirees[0].Backend != BackendMemory {
		t.Fatalf("retiree: want memory, got %s", retirees[0].Backend)
	}
}

func TestStoreRegistry_RetireeExpiry(t *testing.T) {
	memStore := NewMemoryStore(&stubEmbedder{vec: []float64{1.0}}, "")
	r := NewStoreRegistry(memStore, BackendMemory)

	// Manually add a retiree that's already expired
	r.mu.Lock()
	r.retirees[BackendSQLite] = &retiredStore{
		store:     NewMemoryStore(&stubEmbedder{vec: []float64{1.0}}, ""),
		retiredAt: time.Now().Add(-31 * 24 * time.Hour),
		retireAt:  time.Now().Add(-1 * 24 * time.Hour), // expired yesterday
	}
	r.mu.Unlock()

	retirees := r.Retirees()
	found := false
	for _, ri := range retirees {
		if ri.Backend == BackendSQLite {
			found = true
			if ri.RetireAt.After(time.Now()) {
				t.Fatal("should be expired")
			}
		}
	}
	if !found {
		t.Fatal("sqlite retiree not found")
	}
}

func TestStoreRegistry_GetRetired(t *testing.T) {
	memStore := NewMemoryStore(&stubEmbedder{vec: []float64{1.0}}, "")
	r := NewStoreRegistry(memStore, BackendMemory)

	r.Switch(context.Background(), BackendMemory, StoreConfig{
		Backend:  BackendMemory,
		Embedder: &stubEmbedder{vec: []float64{1.0}},
	})

	old, err := r.GetRetired(BackendMemory)
	if err != nil {
		t.Fatalf("GetRetired: %v", err)
	}
	if old == nil {
		t.Fatal("nil retired store")
	}
}

func TestStoreRegistry_GetRetired_NotFound(t *testing.T) {
	r := NewStoreRegistry(NewMemoryStore(&stubEmbedder{vec: []float64{1.0}}, ""), BackendMemory)
	_, err := r.GetRetired(BackendSQLite)
	if err == nil {
		t.Fatal("want error for missing retiree")
	}
}

func TestStoreRegistry_RemoveRetired(t *testing.T) {
	memStore := NewMemoryStore(&stubEmbedder{vec: []float64{1.0}}, "")
	r := NewStoreRegistry(memStore, BackendMemory)
	r.Switch(context.Background(), BackendMemory, StoreConfig{
		Backend:  BackendMemory,
		Embedder: &stubEmbedder{vec: []float64{1.0}},
	})

	r.RemoveRetired(BackendMemory)
	retirees := r.Retirees()
	if len(retirees) != 0 {
		t.Fatalf("after remove: want 0, got %d", len(retirees))
	}
}
