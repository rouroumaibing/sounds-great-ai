package threadstore

// StoreConfig controls which storage backend is used.
type StoreConfig struct {
	SQLitePath string // empty = in-memory
}

// NewMessageStore creates a MessageStore using the configured backend.
func NewMessageStore(cfg StoreConfig) (MessageStore, error) {
	if cfg.SQLitePath != "" {
		return NewSQLiteMessageStore(cfg.SQLitePath)
	}
	return NewMemoryMessageStore(), nil
}

// NewThreadStore creates a ThreadStore using the configured backend.
func NewThreadStore(cfg StoreConfig) (ThreadStore, error) {
	if cfg.SQLitePath != "" {
		return NewSQLiteThreadStore(cfg.SQLitePath)
	}
	return NewInMemoryThreadStore(), nil
}
