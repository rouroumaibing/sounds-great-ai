// internal/ragstore/memory_store.go
package ragstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// memoryEntry is the per-document record stored in MemoryStore.
// Fields are exported so json.Marshal/Unmarshal in saveToDisk/loadFromDisk
// round-trip the doc and vector (MemoryStore persists vectors to JSON so it
// survives restarts without re-embedding).
type memoryEntry struct {
	Doc    *schema.Document
	Vector []float64
}

type MemoryStore struct {
	embedder    embedding.Embedder
	persistPath string
	mu          sync.RWMutex
	persistMu   sync.Mutex // serializes async persist goroutines
	entries     map[string]*memoryEntry
}

func NewMemoryStore(embedder embedding.Embedder, persistPath string) *MemoryStore {
	s := &MemoryStore{
		embedder:    embedder,
		persistPath: persistPath,
		entries:     make(map[string]*memoryEntry),
	}
	if persistPath != "" {
		_ = s.loadFromDisk()
	}
	return s
}

func (s *MemoryStore) Upsert(ctx context.Context, docs []*schema.Document) error {
	const embedBatchSize = 64
	vectors := make([][]float64, 0, len(docs))
	for i := 0; i < len(docs); i += embedBatchSize {
		end := i + embedBatchSize
		if end > len(docs) {
			end = len(docs)
		}
		texts := make([]string, end-i)
		for j := i; j < end; j++ {
			texts[j-i] = docs[j].Content
		}
		batchVecs, err := s.embedder.EmbedStrings(ctx, texts)
		if err != nil {
			return err
		}
		vectors = append(vectors, batchVecs...)
	}

	s.mu.Lock()
	for i, d := range docs {
		s.entries[d.ID] = &memoryEntry{Doc: d, Vector: vectors[i]}
	}
	snapshot := make(map[string]*memoryEntry, len(s.entries))
	for k, v := range s.entries {
		snapshot[k] = v
	}
	persistPath := s.persistPath
	s.mu.Unlock()

	if persistPath != "" {
		go s.saveToDisk(snapshot)
	}
	return nil
}

func (s *MemoryStore) Search(ctx context.Context, query string, opts SearchOpts) ([]*schema.Document, error) {
	qVec, err := s.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		doc *schema.Document
		sim float64
	}
	var results []scored
	for _, e := range s.entries {
		if opts.Namespace != "" {
			ns, _ := e.Doc.MetaData["namespace"].(string)
			if ns != opts.Namespace {
				continue
			}
		}
		sim := cosineSimilarity(qVec[0], e.Vector)
		if sim >= opts.Threshold {
			// Deep-copy MetaData to prevent concurrent map writes when
			// callers mutate the returned "score" field.
			newMeta := make(map[string]any, len(e.Doc.MetaData)+1)
			for k, v := range e.Doc.MetaData {
				newMeta[k] = v
			}
			newMeta["score"] = sim
			out := &schema.Document{
				ID:       e.Doc.ID,
				Content:  e.Doc.Content,
				MetaData: newMeta,
			}
			results = append(results, scored{doc: out, sim: sim})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].sim > results[j].sim })
	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}
	docs := make([]*schema.Document, len(results))
	for i, r := range results {
		docs[i] = r.doc
	}
	return docs, nil
}

func (s *MemoryStore) Delete(ctx context.Context, ids []string) error {
	s.mu.Lock()
	for _, id := range ids {
		delete(s.entries, id)
	}
	snapshot := make(map[string]*memoryEntry, len(s.entries))
	for k, v := range s.entries {
		snapshot[k] = v
	}
	persistPath := s.persistPath
	s.mu.Unlock()

	if persistPath != "" {
		go s.saveToDisk(snapshot)
	}
	return nil
}

func (s *MemoryStore) Close() error { return nil }

func (s *MemoryStore) ListAll(ctx context.Context) ([]*schema.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	docs := make([]*schema.Document, 0, len(s.entries))
	for _, e := range s.entries {
		docs = append(docs, &schema.Document{
			ID:       e.Doc.ID,
			Content:  e.Doc.Content,
			MetaData: e.Doc.MetaData,
		})
	}
	return docs, nil
}

func (s *MemoryStore) GetByID(ctx context.Context, id string) (*schema.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return &schema.Document{
		ID:       e.Doc.ID,
		Content:  e.Doc.Content,
		MetaData: e.Doc.MetaData,
	}, nil
}

func (s *MemoryStore) DropAll(ctx context.Context) error {
	s.mu.Lock()
	s.entries = make(map[string]*memoryEntry)
	persistPath := s.persistPath
	s.mu.Unlock()
	if persistPath != "" {
		_ = os.Remove(persistPath)
	}
	return nil
}

// saveToDisk writes a snapshot to JSON file (atomic: tmp + rename).
// Called outside the main lock (snapshot is an independent copy).
func (s *MemoryStore) saveToDisk(snapshot map[string]*memoryEntry) {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	data, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	tmp := s.persistPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, s.persistPath)
}

func (s *MemoryStore) loadFromDisk() error {
	data, err := os.ReadFile(s.persistPath)
	if err != nil {
		return err
	}
	var entries map[string]*memoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	s.entries = entries
	return nil
}
