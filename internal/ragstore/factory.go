// internal/ragstore/factory.go
package ragstore

import (
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
)

// StoreConfig configures a VectorStore backend.
type StoreConfig struct {
	Backend  BackendType
	Embedder embedding.Embedder

	// Memory backend
	PersistPath string // JSON persist path, empty = in-memory only

	// SQLite backend (Phase 2)
	SQLitePath string // .db file path

	// Eino backend (Phase 3)
	EinoIndexer   indexer.Indexer
	EinoRetriever retriever.Retriever
}

func NewStore(cfg StoreConfig) (VectorStore, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("store config: Embedder is required")
	}
	switch cfg.Backend {
	case BackendMemory:
		return NewMemoryStore(cfg.Embedder, cfg.PersistPath), nil
	case BackendSQLite:
		return NewSQLiteStore(cfg.Embedder, cfg.SQLitePath)
	case BackendEino:
		return NewEinoStore(cfg.EinoIndexer, cfg.EinoRetriever)
	default:
		return nil, fmt.Errorf("unknown backend: %s", cfg.Backend)
	}
}
