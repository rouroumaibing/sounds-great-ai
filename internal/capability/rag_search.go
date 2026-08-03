// internal/capability/rag_search.go
package capability

import (
	"context"
	"fmt"

	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/pkg/pack"
)

// RagSearch is the rag_search capability adapter.
// It searches the vector store for documents relevant to the query.
type RagSearch struct {
	registry *ragstore.StoreRegistry
}

func NewRagSearch(registry *ragstore.StoreRegistry) *RagSearch {
	return &RagSearch{registry: registry}
}

func (r *RagSearch) Name() string                   { return "rag_search" }
func (r *RagSearch) Version() string                { return "v1" }
func (r *RagSearch) Init(ctx context.Context) error { return nil }
func (r *RagSearch) Health() error                  { return nil }
func (r *RagSearch) Close() error                   { return nil }

func (r *RagSearch) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	query := getStringConfig(input.CapabilityConfig, "query", input.Query)
	topK := getIntConfig(input.CapabilityConfig, "top_k", 5)
	threshold := getFloatConfig(input.CapabilityConfig, "threshold", 0.0)
	namespace := getStringConfig(input.CapabilityConfig, "namespace", "")

	store, _ := r.registry.Active()
	docs, err := store.Search(ctx, query, ragstore.SearchOpts{
		TopK: topK, Threshold: threshold, Namespace: namespace,
	})
	if err != nil {
		// Fail-open: return empty results, don't block workflow
		return &pack.TaskOutput{
			Approved: false,
			Reason:   fmt.Sprintf("rag_search failed: %v", err),
			Data: map[string]any{
				"matches":  []any{},
				"count":    0,
				"fallback": true,
			},
		}, nil
	}

	matches := make([]any, 0, len(docs))
	for _, doc := range docs {
		matches = append(matches, map[string]any{
			"id":      doc.ID,
			"content": doc.Content,
			"score":   doc.MetaData["score"],
			"source":  doc.MetaData["source"],
		})
	}
	return &pack.TaskOutput{
		Approved: len(docs) > 0,
		Reason:   fmt.Sprintf("found %d matches", len(docs)),
		Data: map[string]any{
			"matches":   matches,
			"count":     len(docs),
			"query":     query,
			"namespace": namespace,
		},
	}, nil
}
