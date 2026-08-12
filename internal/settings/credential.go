package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CredentialStore is the port interface for API key storage.
// Separated from account metadata for security (0o600 vs 0o644).
type CredentialStore interface {
	Get(accountID string) (string, error)
	Set(accountID, apiKey string) error
	Delete(accountID string) error
	Has(accountID string) bool
}

// MemoryCredentialStore is an in-memory CredentialStore for testing.
type MemoryCredentialStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewMemoryCredentialStore() *MemoryCredentialStore {
	return &MemoryCredentialStore{data: make(map[string]string)}
}

func (s *MemoryCredentialStore) Get(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[id]
	if !ok {
		return "", fmt.Errorf("credential %q not found", id)
	}
	return val, nil
}

func (s *MemoryCredentialStore) Set(id, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = key
	return nil
}

func (s *MemoryCredentialStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}

func (s *MemoryCredentialStore) Has(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data[id]
	return ok
}

// FileCredentialStore persists credentials to a JSON file with mode 0o600.
// When watch is true, external edits are picked up ~30s after detection.
type FileCredentialStore struct {
	path   string
	mu     sync.RWMutex
	data   map[string]string
	loaded bool
	reload *HotReloader
}

// NewFileCredentialStore creates a file-backed credential store. watch enables
// hot-reload when the credentials file is modified externally.
func NewFileCredentialStore(path string, watch bool) *FileCredentialStore {
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	s := &FileCredentialStore{path: path, data: make(map[string]string)}
	if watch {
		s.reload = NewHotReloader([]string{path}, func() {
			s.mu.Lock()
			_ = s.reloadFromDisk()
			s.mu.Unlock()
		})
		s.reload.Start()
	}
	return s
}

// ensureLoaded loads the file on first access. Callers must hold s.mu.
func (s *FileCredentialStore) ensureLoaded() error {
	if s.loaded {
		return nil
	}
	return s.reloadFromDisk()
}

// reloadFromDisk re-reads the credentials file, replacing in-memory state.
// Callers must hold s.mu.
func (s *FileCredentialStore) reloadFromDisk() error {
	s.loaded = true
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = make(map[string]string)
			return nil
		}
		return err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		// Back up the corrupt file for recovery, then treat it as empty.
		backupCorrupt(s.path, raw)
		s.data = make(map[string]string)
	}
	return nil
}

func (s *FileCredentialStore) flush() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *FileCredentialStore) Get(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return "", err
	}
	val, ok := s.data[id]
	if !ok {
		return "", fmt.Errorf("credential %q not found", id)
	}
	return val, nil
}

func (s *FileCredentialStore) Set(id, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	s.data[id] = key
	return s.flush()
}

func (s *FileCredentialStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	delete(s.data, id)
	return s.flush()
}

func (s *FileCredentialStore) Has(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return false
	}
	_, ok := s.data[id]
	return ok
}
