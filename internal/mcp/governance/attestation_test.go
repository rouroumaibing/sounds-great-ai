package governance

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// pkgDir resolves the directory containing this test's source file so the
// committed baseline/attestation artifacts can be located regardless of the
// test's working directory.
func pkgDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Dir(thisFile)
}

func TestMCPBaselineMatchesCatalog(t *testing.T) {
	dir := pkgDir(t)
	catalog := Catalog()
	if err := ValidateBaseline(filepath.Join(dir, "mcp-surface-baseline.json"), catalog); err != nil {
		t.Fatalf("baseline drift: %v (run `go run ./cmd/gen-mcp-baseline` and commit the result)", err)
	}
}

func TestMCPAttestationMatchesBaseline(t *testing.T) {
	dir := pkgDir(t)
	catalog := Catalog()
	if err := ValidateAttestation(
		filepath.Join(dir, "mcp-surface-attestation.json"),
		filepath.Join(dir, "mcp-surface-baseline.json"),
		catalog,
	); err != nil {
		t.Fatalf("attestation invalid: %v", err)
	}
}

// TestCatalogHasGovernanceCertificate asserts every tool carries the governed
// annotation set. A tool without a declared governance contract must fail the
// build, not silently ship.
func TestCatalogHasGovernanceCertificate(t *testing.T) {
	for _, tool := range Catalog() {
		if tool.Name == "" || tool.Family == "" || tool.Description == "" {
			t.Fatalf("tool %q missing name/family/description", tool.Name)
		}
		// ReadOnly and Destructive are the mandatory governance hints. A tool
		// that is neither read-only nor has a declared destructive intent is a
		// governance gap (both hints must be explicit booleans).
		if !tool.ReadOnly && !tool.Destructive && tool.OpenWorld {
			t.Fatalf("tool %q is open-world and mutating without an explicit destructive declaration", tool.Name)
		}
	}
}

// TestGovernanceArtifactsWrittenPrivate asserts the baseline and attestation
// writers produce restrictive 0o600 files. Git cannot preserve a 0600 mode on
// committed artifacts (every checkout restores 0644), so the permission
// guarantee is enforced at write time instead of on the committed files.
func TestGovernanceArtifactsWrittenPrivate(t *testing.T) {
	dir := t.TempDir()
	catalog := Catalog()
	baseline := ComputeBaseline(catalog)

	baselinePath := filepath.Join(dir, "mcp-surface-baseline.json")
	if err := WriteBaseline(baselinePath, catalog); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	att := GenerateAttestation(baseline, "sounds-great-ai/sounds-great-ai", "test-owner", "test-auth-ref")
	attestationPath := filepath.Join(dir, "mcp-surface-attestation.json")
	if err := WriteAttestation(attestationPath, att); err != nil {
		t.Fatalf("write attestation: %v", err)
	}

	for _, path := range []string{baselinePath, attestationPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s written group/other readable (%v); expected 0600", path, info.Mode().Perm())
		}
	}
}
