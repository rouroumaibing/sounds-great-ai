package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (external dependency, default-on since 2026-08-17)
)

// persister abstracts how the experience-memory document is loaded/saved.
// jsonPersister is the legacy file backend; sqlitePersister is the default-on
// external-dependency backend (modernc.org/sqlite, pure Go, no server).
//
// 2026-08-17: NewMemoryStoreAt / NewEvidenceStoreAt prefer SQLite and fall back
// to JSON when the sqlite driver cannot be initialized (e.g. a stripped build
// without the driver). SQLite is the default store, with JSON kept as a
// zero-dependency fallback.
type persister interface {
	load() (*memoryDocument, error)
	save(doc *memoryDocument) error
	close()
}

// ---- JSON persister (legacy fallback) ----

type jsonPersister struct {
	path string
}

func newJSONPersister(path string) *jsonPersister { return &jsonPersister{path: path} }

func (j *jsonPersister) load() (*memoryDocument, error) {
	if j.path == "" {
		return &memoryDocument{}, nil
	}
	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &memoryDocument{}, nil
		}
		return nil, err
	}
	var doc memoryDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (j *jsonPersister) save(doc *memoryDocument) error {
	if j.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(j.path, data)
}

func (j *jsonPersister) close() {}

// ---- SQLite persister (default) ----

type sqlitePersister struct {
	db *sql.DB
}

// newSQLitePersister opens (or creates) a SQLite-backed store at path+".db".
// Returns an error if the sqlite driver cannot be initialized, so callers can
// fall back to the JSON persister.
func newSQLitePersister(path string) (*sqlitePersister, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite persister: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dbPath := path + ".db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	const schema = `
	CREATE TABLE IF NOT EXISTS experience (
		kind    TEXT NOT NULL,
		id      TEXT NOT NULL,
		breed   TEXT,
		content TEXT,
		task    TEXT,
		ctx     TEXT,
		topic   TEXT,
		decision TEXT,
		reason  TEXT,
		PRIMARY KEY (kind, id)
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &sqlitePersister{db: db}, nil
}

func (s *sqlitePersister) load() (*memoryDocument, error) {
	doc := &memoryDocument{Decisions: map[string]*Decision{}}
	rows, err := s.db.Query("SELECT kind, id, breed, content, task, ctx, topic, decision, reason FROM experience")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, id string
		var breed, content, task, ctx, topic, decision, reason sql.NullString
		if err := rows.Scan(&kind, &id, &breed, &content, &task, &ctx, &topic, &decision, &reason); err != nil {
			return nil, err
		}
		switch kind {
		case "evidence":
			doc.Evidence = append(doc.Evidence, Evidence{ID: id, Breed: breed.String, Content: content.String, Task: task.String})
		case "lesson":
			doc.Lessons = append(doc.Lessons, Lesson{ID: id, Content: content.String, Context: ctx.String})
		case "decision":
			doc.Decisions[id] = &Decision{ID: id, Topic: topic.String, Decision: decision.String, Reason: reason.String}
		}
	}
	return doc, nil
}

func (s *sqlitePersister) save(doc *memoryDocument) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM experience"); err != nil {
		tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO experience (kind,id,breed,content,task,ctx,topic,decision,reason) VALUES (?,?,?,?,?,?,?,?,?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, e := range doc.Evidence {
		if _, err := stmt.Exec("evidence", e.ID, e.Breed, e.Content, e.Task, nil, nil, nil, nil); err != nil {
			tx.Rollback()
			return err
		}
	}
	for _, l := range doc.Lessons {
		if _, err := stmt.Exec("lesson", l.ID, nil, l.Content, nil, l.Context, nil, nil, nil); err != nil {
			tx.Rollback()
			return err
		}
	}
	for id, d := range doc.Decisions {
		if _, err := stmt.Exec("decision", id, nil, nil, nil, nil, d.Topic, d.Decision, d.Reason); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *sqlitePersister) close() { _ = s.db.Close() }

// newDefaultPersister prefers the SQLite backend and falls back to JSON.
func newDefaultPersister(path string) persister {
	if p, err := newSQLitePersister(path); err == nil {
		return p
	}
	log.Printf("memory: sqlite unavailable, falling back to JSON at %s", path)
	return newJSONPersister(path)
}
