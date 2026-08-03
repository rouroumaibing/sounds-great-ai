// internal/ragstore/registry.go
package ragstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

// retiredStore wraps a backend that has been superseded by a Switch.
// It is kept for ~30 days so data can be migrated forward or rolled back.
type retiredStore struct {
	store     VectorStore
	retiredAt time.Time
	retireAt  time.Time
}

// RetiredInfo is the public view of a retired backend returned by Retirees().
type RetiredInfo struct {
	Backend   BackendType
	RetiredAt time.Time
	RetireAt  time.Time
}

// StoreRegistry holds the active VectorStore plus a pool of retired backends.
// Switch moves the current active into the retirees pool and installs a new
// active. This is the core of Phase 4 (dynamic backend switching).
type StoreRegistry struct {
	active    VectorStore
	activeBk  BackendType
	activeCfg StoreConfig
	retirees  map[BackendType]*retiredStore
	migrator  *Migrator
	db        *sql.DB
	embedder  embedding.Embedder
	mu        sync.RWMutex
}

// NewStoreRegistry builds a registry seeded with an already-constructed store.
func NewStoreRegistry(initial VectorStore, initialBk BackendType) *StoreRegistry {
	return &StoreRegistry{
		active:   initial,
		activeBk: initialBk,
		retirees: make(map[BackendType]*retiredStore),
	}
}

// Active returns the current backend and its type.
func (r *StoreRegistry) Active() (VectorStore, BackendType) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active, r.activeBk
}

// Switch installs a new active backend built from cfg (cfg.Backend is forced to
// newBk). The previous active is moved into the retirees pool for 30 days; any
// existing retiree for the same backend type is closed first.
func (r *StoreRegistry) Switch(ctx context.Context, newBk BackendType, cfg StoreConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg.Backend = newBk
	newStore, err := NewStore(cfg)
	if err != nil {
		return err
	}

	if r.active != nil {
		now := time.Now()
		retireAt := now.Add(30 * 24 * time.Hour)
		oldBk := r.activeBk
		oldCfg := r.activeCfg
		if old, ok := r.retirees[oldBk]; ok {
			_ = old.store.Close()
		}
		r.retirees[oldBk] = &retiredStore{
			store:     r.active,
			retiredAt: now,
			retireAt:  retireAt,
		}
		if r.db != nil {
			cfgJSON, _ := json.Marshal(oldCfg)
			_, _ = r.db.ExecContext(ctx,
				`INSERT OR REPLACE INTO retired_stores (backend, retired_at, retire_at, store_config)
				 VALUES (?, ?, ?, ?)`, string(oldBk), now, retireAt, string(cfgJSON))
		}
	}

	r.active = newStore
	r.activeBk = newBk
	r.activeCfg = cfg
	return nil
}

// GetRetired returns a previously retired backend by type.
func (r *StoreRegistry) GetRetired(bk BackendType) (VectorStore, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rs, ok := r.retirees[bk]
	if !ok {
		return nil, &RetiredNotFoundError{Backend: bk}
	}
	return rs.store, nil
}

// RetiredNotFoundError is returned by GetRetired when no retiree exists for bk.
type RetiredNotFoundError struct{ Backend BackendType }

func (e *RetiredNotFoundError) Error() string { return "no retired store: " + string(e.Backend) }

// RemoveRetired closes and drops a retired backend from the pool.
func (r *StoreRegistry) RemoveRetired(bk BackendType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rs, ok := r.retirees[bk]; ok {
		_ = rs.store.Close()
		delete(r.retirees, bk)
	}
	if r.db != nil {
		_, _ = r.db.Exec(`DELETE FROM retired_stores WHERE backend = ?`, string(bk))
	}
}

// Retirees returns info about all retired backends.
func (r *StoreRegistry) Retirees() []RetiredInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	infos := make([]RetiredInfo, 0, len(r.retirees))
	for bk, rs := range r.retirees {
		infos = append(infos, RetiredInfo{
			Backend:   bk,
			RetiredAt: rs.retiredAt,
			RetireAt:  rs.retireAt,
		})
	}
	return infos
}

// SetMigrator attaches a Migrator to the registry. The registry does not own
// the migrator's lifecycle; callers are responsible for closing it.
func (r *StoreRegistry) SetMigrator(m *Migrator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.migrator = m
}

// Migrator returns the registry's attached migrator, or nil if none has been
// set. Callers should guard against a nil return.
func (r *StoreRegistry) Migrator() *Migrator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.migrator
}

// SetDB attaches the migration-log database so Switch/RemoveRetired can persist
// retired_stores rows. The table is created idempotently. A nil db is allowed
// (the registry simply skips SQL persistence), which keeps tests that don't use
// a real DB working.
func (r *StoreRegistry) SetDB(db *sql.DB) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.db = db
	if db != nil {
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS retired_stores (
			backend      TEXT PRIMARY KEY,
			retired_at   DATETIME NOT NULL,
			retire_at    DATETIME NOT NULL,
			store_config TEXT NOT NULL
		)`)
	}
}

// SetEmbedder records the embedder used to rebuild retired stores on restart.
// LoadRetirees injects it into each deserialized StoreConfig because the
// Embedder interface cannot be round-tripped through JSON.
func (r *StoreRegistry) SetEmbedder(e embedding.Embedder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.embedder = e
}

// SetActiveConfig records the config used to build the current active store.
// Switch persists it as the retiree's config when the active store is retired.
// setupRAG should call this after constructing the initial store.
func (r *StoreRegistry) SetActiveConfig(cfg StoreConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activeCfg = cfg
}

// LoadRetirees restores the retired pool from migration.db on restart.
// It reads the retired_stores table, deserializes each StoreConfig, rebuilds
// the store, and inserts it into the registry's retirees map.
//
// StoreConfig.Embedder is an interface and cannot be round-tripped through
// json.Unmarshal — it will be nil after deserialization. LoadRetirees injects
// the registry's embedder (set via SetEmbedder) into each config before calling
// NewStore so the retiree is functional for ListAll/GetByID/DropAll (migration
// and cleanup). If no embedder has been set, the retiree is skipped.
func (r *StoreRegistry) LoadRetirees(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx,
		`SELECT backend, retired_at, retire_at, store_config FROM retired_stores`)
	if err != nil {
		return err
	}
	defer rows.Close()
	r.mu.Lock()
	defer r.mu.Unlock()
	for rows.Next() {
		var bk, cfgJSON string
		var retiredAt, retireAt time.Time
		if err := rows.Scan(&bk, &retiredAt, &retireAt, &cfgJSON); err != nil {
			continue
		}
		var cfg StoreConfig
		if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
			continue
		}
		if cfg.Embedder == nil {
			cfg.Embedder = r.embedder
		}
		store, err := NewStore(cfg)
		if err != nil {
			continue
		}
		r.retirees[BackendType(bk)] = &retiredStore{
			store:     store,
			retiredAt: retiredAt,
			retireAt:  retireAt,
		}
	}
	return nil
}
