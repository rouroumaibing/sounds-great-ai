package threadstore

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// sqliteMessageStore implements MessageStore backed by SQLite.
type sqliteMessageStore struct {
	db *sql.DB
}

// NewSQLiteMessageStore creates a SQLite-backed MessageStore.
func NewSQLiteMessageStore(path string) (MessageStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Set pragmas for performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	// Create schema
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		thread_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		sender TEXT NOT NULL,
		timestamp INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(thread_id, timestamp);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &sqliteMessageStore{db: db}, nil
}

func (s *sqliteMessageStore) Append(msg *Message) error {
	if msg.ThreadID == "" {
		return fmt.Errorf("threadID is required")
	}

	if msg.ID == "" {
		msg.ID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	_, err := s.db.Exec(
		`INSERT INTO messages (id, thread_id, role, content, sender, timestamp) VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ThreadID, msg.Role, msg.Content, msg.Sender, msg.Timestamp.UnixNano(),
	)
	return err
}

func (s *sqliteMessageStore) GetByThread(threadID string, limit int) ([]*Message, error) {
	query := `SELECT id, thread_id, role, content, sender, timestamp FROM messages WHERE thread_id = ? ORDER BY timestamp ASC`
	args := []interface{}{threadID}

	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Message
	for rows.Next() {
		msg := &Message{}
		var ts int64
		if err := rows.Scan(&msg.ID, &msg.ThreadID, &msg.Role, &msg.Content, &msg.Sender, &ts); err != nil {
			return nil, err
		}
		msg.Timestamp = time.Unix(0, ts)
		result = append(result, msg)
	}

	return result, rows.Err()
}

func (s *sqliteMessageStore) GetByThreadBefore(threadID string, before time.Time, beforeID string, limit int) ([]*Message, error) {
	var query string
	var args []interface{}

	if before.IsZero() {
		// No cursor — return most recent N messages
		query = `SELECT id, thread_id, role, content, sender, timestamp FROM messages WHERE thread_id = ? ORDER BY timestamp DESC`
		args = []interface{}{threadID}
	} else {
		// Cursor query: messages older than (before, beforeID)
		// Same timestamp uses ID lexicographic tiebreaker
		query = `SELECT id, thread_id, role, content, sender, timestamp FROM messages
			WHERE thread_id = ? AND (timestamp < ? OR (timestamp = ? AND id < ?))
			ORDER BY timestamp DESC`
		args = []interface{}{threadID, before.UnixNano(), before.UnixNano(), beforeID}
	}

	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Message
	for rows.Next() {
		msg := &Message{}
		var ts int64
		if err := rows.Scan(&msg.ID, &msg.ThreadID, &msg.Role, &msg.Content, &msg.Sender, &ts); err != nil {
			return nil, err
		}
		msg.Timestamp = time.Unix(0, ts)
		result = append(result, msg)
	}

	// Reverse to ascending order (oldest first)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, rows.Err()
}

// Close closes the underlying database connection.
func (s *sqliteMessageStore) Close() error {
	return s.db.Close()
}
