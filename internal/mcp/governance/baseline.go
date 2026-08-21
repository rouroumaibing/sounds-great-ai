package governance

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// Baseline constants. The FeatureID namespaces SG's governed MCP surface.
const (
	BaselineSchemaVersion = 1
	BaselineFeatureID     = "SG-MCP-SURFACE"
)

// BaselineEntry is the per-tool, governance-relevant projection that enters the
// baseline digest. It deliberately excludes volatile fields (description,
// path) so a doc-only edit does not invalidate the surface contract.
type BaselineEntry struct {
	Name        string `json:"name"`
	Family      string `json:"family"`
	ReadOnly    bool   `json:"read_only"`
	Destructive bool   `json:"destructive"`
	Idempotent  bool   `json:"idempotent"`
	OpenWorld   bool   `json:"open_world"`
}

// Baseline is the committed, signed-off tool surface. `Digest` is a sha256 over
// the canonical (name-sorted) entry list and is what the attestation pins.
type Baseline struct {
	SchemaVersion int             `json:"schemaVersion"`
	FeatureID     string          `json:"featureId"`
	GeneratedAt   string          `json:"generatedAt"`
	Tools         []BaselineEntry `json:"tools"`
	Digest        string          `json:"digest"`
}

// canonicalEntries returns the catalog projected to baseline entries, sorted by
// name, for a stable digest.
func canonicalEntries(catalog []ToolDefinition) []BaselineEntry {
	entries := make([]BaselineEntry, 0, len(catalog))
	for _, t := range catalog {
		entries = append(entries, BaselineEntry{
			Name:        t.Name,
			Family:      t.Family,
			ReadOnly:    t.ReadOnly,
			Destructive: t.Destructive,
			Idempotent:  t.Idempotent,
			OpenWorld:   t.OpenWorld,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// ComputeBaseline builds a Baseline (with digest) from the live catalog. The
// digest covers only the canonical entry list, so GeneratedAt does not affect
// it.
func ComputeBaseline(catalog []ToolDefinition) Baseline {
	entries := canonicalEntries(catalog)
	raw, _ := json.Marshal(entries)
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	return Baseline{
		SchemaVersion: BaselineSchemaVersion,
		FeatureID:     BaselineFeatureID,
		Tools:         entries,
		Digest:        digest,
	}
}

// WriteBaseline writes a baseline file (0o600) from the live catalog. The
// generator command (cmd/gen-mcp-baseline) calls this; do not edit the
// produced JSON by hand — re-run the generator.
func WriteBaseline(path string, catalog []ToolDefinition) error {
	b := ComputeBaseline(catalog)
	b.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ValidateBaseline recomputes the digest from the live catalog and compares it
// to the committed baseline file. It fails closed (returns an error) on any
// drift, so the committed baseline cannot be silently rotated without
// re-running the generator.
func ValidateBaseline(baselinePath string, catalog []ToolDefinition) error {
	committed, err := os.ReadFile(baselinePath)
	if err != nil {
		return fmt.Errorf("mcp surface baseline missing (%s): %w", baselinePath, err)
	}
	var b Baseline
	if err := json.Unmarshal(committed, &b); err != nil {
		return fmt.Errorf("mcp surface baseline corrupt: %w", err)
	}
	live := ComputeBaseline(catalog)
	if live.Digest != b.Digest {
		return fmt.Errorf("mcp surface drift detected: live digest %s != committed baseline %s (run `go run ./cmd/gen-mcp-baseline`)", live.Digest, b.Digest)
	}
	return nil
}
