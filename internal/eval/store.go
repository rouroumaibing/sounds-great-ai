package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResultStore persists verdicts and evidence bundles to the filesystem.
type ResultStore struct {
	baseDir string // docs/eval-results/
}

// NewResultStore creates a ResultStore rooted at baseDir.
func NewResultStore(baseDir string) *ResultStore {
	return &ResultStore{baseDir: baseDir}
}

// SaveVerdict writes the verdict markdown + evidence bundle to disk.
func (s *ResultStore) SaveVerdict(v *VerdictHandoffPacket) error {
	verdictsDir := filepath.Join(s.baseDir, "verdicts")
	bundlesDir := filepath.Join(s.baseDir, "bundles", v.ID)
	if err := os.MkdirAll(verdictsDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(bundlesDir, 0755); err != nil {
		return err
	}
	// Write verdict markdown
	md := fmt.Sprintf("# Verdict %s\n\nDomain: %s\nVerdict: %s\nPhenomenon: %s\n", v.ID, v.DomainID, v.Verdict, v.Phenomenon)
	if err := os.WriteFile(filepath.Join(verdictsDir, v.ID+".md"), []byte(md), 0644); err != nil {
		return err
	}
	// Write evidence bundle
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(bundlesDir, "evidence.json"), data, 0644)
}

// ListVerdicts returns all verdicts for a domain (empty domainID = all).
func (s *ResultStore) ListVerdicts(domainID string) ([]VerdictHandoffPacket, error) {
	bundlesDir := filepath.Join(s.baseDir, "bundles")
	entries, err := os.ReadDir(bundlesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []VerdictHandoffPacket
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(bundlesDir, entry.Name(), "evidence.json"))
		if err != nil {
			continue
		}
		var v VerdictHandoffPacket
		if err := json.Unmarshal(data, &v); err != nil {
			continue
		}
		if domainID == "" || v.DomainID == domainID {
			result = append(result, v)
		}
	}
	return result, nil
}

// GetVerdict returns a single verdict by ID.
func (s *ResultStore) GetVerdict(verdictID string) (*VerdictHandoffPacket, error) {
	data, err := os.ReadFile(filepath.Join(s.baseDir, "bundles", verdictID, "evidence.json"))
	if err != nil {
		return nil, err
	}
	var v VerdictHandoffPacket
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// verdictIDFromFilename strips the .md extension.
func verdictIDFromFilename(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}
