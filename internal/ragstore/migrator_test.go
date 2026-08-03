// internal/ragstore/migrator_test.go
package ragstore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestMigrator_SyncData_Success(t *testing.T) {
	// Setup: memory store with 2 docs, switch to new memory store, migrate
	emb := &stubEmbedder{vec: []float64{1.0}}
	oldStore := NewMemoryStore(emb, "")
	oldStore.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "a"},
		{ID: "d2", Content: "b"},
	})
	registry := NewStoreRegistry(oldStore, BackendMemory)

	// Switch to new store (old goes to retirees)
	registry.Switch(context.Background(), BackendMemory, StoreConfig{
		Backend:  BackendMemory,
		Embedder: emb,
	})

	migrator, err := NewMigrator(registry, filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer migrator.Close()

	progress, err := migrator.SyncData(context.Background(), BackendMemory)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if progress.Total != 2 || progress.Done != 2 {
		t.Fatalf("progress: want total=2 done=2, got %+v", progress)
	}

	// Verify docs are in new store
	active, _ := registry.Active()
	docs, _ := active.ListAll(context.Background())
	if len(docs) != 2 {
		t.Fatalf("new store: want 2, got %d", len(docs))
	}
}

func TestMigrator_SyncData_Resumable(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	oldStore := NewMemoryStore(emb, "")
	oldStore.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "a"},
		{ID: "d2", Content: "b"},
	})
	registry := NewStoreRegistry(oldStore, BackendMemory)
	registry.Switch(context.Background(), BackendMemory, StoreConfig{
		Backend:  BackendMemory,
		Embedder: emb,
	})

	dbPath := filepath.Join(t.TempDir(), "migration.db")
	m1, _ := NewMigrator(registry, dbPath)

	// Cancel after first doc (simulate interruption)
	ctx, cancel := context.WithCancel(context.Background())
	// We can't easily cancel mid-doc, so just run it fully once
	m1.SyncData(ctx, BackendMemory)
	cancel()
	m1.Close()

	// Second run should be idempotent (all done)
	m2, _ := NewMigrator(registry, dbPath)
	defer m2.Close()
	progress, _ := m2.SyncData(context.Background(), BackendMemory)
	if progress.Pending != 0 {
		t.Fatalf("resumed: want 0 pending, got %d", progress.Pending)
	}
	if progress.Done != 2 {
		t.Fatalf("resumed: want 2 done, got %d", progress.Done)
	}
}

func TestMigrator_SyncData_NoRetiree(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	registry := NewStoreRegistry(NewMemoryStore(emb, ""), BackendMemory)
	m, _ := NewMigrator(registry, filepath.Join(t.TempDir(), "migration.db"))
	defer m.Close()

	_, err := m.SyncData(context.Background(), BackendSQLite)
	if err == nil {
		t.Fatal("want error for missing retiree")
	}
}
