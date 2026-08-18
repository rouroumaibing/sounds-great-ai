package memory

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// migration is an ordered, idempotent schema change guarded by schema_migrations.
type migration struct {
	version int
	apply   func(db *sql.DB) error
}

// allMigrations defines the ordered schema evolution (homologous clowder's
// 39-version migration history, shrunk to SG's needs). Each is safe to re-run.
func allMigrations() []migration {
	return []migration{
		{
			version: 1,
			apply: func(db *sql.DB) error {
				// Full current schema so fresh installs get every column at once.
				_, err := db.Exec(`
				CREATE TABLE IF NOT EXISTS lane_entry (
					id           TEXT PRIMARY KEY,
					lane         TEXT NOT NULL,
					content      TEXT,
					source       TEXT,
					timestamp    INTEGER,
					status       TEXT,
					operator_id  TEXT DEFAULT '',
					sensitivity  TEXT DEFAULT '',
					collection_id TEXT DEFAULT ''
				);`)
				return err
			},
		},
		{
			version: 2,
			apply: func(db *sql.DB) error {
				return addColumnIfMissing(db, "lane_entry", "operator_id", "TEXT")
			},
		},
		{
			version: 3,
			apply: func(db *sql.DB) error {
				if err := addColumnIfMissing(db, "lane_entry", "sensitivity", "TEXT"); err != nil {
					return err
				}
				return addColumnIfMissing(db, "lane_entry", "collection_id", "TEXT")
			},
		},
		{
			version: 4,
			apply: func(db *sql.DB) error {
				// Recall events now live in the same SQLite store (P1-3) instead of
				// a separate JSONL file, so recall + lane durability share one DB.
				_, err := db.Exec(`
				CREATE TABLE IF NOT EXISTS recall_entry (
					id          TEXT PRIMARY KEY,
					operator_id TEXT,
					timestamp   INTEGER,
					kind        TEXT,
					trigger     TEXT,
					entry_ids   TEXT,
					count       INTEGER,
					outcome     TEXT DEFAULT ''
				);`)
				return err
			},
		},
		{
			version: 5,
			apply: func(db *sql.DB) error {
				// FTS5 full-text index over lane content (P1-5), contentless so we
				// own row lifecycle and rebuild it on every save. FTS5 may be
				// unavailable in some SQLite builds; treat its absence as a soft
				// degrade (search falls back to LIKE) rather than failing the
				// whole migration — lane durability must not depend on FTS5.
				_, err := db.Exec(`
				CREATE VIRTUAL TABLE IF NOT EXISTS lane_entry_fts USING fts5(
					content, entry_id UNINDEXED, lane UNINDEXED, operator_id UNINDEXED, content=''
				);`)
				if err != nil {
					log.Printf("memory: FTS5 unavailable, search will fall back to LIKE: %v", err)
				}
				return nil
			},
		},
		{
			version: 6,
			apply: func(db *sql.DB) error {
				// Recall three-axis semantics (P1): recall_entry gains axis +
				// maturity columns so the ledger can report beneficial/unmet/
				// attention quality with measured/estimated maturity labels.
				if err := addColumnIfMissing(db, "recall_entry", "axis", "TEXT DEFAULT ''"); err != nil {
					return err
				}
				if err := addColumnIfMissing(db, "recall_entry", "maturity", "TEXT DEFAULT ''"); err != nil {
					return err
				}
				// Append-only lifecycle trace (homologous clowder lifecycle_traces,
				// Task #39). Recorded at semantic points (approve / modify / retire /
				// recall) rather than via a low-level UPDATE/DELETE trigger, which
				// would amplify audit noise on every bulk flush save.
				_, err := db.Exec(`
				CREATE TABLE IF NOT EXISTS lifecycle_trace (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					axis TEXT, entry_id TEXT, lane TEXT, detail TEXT,
					maturity TEXT DEFAULT 'measured', timestamp INTEGER
				);`)
				return err
			},
		},
	}
}

func addColumnIfMissing(db *sql.DB, table, col, colType string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == col {
			return nil
		}
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s DEFAULT ''", table, col, colType))
	return err
}

// runMigrations applies any pending migrations in order, recording each in
// schema_migrations so they never run twice.
func runMigrations(db *sql.DB) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER)"); err != nil {
		return err
	}
	applied := map[int]bool{}
	if rows, err := db.Query("SELECT version FROM schema_migrations"); err == nil {
		for rows.Next() {
			var v int
			if rows.Scan(&v) == nil {
				applied[v] = true
			}
		}
		rows.Close()
	}
	for _, m := range allMigrations() {
		if applied[m.version] {
			continue
		}
		if err := m.apply(db); err != nil {
			return fmt.Errorf("migration %d: %w", m.version, err)
		}
		if _, err := db.Exec("INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(?, ?)", m.version, time.Now().UnixMilli()); err != nil {
			return err
		}
	}
	return nil
}
