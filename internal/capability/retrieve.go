package capability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/pkg/pack"
)

const ragCacheTTL = 5 * time.Minute

type cacheEntry struct {
	output  *pack.TaskOutput
	expires time.Time
}

// ragCache is a simple TTL cache for RAG search results.
// Key = query + namespace. Reduces redundant embedder/store calls.
type ragCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	hits    int
	misses  int
}

func newRagCache() *ragCache {
	return &ragCache{entries: make(map[string]cacheEntry)}
}

func (c *ragCache) get(key string) (*pack.TaskOutput, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}
	if time.Now().After(entry.expires) {
		c.mu.Lock()
		delete(c.entries, key)
		c.misses++
		c.mu.Unlock()
		return nil, false
	}
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
	return entry.output, true
}

func (c *ragCache) set(key string, output *pack.TaskOutput) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{
		output:  output,
		expires: time.Now().Add(ragCacheTTL),
	}
}

// Stats returns cache hit/miss counts for diagnostics.
func (c *ragCache) Stats() (hits, misses, size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses, len(c.entries)
}

// RetrieveCapability is the retrieve capability adapter.
// It queries the RAG vector store for relevant documents and returns
// matches for downstream context_assemble consumption.
type RetrieveCapability struct {
	registry *ragstore.StoreRegistry
	embedder embedding.Embedder
	cache    *ragCache
}

// NewRetrieveCapability constructs a RetrieveCapability from a store registry
// and embedder. Both must be non-nil.
func NewRetrieveCapability(registry *ragstore.StoreRegistry, embedder embedding.Embedder) *RetrieveCapability {
	return &RetrieveCapability{
		registry: registry,
		embedder: embedder,
		cache:    newRagCache(),
	}
}

func (c *RetrieveCapability) Name() string    { return "retrieve" }
func (c *RetrieveCapability) Version() string { return "v1" }

func (c *RetrieveCapability) Init(ctx context.Context) error {
	if c.registry == nil {
		return fmt.Errorf("retrieve: registry is nil")
	}
	return nil
}

func (c *RetrieveCapability) Health() error { return nil }
func (c *RetrieveCapability) Close() error   { return nil }

// CacheStats returns the RAG cache statistics for diagnostics.
func (c *RetrieveCapability) CacheStats() (hits, misses, size int) {
	return c.cache.Stats()
}

// Run extracts the query from input, searches the active RAG store, and returns
// matches as [{content, score, source, metadata}] for context_assemble.
func (c *RetrieveCapability) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	query := input.Query
	if query == "" {
		return &pack.TaskOutput{
			Approved: false,
			Reason:   "empty query",
			Data:     map[string]any{"matches": []any{}},
		}, nil
	}

	// Determine namespace for cache key
	store, _ := c.registry.Active()
	namespace := ""
	if store != nil {
		// Use store's type as namespace identifier
		namespace = fmt.Sprintf("%T", store)
	}

	// Check cache
	cacheKey := query + "|" + namespace
	if cached, ok := c.cache.get(cacheKey); ok {
		return cached, nil
	}

	if store == nil {
		return &pack.TaskOutput{
			Approved: false,
			Reason:   "no active store",
			Data:     map[string]any{"matches": []any{}},
		}, nil
	}

	topK := getIntConfig(input.CapabilityConfig, "top_k", 5)
	threshold := getFloatConfig(input.CapabilityConfig, "threshold", 0.3)

	docs, err := store.Search(ctx, query, ragstore.SearchOpts{
		TopK:      topK,
		Threshold: threshold,
	})
	if err != nil {
		return &pack.TaskOutput{
			Approved: false,
			Reason:   fmt.Sprintf("search failed: %v", err),
			Data:     map[string]any{"matches": []any{}},
		}, nil
	}

	matches := make([]any, 0, len(docs))
	for _, doc := range docs {
		score, _ := doc.MetaData["score"].(float64)
		source, _ := doc.MetaData["source"].(string)
		if source == "" {
			source, _ = doc.MetaData["namespace"].(string)
		}
		matches = append(matches, map[string]any{
			"id":       doc.ID,
			"content":  doc.Content,
			"score":    score,
			"source":   source,
			"metadata": doc.MetaData,
		})
	}

	result := &pack.TaskOutput{
		Approved: len(matches) > 0,
		Reason:   fmt.Sprintf("retrieved %d matches", len(matches)),
		Data: map[string]any{
			"matches": matches,
		},
	}

	// Cache the result
	c.cache.set(cacheKey, result)

	return result, nil
}
