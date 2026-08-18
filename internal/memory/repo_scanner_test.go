package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepoScannerExtractsTypedCandidates verifies the deterministic Markdown
// scanner surfaces typed candidates (decision/lesson/preference/profile) with a
// repo:<relpath> source — homologous clowder GenericRepoScanner, no LLM.
func TestRepoScannerExtractsTypedCandidates(t *testing.T) {
	dir := t.TempDir()
	doc := `# Project Notes

We decided to adopt event sourcing for the audit log.
Correction: the previous cache layer was wrong.
I prefer async communication between services.
I am the platform tech lead.
`
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cands := NewRepoScanner().Scan(dir)
	if len(cands) == 0 {
		t.Fatal("expected at least one candidate, got none")
	}

	byLane := map[LaneType][]string{}
	for _, c := range cands {
		byLane[c.Lane] = append(byLane[c.Lane], c.Content)
		if !strings.HasPrefix(c.Source, "repo:") {
			t.Fatalf("expected repo: source, got %q", c.Source)
		}
	}

	if len(byLane[LaneDecision]) == 0 {
		t.Fatalf("expected a decision candidate, got lanes=%v", byLane)
	}
	if len(byLane[LaneLesson]) == 0 {
		t.Fatalf("expected a lesson candidate, got lanes=%v", byLane)
	}
	if len(byLane[LaneTaste]) == 0 {
		t.Fatalf("expected a preference (taste) candidate, got lanes=%v", byLane)
	}
	if len(byLane[LaneProfile]) == 0 {
		t.Fatalf("expected an identity (profile) candidate, got lanes=%v", byLane)
	}
}

// TestRepoScannerSkipsIgnoredDirs ensures .git / node_modules / hidden dirs
// are not walked (so scanning the repo root is safe and scoped).
func TestRepoScannerSkipsIgnoredDirs(t *testing.T) {
	dir := t.TempDir()
	// A candidate inside node_modules must be ignored.
	_ = os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755)
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "x.md"),
		[]byte("We decided to ship it."), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}
	// A candidate at the top level must be found.
	if err := os.WriteFile(filepath.Join(dir, "top.md"),
		[]byte("We decided to keep it simple."), 0o644); err != nil {
		t.Fatalf("write top file: %v", err)
	}

	cands := NewRepoScanner().Scan(dir)
	// node_modules content must be excluded entirely (scanner skips ignored dirs).
	for _, c := range cands {
		if strings.Contains(c.Source, "node_modules") {
			t.Fatalf("node_modules content leaked into candidates: %q", c.Source)
		}
	}
	if len(cands) == 0 {
		t.Fatalf("expected the top-level candidate to be found, got none")
	}
}

// TestRepoScannerRunScanSubmitsPending confirms RunScan submits detected
// candidates as pending entries scoped to the operator (idempotent dedup).
func TestRepoScannerRunScanSubmitsPending(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doc.md"),
		[]byte("We decided to use SQLite for the lane store."), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	reg := NewLaneRegistryAt(filepath.Join(t.TempDir(), "lanes.json"))
	defer reg.Close()

	scanner := NewRepoScanner()
	n1 := scanner.RunScan(reg, dir, "operator")
	if n1 <= 0 {
		t.Fatalf("expected at least 1 submitted candidate, got %d", n1)
	}
	// Re-scanning the same content must not duplicate (idempotent dedup).
	n2 := scanner.RunScan(reg, dir, "operator")
	if n2 != 0 {
		t.Fatalf("expected 0 duplicates on re-scan, got %d", n2)
	}
}
