// internal/ragstore/sqlite_hybrid.go
package ragstore

import (
	"context"
	"sort"

	"github.com/cloudwego/eino/schema"
)

// bm25Search queries the FTS5 table for BM25-ranked doc_ids.
// Returns nil on any error (caller falls back to vector-only).
func (s *SQLiteStore) bm25Search(ctx context.Context, query string, opts SearchOpts) []string {
	k := opts.TopK
	if k <= 0 {
		k = 10
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT doc_id FROM docs_fts WHERE docs_fts MATCH ? ORDER BY rank LIMIT ?",
		query, k)
	if err != nil {
		return nil // FTS5 query syntax error → fall back
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil
		}
		// Namespace filter (FTS5 doesn't know namespaces)
		if opts.Namespace != "" {
			var ns string
			s.db.QueryRowContext(ctx,
				"SELECT namespace FROM documents WHERE id = ?", id).Scan(&ns)
			if ns != opts.Namespace {
				continue
			}
		}
		ids = append(ids, id)
	}
	return ids
}

// rrrFusion merges vector and BM25 ranked lists using Reciprocal Rank Fusion.
// RRF score = 1/(60+bm25_rank) + 1/(60+vector_rank), k=60 (对标 clowder-ai).
func (s *SQLiteStore) rrrFusion(vecResults []*schema.Document, bm25IDs []string, opts SearchOpts) []*schema.Document {
	const rrfK = 60.0

	// Build rank maps (0-indexed)
	vecRank := make(map[string]int, len(vecResults))
	for i, doc := range vecResults {
		vecRank[doc.ID] = i
	}
	bm25Rank := make(map[string]int, len(bm25IDs))
	for i, id := range bm25IDs {
		bm25Rank[id] = i
	}

	// Collect all unique doc IDs
	allIDs := make(map[string]bool)
	for id := range vecRank {
		allIDs[id] = true
	}
	for id := range bm25Rank {
		allIDs[id] = true
	}

	// Compute RRF scores
	type rrfEntry struct {
		id    string
		score float64
	}
	var entries []rrfEntry
	for id := range allIDs {
		score := 0.0
		if rank, ok := vecRank[id]; ok {
			score += 1.0 / (rrfK + float64(rank))
		}
		if rank, ok := bm25Rank[id]; ok {
			score += 1.0 / (rrfK + float64(rank))
		}
		entries = append(entries, rrfEntry{id: id, score: score})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].score > entries[j].score })

	// Build result docs, reusing vector results where available
	vecByID := make(map[string]*schema.Document, len(vecResults))
	for _, doc := range vecResults {
		vecByID[doc.ID] = doc
	}

	topK := opts.TopK
	if topK > 0 && len(entries) > topK {
		entries = entries[:topK]
	}
	docs := make([]*schema.Document, 0, len(entries))
	for _, e := range entries {
		if doc, ok := vecByID[e.id]; ok {
			docs = append(docs, doc)
		} else {
			// BM25-only result — fetch from documents table
			doc, err := s.GetByID(context.Background(), e.id)
			if err != nil || doc == nil {
				continue
			}
			if doc.MetaData == nil {
				doc.MetaData = map[string]any{}
			}
			doc.MetaData["score"] = e.score
			docs = append(docs, doc)
		}
	}
	return docs
}
