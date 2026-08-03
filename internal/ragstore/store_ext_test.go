// internal/ragstore/store_ext_test.go
package ragstore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestMemoryStore_ListAll_GetByID_DropAll(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store := NewMemoryStore(emb, "")
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "a"},
		{ID: "d2", Content: "b"},
	})

	docs, err := store.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("ListAll: want 2, got %d", len(docs))
	}

	doc, err := store.GetByID(context.Background(), "d1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if doc.ID != "d1" {
		t.Fatalf("GetByID: want d1, got %s", doc.ID)
	}

	if err := store.DropAll(context.Background()); err != nil {
		t.Fatalf("DropAll: %v", err)
	}
	docs, _ = store.ListAll(context.Background())
	if len(docs) != 0 {
		t.Fatalf("after DropAll: want 0, got %d", len(docs))
	}
}

func TestSQLiteStore_ListAll_GetByID_DropAll(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store, _ := NewSQLiteStore(emb, filepath.Join(t.TempDir(), "test.db"))
	defer store.Close()
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "a"},
		{ID: "d2", Content: "b"},
	})

	docs, err := store.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("ListAll: want 2, got %d", len(docs))
	}

	doc, err := store.GetByID(context.Background(), "d1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if doc.ID != "d1" {
		t.Fatalf("GetByID: want d1, got %s", doc.ID)
	}

	if err := store.DropAll(context.Background()); err != nil {
		t.Fatalf("DropAll: %v", err)
	}
	docs, _ = store.ListAll(context.Background())
	if len(docs) != 0 {
		t.Fatalf("after DropAll: want 0, got %d", len(docs))
	}
}

func TestEinoStore_ListAll_Unsupported(t *testing.T) {
	store, _ := NewEinoStore(&mockIndexer{}, &mockRetriever{})
	_, err := store.ListAll(context.Background())
	if err == nil {
		t.Fatal("want ErrUnsupported")
	}
}
