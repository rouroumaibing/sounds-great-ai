package settings

// NewSettingsStore creates a SettingsStore implementation based on environment.
// Currently only in-memory; future: SQLite/Redis.
func NewSettingsStore() SettingsStore {
	return NewInMemorySettingsStore()
}
