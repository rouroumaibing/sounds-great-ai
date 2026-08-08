// internal/ragstore/migrator.go
package ragstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/cloudwego/eino/schema"
)

// Migrator performs doc-level resumable migration from a retired backend to the
// current active backend. Each doc's migration status is persisted in a
// SQLite migration_log table so that interrupted runs can resume by retrying
// only pending docs on the next SyncData call.
type Migrator struct {
	registry *StoreRegistry
	db       *sql.DB
}

// MigrationProgress summarizes the state of a migration between two backends.
type MigrationProgress struct {
	Total   int
	Done    int
	Failed  int
	Pending int
}

// NewMigrator opens (or creates) the migration log database at dbPath and
// ensures the migration_log and retired_stores tables exist.
func NewMigrator(registry *StoreRegistry, dbPath string) (*Migrator, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS migration_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			backend_from TEXT NOT NULL,
			backend_to   TEXT NOT NULL,
			doc_id       TEXT NOT NULL,
			status       TEXT NOT NULL,
			error        TEXT,
			migrated_at  DATETIME,
			UNIQUE(backend_from, backend_to, doc_id)
		);
		CREATE INDEX IF NOT EXISTS idx_status ON migration_log(status);
		CREATE TABLE IF NOT EXISTS retired_stores (
			backend      TEXT PRIMARY KEY,
			retired_at   DATETIME NOT NULL,
			retire_at    DATETIME NOT NULL,
			store_config TEXT NOT NULL
		);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Migrator{registry: registry, db: db}, nil
}

// DB exposes the underlying migration log database so callers (e.g. the HTTP
// handler in Task 17) can run their own queries or persist retiree metadata.
func (m *Migrator) DB() *sql.DB { return m.db }

// Close releases the migration log database.
//
// Close is intentionally NOT wired into the main shutdown path. The migrator's
// *sql.DB is used by handleSync/handleSyncProgress (HTTP handlers) which may
// have in-flight requests at shutdown; closing the DB under them would race.
// Process exit reclaims the handle. Close is retained for use in tests, where
// there are no concurrent HTTP callers.
func (m *Migrator) Close() error { return m.db.Close() }

// SyncData migrates all docs from the retired backend fromBk into the current
// active backend. It is idempotent and resumable: each doc is recorded in
// migration_log, and only docs still in the 'pending' status are retried.
// The returned MigrationProgress reflects the state after this run.
func (m *Migrator) SyncData(ctx context.Context, fromBk BackendType) (*MigrationProgress, error) {
	oldStore, err := m.registry.GetRetired(fromBk)
	if err != nil {
		return nil, err
	}
	_, newBk := m.registry.Active()

	// 1. Enumerate old store docs, insert migration_log (idempotent)
	docs, err := oldStore.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list old store: %w", err)
	}
	for _, doc := range docs {
		if err := m.ensureLogEntry(ctx, fromBk, newBk, doc.ID); err != nil {
			return nil, err
		}
	}

	// 2. Get pending doc IDs
	pendingIDs, err := m.getPendingDocIDs(ctx, fromBk, newBk)
	if err != nil {
		return nil, err
	}

	// 3. Migrate each pending doc
	activeStore, _ := m.registry.Active()
	for _, docID := range pendingIDs {
		if ctx.Err() != nil {
			return m.progress(ctx, fromBk, newBk)
		}
		doc, err := oldStore.GetByID(ctx, docID)
		if err != nil {
			m.markFailed(ctx, fromBk, newBk, docID, fmt.Sprintf("get: %v", err))
			continue
		}
		if err := activeStore.Upsert(ctx, []*schema.Document{doc}); err != nil {
			m.markFailed(ctx, fromBk, newBk, docID, fmt.Sprintf("upsert: %v", err))
			continue
		}
		m.markDone(ctx, fromBk, newBk, docID)
	}

	return m.progress(ctx, fromBk, newBk)
}

// QueryProgress is the public accessor for migration progress between two
// backends. It is used by the HTTP handler (Task 17) to report status.
func (m *Migrator) QueryProgress(ctx context.Context, from, to BackendType) (*MigrationProgress, error) {
	return m.progress(ctx, from, to)
}

func (m *Migrator) ensureLogEntry(ctx context.Context, from, to BackendType, docID string) error {
	_, err := m.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO migration_log (backend_from, backend_to, doc_id, status)
		 VALUES (?, ?, ?, 'pending')`, string(from), string(to), docID)
	return err
}

func (m *Migrator) getPendingDocIDs(ctx context.Context, from, to BackendType) ([]string, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT doc_id FROM migration_log
		 WHERE backend_from = ? AND backend_to = ? AND status = 'pending'`,
		string(from), string(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *Migrator) markDone(ctx context.Context, from, to BackendType, docID string) {
	_, _ = m.db.ExecContext(ctx,
		`UPDATE migration_log SET status = 'done', migrated_at = CURRENT_TIMESTAMP
		 WHERE backend_from = ? AND backend_to = ? AND doc_id = ?`,
		string(from), string(to), docID)
}

func (m *Migrator) markFailed(ctx context.Context, from, to BackendType, docID, errMsg string) {
	_, _ = m.db.ExecContext(ctx,
		`UPDATE migration_log SET status = 'failed', error = ?
		 WHERE backend_from = ? AND backend_to = ? AND doc_id = ?`,
		errMsg, string(from), string(to), docID)
}

func (m *Migrator) progress(ctx context.Context, from, to BackendType) (*MigrationProgress, error) {
	p := &MigrationProgress{}
	rows, err := m.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM migration_log
		 WHERE backend_from = ? AND backend_to = ?
		 GROUP BY status`, string(from), string(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		p.Total += count
		switch status {
		case "done":
			p.Done = count
		case "failed":
			p.Failed = count
		case "pending":
			p.Pending = count
		}
	}
	return p, nil
}
