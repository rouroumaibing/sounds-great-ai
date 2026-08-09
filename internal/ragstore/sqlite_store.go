// internal/ragstore/sqlite_store.go
package ragstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

type SQLiteStore struct {
	embedder     embedding.Embedder
	db           *sql.DB
	mu           sync.Mutex // single-writer lock (SQLite WAL)
	vecCache     map[string][]float64
	cacheMu      sync.RWMutex
	ftsAvailable bool // FTS5 extension available for hybrid search
}

const sqliteSchemaSQL = `
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    meta_data TEXT,
    namespace TEXT,
    embedding BLOB,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_namespace ON documents(namespace);
`

const sqliteFTSSchemaSQL = `
CREATE VIRTUAL TABLE IF NOT EXISTS docs_fts USING fts5(doc_id UNINDEXED, title, content);
`

func NewSQLiteStore(embedder embedding.Embedder, dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, _ = db.Exec("PRAGMA busy_timeout=5000")
	_, err = db.Exec(sqliteSchemaSQL)
	if err != nil {
		db.Close()
		return nil, err
	}
	s := &SQLiteStore{
		embedder: embedder,
		db:       db,
		vecCache: make(map[string][]float64),
	}
	// Try to create FTS5 virtual table; gracefully degrade if unavailable.
	if _, ftsErr := db.Exec(sqliteFTSSchemaSQL); ftsErr == nil {
		s.ftsAvailable = true
	}
	if err := s.loadVecCache(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("load vec cache: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) loadVecCache(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT id, embedding FROM documents")
	if err != nil {
		return err
	}
	defer rows.Close()
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return err
		}
		s.vecCache[id] = decodeFloat64Slice(blob)
	}
	return nil
}

func (s *SQLiteStore) Upsert(ctx context.Context, docs []*schema.Document) error {
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
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, d := range docs {
		metaDataJSON, _ := json.Marshal(d.MetaData)
		ns, _ := d.MetaData["namespace"].(string)
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO documents (id, content, meta_data, namespace, embedding, updated_at)
			 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			d.ID, d.Content, string(metaDataJSON), ns, encodeFloat64Slice(vectors[i]))
		if err != nil {
			return err
		}
		// Sync to FTS5 table if available
		if s.ftsAvailable {
			title, _ := d.MetaData["title"].(string)
			_, _ = tx.ExecContext(ctx, "DELETE FROM docs_fts WHERE doc_id = ?", d.ID)
			_, _ = tx.ExecContext(ctx,
				"INSERT INTO docs_fts (doc_id, title, content) VALUES (?, ?, ?)",
				d.ID, title, d.Content)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	s.cacheMu.Lock()
	for i, d := range docs {
		s.vecCache[d.ID] = vectors[i]
	}
	s.cacheMu.Unlock()
	return nil
}

func (s *SQLiteStore) Search(ctx context.Context, query string, opts SearchOpts) ([]*schema.Document, error) {
	qVec, err := s.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, err
	}

	// 1. Vector NN search (existing cosine scan)
	vecResults := s.vectorSearch(ctx, qVec[0], opts)

	// 2. BM25 recall via FTS5 (if available)
	var bm25Results []string // ranked doc_ids
	if s.ftsAvailable {
		bm25Results = s.bm25Search(ctx, query, opts)
	}

	// 3. RRF fusion or vector-only
	if len(bm25Results) == 0 {
		// No BM25 results (FTS5 unavailable or query error) → vector-only
		return vecResults, nil
	}

	return s.rrrFusion(vecResults, bm25Results, opts), nil
}

// vectorSearch performs the existing cosine similarity scan against the vecCache.
func (s *SQLiteStore) vectorSearch(ctx context.Context, qVec []float64, opts SearchOpts) []*schema.Document {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, content, meta_data FROM documents WHERE namespace = ? OR ? = ''",
		opts.Namespace, opts.Namespace)
	if err != nil {
		return nil
	}
	defer rows.Close()

	type scored struct {
		doc *schema.Document
		sim float64
	}
	var results []scored
	for rows.Next() {
		var id, content, metaDataJSON string
		if err := rows.Scan(&id, &content, &metaDataJSON); err != nil {
			return nil
		}
		s.cacheMu.RLock()
		vec, ok := s.vecCache[id]
		s.cacheMu.RUnlock()
		if !ok {
			continue
		}
		sim := cosineSimilarity(qVec, vec)
		if sim < opts.Threshold {
			continue
		}
		doc := &schema.Document{ID: id, Content: content}
		if metaDataJSON != "" {
			if err := json.Unmarshal([]byte(metaDataJSON), &doc.MetaData); err != nil {
				doc.MetaData = map[string]any{}
			}
		}
		if doc.MetaData == nil {
			doc.MetaData = map[string]any{}
		}
		doc.MetaData["score"] = sim
		results = append(results, scored{doc: doc, sim: sim})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].sim > results[j].sim })
	docs := make([]*schema.Document, len(results))
	for i, r := range results {
		docs[i] = r.doc
	}
	return docs
}

func (s *SQLiteStore) Delete(ctx context.Context, ids []string) error {
	// Lock order: mu → cacheMu (consistent with Upsert, prevents AB-BA)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		_, err := tx.ExecContext(ctx, "DELETE FROM documents WHERE id = ?", id)
		if err != nil {
			return err
		}
		if s.ftsAvailable {
			_, _ = tx.ExecContext(ctx, "DELETE FROM docs_fts WHERE doc_id = ?", id)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, id := range ids {
		delete(s.vecCache, id)
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) ListAll(ctx context.Context) ([]*schema.Document, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, content, meta_data FROM documents")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []*schema.Document
	for rows.Next() {
		var id, content, metaDataJSON string
		if err := rows.Scan(&id, &content, &metaDataJSON); err != nil {
			return nil, err
		}
		doc := &schema.Document{ID: id, Content: content}
		if metaDataJSON != "" {
			if err := json.Unmarshal([]byte(metaDataJSON), &doc.MetaData); err != nil {
				doc.MetaData = map[string]any{}
			}
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (s *SQLiteStore) GetByID(ctx context.Context, id string) (*schema.Document, error) {
	var content, metaDataJSON string
	err := s.db.QueryRowContext(ctx,
		"SELECT content, meta_data FROM documents WHERE id = ?", id).Scan(&content, &metaDataJSON)
	if err != nil {
		return nil, err
	}
	doc := &schema.Document{ID: id, Content: content}
	if metaDataJSON != "" {
		if err := json.Unmarshal([]byte(metaDataJSON), &doc.MetaData); err != nil {
			doc.MetaData = map[string]any{}
		}
	}
	return doc, nil
}

func (s *SQLiteStore) DropAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	_, err := s.db.ExecContext(ctx, "DELETE FROM documents")
	if s.ftsAvailable {
		_, _ = s.db.ExecContext(ctx, "DELETE FROM docs_fts")
	}
	s.vecCache = make(map[string][]float64)
	return err
}
