package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (external dependency, default-on since 2026-08-17)
)

// EvidenceStore is declared in port.go (the package port interface).

// evidencePersister abstracts how evidence records are loaded/saved.
// jsonEvidencePersister is the legacy file backend; sqliteEvidencePersister is
// the default-on external-dependency backend (modernc.org/sqlite, pure Go).
type evidencePersister interface {
	load() ([]*EvidenceRecord, error)
	save(recs []*EvidenceRecord) error
	close()
}

// InMemoryEvidenceStore implements EvidenceStore. When created via
// NewEvidenceStoreAt it is backed by SQLite (default since 2026-08-17,
// modernc.org/sqlite, pure Go, no server) and falls back to a JSON file if
// SQLite cannot be initialized (Persistent Identity layer: learned evidence is
// not lost on restart).
type InMemoryEvidenceStore struct {
	mu       sync.RWMutex
	p        evidencePersister
	evidence []*EvidenceRecord
}

// NewEvidenceStore creates an in-memory (non-persistent) EvidenceStore.
func NewEvidenceStore() EvidenceStore {
	return &InMemoryEvidenceStore{}
}

// NewEvidenceStoreAt creates an EvidenceStore backed by SQLite at path+".db",
// falling back to a JSON file at path if SQLite cannot be initialized.
func NewEvidenceStoreAt(path string) EvidenceStore {
	s := &InMemoryEvidenceStore{}
	if p, err := newSQLiteEvidencePersister(path); err == nil {
		s.p = p
	} else {
		log.Printf("evidence: sqlite unavailable (%v), falling back to JSON at %s", err, path)
		s.p = newJSONEvidencePersister(path)
	}
	if recs, err := s.p.load(); err == nil {
		s.evidence = recs
	} else {
		log.Printf("evidence: load failed (%s): %v", path, err)
	}
	return s
}

func (s *InMemoryEvidenceStore) ListEvidence() ([]*EvidenceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*EvidenceRecord, len(s.evidence))
	copy(result, s.evidence)
	return result, nil
}

func (s *InMemoryEvidenceStore) AddEvidence(threadID, typ, title, content string, tags []string) (*EvidenceRecord, error) {
	if typ == "" {
		typ = "evidence"
	}
	rec := &EvidenceRecord{
		ID:        uuid.NewString(),
		ThreadID:  threadID,
		Type:      typ,
		Title:     title,
		Content:   content,
		Tags:      tags,
		CreatedAt: time.Now().UnixMilli(),
	}
	s.mu.Lock()
	s.evidence = append(s.evidence, rec)
	s.mu.Unlock()
	if s.p != nil {
		if err := s.p.save(s.evidence); err != nil {
			log.Printf("evidence: persist failed: %v", err)
		}
	}
	return rec, nil
}

// ---- JSON evidence persister (legacy fallback) ----

type jsonEvidencePersister struct {
	path string
}

func newJSONEvidencePersister(path string) *jsonEvidencePersister {
	return &jsonEvidencePersister{path: path}
}

func (j *jsonEvidencePersister) load() ([]*EvidenceRecord, error) {
	if j.path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var recs []*EvidenceRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

func (j *jsonEvidencePersister) save(recs []*EvidenceRecord) error {
	if j.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(j.path, data)
}

func (j *jsonEvidencePersister) close() {}

// ---- SQLite evidence persister (default) ----

type sqliteEvidencePersister struct {
	db *sql.DB
}

func newSQLiteEvidencePersister(path string) (*sqliteEvidencePersister, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite evidence persister: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+".db")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	const schema = `
	CREATE TABLE IF NOT EXISTS evidence_records (
		id        TEXT PRIMARY KEY,
		thread_id TEXT,
		typ       TEXT,
		title     TEXT,
		content   TEXT,
		tags      TEXT,
		created_at INTEGER
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &sqliteEvidencePersister{db: db}, nil
}

func (s *sqliteEvidencePersister) load() ([]*EvidenceRecord, error) {
	rows, err := s.db.Query("SELECT id, thread_id, typ, title, content, tags, created_at FROM evidence_records ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var recs []*EvidenceRecord
	for rows.Next() {
		var r EvidenceRecord
		var tags string
		if err := rows.Scan(&r.ID, &r.ThreadID, &r.Type, &r.Title, &r.Content, &tags, &r.CreatedAt); err != nil {
			return nil, err
		}
		if tags != "" {
			_ = json.Unmarshal([]byte(tags), &r.Tags)
		}
		recs = append(recs, &r)
	}
	return recs, nil
}

func (s *sqliteEvidencePersister) save(recs []*EvidenceRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM evidence_records"); err != nil {
		tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO evidence_records (id, thread_id, typ, title, content, tags, created_at) VALUES (?,?,?,?,?,?,?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range recs {
		tags, _ := json.Marshal(r.Tags)
		if _, err := stmt.Exec(r.ID, r.ThreadID, r.Type, r.Title, r.Content, string(tags), r.CreatedAt); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *sqliteEvidencePersister) close() { _ = s.db.Close() }
