package hooks

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type TraceStore struct {
	db *sql.DB
}

func NewTraceStore(dbPath string) (*TraceStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open trace db: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS hook_traces (
		thread_id   TEXT NOT NULL,
		turn_id     TEXT NOT NULL,
		hook_id     TEXT NOT NULL,
		status      TEXT NOT NULL,
		content_hash TEXT,
		token_estimate INTEGER,
		reason_code  TEXT,
		timestamp   TEXT NOT NULL,
		PRIMARY KEY (thread_id, turn_id, hook_id)
	)`)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &TraceStore{db: db}, nil
}

func (s *TraceStore) Persist(threadID, turnID string, events []TraceEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, e := range events {
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO hook_traces (thread_id, turn_id, hook_id, status, content_hash, token_estimate, reason_code, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			threadID, turnID, e.HookID, e.Status, e.ContentHash, e.TokenEstimate, e.ReasonCode, e.Timestamp.Format(time.RFC3339),
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *TraceStore) GetSummary(threadID, turnID string) ([]TraceEvent, error) {
	rows, err := s.db.Query(
		`SELECT hook_id, status, content_hash, token_estimate, reason_code, timestamp FROM hook_traces WHERE thread_id = ? AND turn_id = ?`,
		threadID, turnID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []TraceEvent
	for rows.Next() {
		var e TraceEvent
		var ts string
		err := rows.Scan(&e.HookID, &e.Status, &e.ContentHash, &e.TokenEstimate, &e.ReasonCode, &ts)
		if err != nil {
			return nil, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
		events = append(events, e)
	}
	return events, nil
}

func (s *TraceStore) ListTurns(threadID string, limit int) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT turn_id FROM hook_traces WHERE thread_id = ? ORDER BY timestamp DESC LIMIT ?`,
		threadID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var turns []string
	for rows.Next() {
		var turnID string
		if err := rows.Scan(&turnID); err != nil {
			return nil, err
		}
		turns = append(turns, turnID)
	}
	return turns, nil
}

func (s *TraceStore) Close() error {
	return s.db.Close()
}
