package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (external dependency, default-on since 2026-08-17)
)

// laneDocument is the serializable envelope for the lane registry.
type laneDocument struct {
	OperatorID string       `json:"operator_id"`
	Entries    []*LaneEntry `json:"entries"`
}

// lanePersister abstracts how the lane registry is loaded/saved.
// Mirrors sqlite_persister.go / evidence_store.go: SQLite-preferred, JSON
// fallback, writeAtomic, WAL. This provides the missing durability for typed
// lanes (previously Lane was pure in-memory and its entries were lost on
// restart, inconsistent with the SQLite-persisted experience store).
type lanePersister interface {
	load() (*laneDocument, error)
	save(doc *laneDocument) error
	close()
	// search returns lane entries whose content matches query (FTS5 on SQLite,
	// substring on JSON), visible to operator.
	search(query, operator string) ([]*LaneEntry, error)
}

// ---- Schema migrations (migration struct, allMigrations, addColumnIfMissing,
//      runMigrations) live in lane_migrations.go (P1-4) ----

// openMemoryDB opens (creating if needed) the shared SQLite store at path+".db",
// enables WAL, and applies pending migrations. Shared by both the lane
// persister and the recall store so recall + lane live in one DB (P1-3).
// Migrations are guarded by schema_migrations, so concurrent opens are safe.
func openMemoryDB(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("memory db: empty path")
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
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// ---- JSON persister (legacy fallback) ----

type jsonLanePersister struct {
	path    string
	entries []*LaneEntry // cached for search
}

func newJSONLanePersister(path string) *jsonLanePersister { return &jsonLanePersister{path: path} }

func (j *jsonLanePersister) load() (*laneDocument, error) {
	if j.path == "" {
		return &laneDocument{}, nil
	}
	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &laneDocument{}, nil
		}
		return nil, err
	}
	var doc laneDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	j.entries = doc.Entries
	return &doc, nil
}

func (j *jsonLanePersister) save(doc *laneDocument) error {
	if j.path == "" {
		return nil
	}
	j.entries = doc.Entries
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(j.path, data)
}

func (j *jsonLanePersister) close() {}

func (j *jsonLanePersister) search(query, operator string) ([]*LaneEntry, error) {
	out := make([]*LaneEntry, 0)
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return out, nil
	}
	for _, e := range j.entries {
		if !operatorMatches(e, operator) {
			continue
		}
		if strings.Contains(strings.ToLower(e.Content), q) {
			out = append(out, e)
		}
	}
	return out, nil
}

// ---- SQLite persister (default) ----

type sqliteLanePersister struct {
	db *sql.DB
}

func newSQLiteLanePersister(path string) (*sqliteLanePersister, error) {
	db, err := openMemoryDB(path)
	if err != nil {
		return nil, err
	}
	return &sqliteLanePersister{db: db}, nil
}

func (s *sqliteLanePersister) load() (*laneDocument, error) {
	doc := &laneDocument{}
	rows, err := s.db.Query("SELECT id, lane, content, source, timestamp, status, operator_id, sensitivity, collection_id FROM lane_entry ORDER BY timestamp, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e LaneEntry
		var lane, operatorID, sensitivity, collectionID string
		if err := rows.Scan(&e.ID, &lane, &e.Content, &e.Source, &e.Timestamp, &e.Status, &operatorID, &sensitivity, &collectionID); err != nil {
			return nil, err
		}
		e.Type = LaneType(lane)
		e.OperatorID = operatorID
		e.Sensitivity = sensitivity
		e.CollectionID = collectionID
		doc.Entries = append(doc.Entries, &e)
	}
	return doc, nil
}

func (s *sqliteLanePersister) save(doc *laneDocument) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM lane_entry"); err != nil {
		tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO lane_entry (id, lane, content, source, timestamp, status, operator_id, sensitivity, collection_id) VALUES (?,?,?,?,?,?,?,?,?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, e := range doc.Entries {
		if _, err := stmt.Exec(e.ID, string(e.Type), e.Content, e.Source, e.Timestamp, string(e.Status), e.OperatorID, e.Sensitivity, e.CollectionID); err != nil {
			tx.Rollback()
			return err
		}
	}
	// Commit lane_entry first; the FTS5 rebuild is best-effort and must NEVER
	// roll back lane durability (it runs in its own tx below).
	if err := tx.Commit(); err != nil {
		return err
	}
	rebuildFTS(s.db, doc.Entries)
	return nil
}

// rebuildFTS rebuilds the FTS5 index from canonical rows. Best-effort: if FTS5
// is unavailable the table won't exist and we simply skip (search falls back
// to LIKE). Never returns an error that would jeopardize lane persistence.
func rebuildFTS(db *sql.DB, entries []*LaneEntry) {
	if _, err := db.Exec("DELETE FROM lane_entry_fts"); err != nil {
		return
	}
	fts, err := db.Prepare("INSERT INTO lane_entry_fts (rowid, content, entry_id, lane, operator_id) VALUES (?,?,?,?,?)")
	if err != nil {
		return
	}
	defer fts.Close()
	for i, e := range entries {
		if _, err := fts.Exec(i+1, e.Content, e.ID, string(e.Type), e.OperatorID); err != nil {
			return
		}
	}
}

func (s *sqliteLanePersister) search(query, operator string) ([]*LaneEntry, error) {
	out := make([]*LaneEntry, 0)
	q := strings.TrimSpace(query)
	if q == "" {
		return out, nil
	}
	// Prefer FTS5; fall back to a LIKE scan if FTS5 is unavailable.
	safe := `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
	if rows, err := s.db.Query("SELECT id, lane, content, source, timestamp, status, operator_id, sensitivity, collection_id FROM lane_entry WHERE id IN (SELECT entry_id FROM lane_entry_fts WHERE lane_entry_fts MATCH ?)", safe); err == nil {
		defer rows.Close()
		ok := false
		for rows.Next() {
			ok = true
			var e LaneEntry
			var lane, operatorID, sensitivity, collectionID string
			if err := rows.Scan(&e.ID, &lane, &e.Content, &e.Source, &e.Timestamp, &e.Status, &operatorID, &sensitivity, &collectionID); err != nil {
				return nil, err
			}
			e.Type = LaneType(lane)
			e.OperatorID = operatorID
			e.Sensitivity = sensitivity
			e.CollectionID = collectionID
			if operatorMatches(&e, operator) {
				out = append(out, &e)
			}
		}
		if ok {
			return out, nil
		}
	}
	// LIKE fallback (FTS5 unavailable or no match via FTS).
	like := "%" + q + "%"
	rows, err := s.db.Query("SELECT id, lane, content, source, timestamp, status, operator_id, sensitivity, collection_id FROM lane_entry WHERE content LIKE ?", like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e LaneEntry
		var lane, operatorID, sensitivity, collectionID string
		if err := rows.Scan(&e.ID, &lane, &e.Content, &e.Source, &e.Timestamp, &e.Status, &operatorID, &sensitivity, &collectionID); err != nil {
			return nil, err
		}
		e.Type = LaneType(lane)
		e.OperatorID = operatorID
		e.Sensitivity = sensitivity
		e.CollectionID = collectionID
		if operatorMatches(&e, operator) {
			out = append(out, &e)
		}
	}
	return out, nil
}

func (s *sqliteLanePersister) close() { _ = s.db.Close() }

// newDefaultLanePersister prefers the SQLite backend and falls back to JSON.
func newDefaultLanePersister(path string) lanePersister {
	if p, err := newSQLiteLanePersister(path); err == nil {
		return p
	}
	log.Printf("memory: sqlite lane persister unavailable, falling back to JSON at %s", path)
	return newJSONLanePersister(path)
}
