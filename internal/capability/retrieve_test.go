package capability

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/pkg/pack"
)

// testEmbedder is a test-only embedding.Embedder returning fixed vectors.
type testEmbedder struct {
	vec []float64
}

func (e *testEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	vecs := make([][]float64, len(texts))
	for i := range texts {
		v := make([]float64, len(e.vec))
		copy(v, e.vec)
		vecs[i] = v
	}
	return vecs, nil
}

func TestRetrieveCapability_NameVersion(t *testing.T) {
	emb := &testEmbedder{vec: []float64{1.0, 0.0}}
	store := ragstore.NewMemoryStore(emb, "")
	reg := ragstore.NewStoreRegistry(store, ragstore.BackendMemory)
	c := NewRetrieveCapability(reg, emb)
	if c.Name() != "retrieve" {
		t.Fatalf("name: want retrieve, got %s", c.Name())
	}
	if c.Version() != "v1" {
		t.Fatalf("version: want v1, got %s", c.Version())
	}
}

func TestRetrieveCapability_Run_WithMatches(t *testing.T) {
	emb := &testEmbedder{vec: []float64{1.0, 0.0}}
	store := ragstore.NewMemoryStore(emb, "")
	reg := ragstore.NewStoreRegistry(store, ragstore.BackendMemory)

	// Seed documents
	store.Upsert(context.Background(), []*schema.Document{
		{ID: "d1", Content: "hello world", MetaData: map[string]any{"namespace": "docs", "source": "test"}},
		{ID: "d2", Content: "foo bar", MetaData: map[string]any{"namespace": "docs", "source": "test"}},
	})

	c := NewRetrieveCapability(reg, emb)
	out, err := c.Run(context.Background(), &pack.TaskInput{
		Query: "hello",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	matches, ok := out.Data["matches"].([]any)
	if !ok {
		t.Fatalf("matches not []any: %T", out.Data["matches"])
	}
	if len(matches) == 0 {
		t.Fatal("want at least 1 match")
	}
	first, _ := matches[0].(map[string]any)
	if first["content"] != "hello world" && first["content"] != "foo bar" {
		t.Fatalf("unexpected content: %v", first["content"])
	}
	if _, ok := first["score"].(float64); !ok {
		t.Fatalf("score not float64: %T", first["score"])
	}
}

func TestRetrieveCapability_Run_EmptyQuery(t *testing.T) {
	emb := &testEmbedder{vec: []float64{1.0}}
	store := ragstore.NewMemoryStore(emb, "")
	reg := ragstore.NewStoreRegistry(store, ragstore.BackendMemory)
	c := NewRetrieveCapability(reg, emb)

	out, err := c.Run(context.Background(), &pack.TaskInput{Query: ""})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.Approved {
		t.Fatal("want Approved=false for empty query")
	}
	matches, _ := out.Data["matches"].([]any)
	if len(matches) != 0 {
		t.Fatalf("want 0 matches, got %d", len(matches))
	}
}

func TestRetrieveCapability_Init_NilRegistry(t *testing.T) {
	c := NewRetrieveCapability(nil, nil)
	if err := c.Init(context.Background()); err == nil {
		t.Fatal("want error for nil registry")
	}
}
