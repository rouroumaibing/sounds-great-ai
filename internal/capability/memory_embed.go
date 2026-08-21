package capability

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/embedding"

	"sounds-great-ai/internal/component"
)

// LaneEmbedder produces a dense vector for a single text (Gap3 semantic
// recall). It is an interface so the
// transport handler stays decoupled and is trivially testable with a stub.
type LaneEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// memoryEmbed wraps an eino embedding.Embedder (OpenAI-compatible) and adapts
// its [][]float64 output to the []float32 the vector store expects.
type memoryEmbed struct {
	emb embedding.Embedder
}

// NewMemoryEmbed builds the embedder over an injected eino embedding model.
func NewMemoryEmbed(emb embedding.Embedder) LaneEmbedder {
	return &memoryEmbed{emb: emb}
}

// Embed vectors a single text.
func (m *memoryEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.emb == nil {
		return nil, fmt.Errorf("memory_embed: no embedding model configured")
	}
	vecs, err := m.emb.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("memory_embed: embed failed: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("memory_embed: empty embedding")
	}
	out := make([]float32, len(vecs[0]))
	for i, f := range vecs[0] {
		out[i] = float32(f)
	}
	return out, nil
}

// NewEmbedModelFromEnv builds an embedding model from env (opt-in, like
// NewReflectModelFromEnv). SG_EMBED_API_KEY (+ optional SG_EMBED_MODEL,
// SG_EMBED_BASE_URL) must be set; otherwise returns an error and the platform
// stays deterministic (semantic search degrades to a clear 501).
func NewEmbedModelFromEnv(ctx context.Context) (embedding.Embedder, error) {
	apiKey := os.Getenv("SG_EMBED_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("memory_embed: set SG_EMBED_API_KEY (and optional SG_EMBED_MODEL / SG_EMBED_BASE_URL)")
	}
	model := os.Getenv("SG_EMBED_MODEL")
	if model == "" {
		model = "text-embedding-3-small"
	}
	return component.NewOpenAIEmbedder(ctx, component.EmbedConfig{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: os.Getenv("SG_EMBED_BASE_URL"),
	})
}
