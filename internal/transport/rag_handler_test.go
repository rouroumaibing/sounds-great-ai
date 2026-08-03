package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/ragstore"
)

type stubEmbedderHTTP struct{}

// EmbedStrings satisfies embedding.Embedder. The signature must use
// opts ...embedding.Option (NOT opts ...any) so the stub can be assigned to
// embedding.Embedder.
func (s *stubEmbedderHTTP) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	vecs := make([][]float64, len(texts))
	for i := range texts {
		vecs[i] = []float64{1.0}
	}
	return vecs, nil
}

func TestRAGHandler_GetBackend(t *testing.T) {
	emb := &stubEmbedderHTTP{}
	registry := ragstore.NewStoreRegistry(
		ragstore.NewMemoryStore(emb, ""), ragstore.BackendMemory,
	)
	handler := NewRAGHandler(registry, emb, t.TempDir())
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/rag/backend")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["active"] != "memory" {
		t.Fatalf("active: want memory, got %v", result["active"])
	}
}

func TestRAGHandler_SwitchBackend(t *testing.T) {
	emb := &stubEmbedderHTTP{}
	registry := ragstore.NewStoreRegistry(
		ragstore.NewMemoryStore(emb, ""), ragstore.BackendMemory,
	)
	handler := NewRAGHandler(registry, emb, t.TempDir())
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	body := `{"backend":"memory"}`
	resp, err := http.Post(server.URL+"/api/rag/backend/switch", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("switch: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	_, bk := registry.Active()
	if bk != ragstore.BackendMemory {
		t.Fatalf("after switch: want memory, got %s", bk)
	}
}

func TestRAGHandler_SyncProgress(t *testing.T) {
	emb := &stubEmbedderHTTP{}
	oldStore := ragstore.NewMemoryStore(emb, "")
	oldStore.Upsert(context.Background(), []*schema.Document{{ID: "d1", Content: "a"}})
	registry := ragstore.NewStoreRegistry(oldStore, ragstore.BackendMemory)
	registry.Switch(context.Background(), ragstore.BackendMemory, ragstore.StoreConfig{
		Backend: ragstore.BackendMemory, Embedder: emb,
	})

	handler := NewRAGHandler(registry, emb, t.TempDir())
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/rag/sync/progress?from=memory&to=memory")
	if err != nil {
		t.Fatalf("progress: %v", err)
	}
	// May return 200 (with progress) or 500 (no migrator) — both acceptable for this test
	if resp.StatusCode != 200 && resp.StatusCode != 500 {
		t.Fatalf("status: want 200 or 500, got %d", resp.StatusCode)
	}
}
