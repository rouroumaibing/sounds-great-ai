package threadstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// sqliteThreadStore implements ThreadStore backed by SQLite.
type sqliteThreadStore struct {
	db *sql.DB
}

// NewSQLiteThreadStore creates a SQLite-backed ThreadStore.
func NewSQLiteThreadStore(path string) (ThreadStore, error) {
	// Ensure the parent directory exists (see NewSQLiteMessageStore for why).
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir %q: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS threads (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		deleted_at INTEGER
	);
	CREATE TABLE IF NOT EXISTS thread_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		thread_id TEXT NOT NULL,
		event TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_events_thread ON thread_events(thread_id, id);
	CREATE TABLE IF NOT EXISTS thread_sessions (
		id TEXT PRIMARY KEY,
		thread_id TEXT NOT NULL,
		breed_id TEXT NOT NULL,
		seq INTEGER DEFAULT 0,
		status TEXT DEFAULT 'active',
		message_count INTEGER DEFAULT 0,
		seal_reason TEXT,
		created_at INTEGER NOT NULL,
		sealed_at INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_thread ON thread_sessions(thread_id);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &sqliteThreadStore{db: db}, nil
}

func (s *sqliteThreadStore) CreateThread(title string) (*Thread, error) {
	thread := &Thread{
		ID:        fmt.Sprintf("thread-%d", time.Now().UnixNano()),
		Title:     title,
		CreatedAt: time.Now().Unix(),
	}

	_, err := s.db.Exec(
		`INSERT INTO threads (id, title, created_at) VALUES (?, ?, ?)`,
		thread.ID, thread.Title, thread.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return thread, nil
}

func (s *sqliteThreadStore) ListThreads() ([]*Thread, error) {
	rows, err := s.db.Query(
		`SELECT id, title, created_at, deleted_at FROM threads WHERE deleted_at IS NULL ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Thread
	for rows.Next() {
		t := &Thread{}
		if err := rows.Scan(&t.ID, &t.Title, &t.CreatedAt, &t.DeletedAt); err != nil {
			return nil, err
		}
		result = append(result, t)
	}

	return result, rows.Err()
}

func (s *sqliteThreadStore) DeleteThread(id string) error {
	_, err := s.db.Exec(`UPDATE threads SET deleted_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

func (s *sqliteThreadStore) UpdateTitle(id string, title string) error {
	_, err := s.db.Exec(`UPDATE threads SET title = ? WHERE id = ?`, title, id)
	return err
}

func (s *sqliteThreadStore) AddEvent(threadID string, event json.RawMessage) error {
	_, err := s.db.Exec(
		`INSERT INTO thread_events (thread_id, event, created_at) VALUES (?, ?, ?)`,
		threadID, string(event), time.Now().UnixNano(),
	)
	return err
}

func (s *sqliteThreadStore) GetEvents(threadID string) ([]json.RawMessage, error) {
	rows, err := s.db.Query(
		`SELECT event FROM thread_events WHERE thread_id = ? ORDER BY id ASC`,
		threadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []json.RawMessage
	for rows.Next() {
		var event string
		if err := rows.Scan(&event); err != nil {
			return nil, err
		}
		result = append(result, json.RawMessage(event))
	}

	return result, rows.Err()
}

func (s *sqliteThreadStore) CreateSession(threadID, breedID string) (*SessionRecord, error) {
	session := &SessionRecord{
		ID:        fmt.Sprintf("session-%d", time.Now().UnixNano()),
		ThreadID:  threadID,
		BreedID:   breedID,
		Status:    "active",
		CreatedAt: time.Now().Unix(),
	}

	_, err := s.db.Exec(
		`INSERT INTO thread_sessions (id, thread_id, breed_id, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		session.ID, session.ThreadID, session.BreedID, session.Status, session.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (s *sqliteThreadStore) ListSessions(threadID string) ([]*SessionRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, thread_id, breed_id, seq, status, message_count, seal_reason, created_at, sealed_at
		 FROM thread_sessions WHERE thread_id = ? ORDER BY created_at ASC`,
		threadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*SessionRecord
	for rows.Next() {
		sr := &SessionRecord{}
		if err := rows.Scan(&sr.ID, &sr.ThreadID, &sr.BreedID, &sr.Seq, &sr.Status,
			&sr.MessageCount, &sr.SealReason, &sr.CreatedAt, &sr.SealedAt); err != nil {
			return nil, err
		}
		result = append(result, sr)
	}

	return result, rows.Err()
}

func (s *sqliteThreadStore) UnsealSession(sessionID string) error {
	_, err := s.db.Exec(
		`UPDATE thread_sessions SET status = 'active', sealed_at = NULL WHERE id = ?`,
		sessionID,
	)
	return err
}

// Close closes the underlying database connection.
func (s *sqliteThreadStore) Close() error {
	return s.db.Close()
}
