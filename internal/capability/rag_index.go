// internal/capability/rag_index.go
package capability

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/pkg/pack"
)

// RagIndex is the rag_index capability adapter.
// It indexes documents into the vector store.
type RagIndex struct {
	registry *ragstore.StoreRegistry
}

func NewRagIndex(registry *ragstore.StoreRegistry) *RagIndex {
	return &RagIndex{registry: registry}
}

func (r *RagIndex) Name() string                   { return "rag_index" }
func (r *RagIndex) Version() string                { return "v1" }
func (r *RagIndex) Init(ctx context.Context) error { return nil }
func (r *RagIndex) Health() error                  { return nil }
func (r *RagIndex) Close() error                   { return nil }

func (r *RagIndex) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	namespace := getStringConfig(input.CapabilityConfig, "namespace", "default")
	source := getStringConfig(input.CapabilityConfig, "source", "manual")

	var docs []*schema.Document

	if rawDocs, ok := input.CapabilityConfig["documents"]; ok {
		// Batch mode: parse document list from config
		docs = parseDocuments(rawDocs, namespace, source)
	} else {
		// Single doc mode: Query is the content
		docs = []*schema.Document{{
			ID:      uuid.NewString(),
			Content: input.Query,
			MetaData: map[string]any{
				"namespace": namespace,
				"source":    source,
			},
		}}
	}

	store, _ := r.registry.Active()
	if err := store.Upsert(ctx, docs); err != nil {
		return &pack.TaskOutput{
			Approved: false,
			Reason:   fmt.Sprintf("rag_index failed: %v", err),
			Data:     map[string]any{"indexed": 0, "error": err.Error()},
		}, nil
	}

	ids := make([]string, 0, len(docs))
	for _, d := range docs {
		ids = append(ids, d.ID)
	}
	return &pack.TaskOutput{
		Approved: true,
		Reason:   fmt.Sprintf("indexed %d documents", len(docs)),
		Data: map[string]any{
			"indexed":   len(docs),
			"ids":       ids,
			"namespace": namespace,
		},
	}, nil
}

// parseDocuments parses a document list from CapabilityConfig["documents"].
// Each item can be a string (content only) or a map with id/content fields.
func parseDocuments(raw any, namespace, source string) []*schema.Document {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	docs := make([]*schema.Document, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			docs = append(docs, &schema.Document{
				ID:      uuid.NewString(),
				Content: v,
				MetaData: map[string]any{
					"namespace": namespace,
					"source":    source,
				},
			})
		case map[string]any:
			id, _ := v["id"].(string)
			if id == "" {
				id = uuid.NewString()
			}
			content, _ := v["content"].(string)
			docs = append(docs, &schema.Document{
				ID:      id,
				Content: content,
				MetaData: map[string]any{
					"namespace": namespace,
					"source":    source,
				},
			})
		}
	}
	return docs
}
