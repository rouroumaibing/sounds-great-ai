package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	LoadPolicyFailFast    = 0
	LoadPolicySkipInvalid = 1
)

// Loader loads breed configs from disk.
type Loader struct {
	Policy int
}

// NewLoader creates a Loader with FailFast policy.
func NewLoader() *Loader {
	return &Loader{Policy: LoadPolicyFailFast}
}

// LoadFromDir reads all *.json files from a directory and parses them as BreedConfigs.
func (l *Loader) LoadFromDir(dir string) (map[string]*BreedConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	breeds := make(map[string]*BreedConfig)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			if l.Policy == LoadPolicySkipInvalid {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		var bc BreedConfig
		if err := json.Unmarshal(data, &bc); err != nil {
			if l.Policy == LoadPolicySkipInvalid {
				continue
			}
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		breeds[bc.ID] = &bc
	}
	return breeds, nil
}
