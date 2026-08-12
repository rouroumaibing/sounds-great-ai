package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Loader loads breed configs from disk into a map. It is a standalone helper
// used for breed-existence checks and consolidated-template reads; it does NOT
// mutate the Pack registry (unlike Pack.LoadFromDir / Pack.LoadFromFile).
type Loader struct {
	Policy LoadPolicy
}

// NewLoader creates a Loader with FailFast policy.
func NewLoader() *Loader {
	return &Loader{Policy: LoadPolicyFailFast}
}

// LoadFromDir reads all *.json files from a directory and parses them as
// BreedConfigs, returning a map keyed by breed ID.
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
		// Skip non-breed artifacts (e.g. dog-template.json) that live alongside
		// breed configs. These are valid JSON but not BreedConfig objects.
		if strings.Contains(strings.TrimSuffix(entry.Name(), ".json"), "template") {
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

// LoadFromFile reads a single consolidated template file (dog-template.json)
// and returns every breed found in its "breeds" array. A missing file is
// treated as an empty pack (nil error).
func (l *Loader) LoadFromFile(path string) (map[string]*BreedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*BreedConfig{}, nil
		}
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	var tmpl DogTemplateFile
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parse file %s: %w", path, err)
	}
	breeds := make(map[string]*BreedConfig)
	for i := range tmpl.Breeds {
		breeds[tmpl.Breeds[i].ID] = &tmpl.Breeds[i]
	}
	return breeds, nil
}
