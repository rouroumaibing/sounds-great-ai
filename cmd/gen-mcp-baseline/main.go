// Command gen-mcp-baseline regenerates the committed MCP surface baseline and
// attestation files from the live tool catalog. Run it after any change to
// internal/mcp/governance/catalog.go, then commit the resulting JSON. The
// governance test (TestMCPBaselineMatchesCatalog) fails closed if the
// committed baseline drifts from the catalog, so CI enforces re-running this
// generator on catalog edits.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"sounds-great-ai/internal/mcp/governance"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "getwd:", err)
		os.Exit(1)
	}
	// Resolve the governance package directory relative to this command so the
	// generator works regardless of the current working directory.
	pkgDir := filepath.Join(dir, "internal", "mcp", "governance")
	baselinePath := filepath.Join(pkgDir, "mcp-surface-baseline.json")
	attestationPath := filepath.Join(pkgDir, "mcp-surface-attestation.json")

	catalog := governance.Catalog()
	baseline := governance.ComputeBaseline(catalog)

	if err := governance.WriteBaseline(baselinePath, catalog); err != nil {
		fmt.Fprintln(os.Stderr, "write baseline:", err)
		os.Exit(1)
	}

	// The attestation references the baseline. Target repository and owner are
	// fixed SG identities; the authorization ref is the governance decision
	// that authorized the MCP surface (kept as a human-readable pointer).
	att := governance.GenerateAttestation(baseline, "sounds-great-ai/sounds-great-ai", "architecture-cell:mcp-surface-governance", "adr:sg-mcp-surface")
	if err := governance.WriteAttestation(attestationPath, att); err != nil {
		fmt.Fprintln(os.Stderr, "write attestation:", err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s (digest %s, %d tools)\n", baselinePath, baseline.Digest, len(baseline.Tools))
	fmt.Printf("wrote %s\n", attestationPath)
}
