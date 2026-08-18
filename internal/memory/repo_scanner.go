package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// RepoScanner scans repository Markdown for typed memory candidates, providing
// a second candidate source beyond session-close text (homologous clowder
// GenericRepoScanner). It reuses the same deterministic pattern extractors as
// the session DeltaProducer — no LLM is involved (VISION §3). Scanning is
// opt-in (cmd/memory scan), never auto-wired to session seal, so it cannot
// flood the pending queue unnoticed.
type RepoScanner struct{}

// NewRepoScanner creates a new RepoScanner.
func NewRepoScanner() *RepoScanner { return &RepoScanner{} }

// Scan walks root for .md files (skipping .git, node_modules, and hidden
// dirs), extracts typed candidates from each file's text, and returns them
// with a repo:<relpath> source. It does not submit — the caller decides.
func (rs *RepoScanner) Scan(root string) []DeltaCandidate {
	var candidates []DeltaCandidate
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		source := "repo:" + rel
		content := string(data)
		for _, d := range extractDecisions(content) {
			candidates = append(candidates, DeltaCandidate{Lane: LaneDecision, Content: d, Source: source})
		}
		for _, c := range extractCorrections(content) {
			candidates = append(candidates, DeltaCandidate{Lane: LaneLesson, Content: c, Source: source})
		}
		for _, id := range extractIdentityChanges(content) {
			candidates = append(candidates, DeltaCandidate{Lane: LaneProfile, Content: id, Source: source})
		}
		for _, p := range extractPreferences(content) {
			candidates = append(candidates, DeltaCandidate{Lane: LaneTaste, Content: p, Source: source})
		}
		return nil
	})
	return candidates
}

// RunScan scans root and submits detected candidates to the registry, scoped
// to operator. Returns the number of new entries submitted (duplicates already
// present in the lane are skipped by SubmitCandidates' idempotent dedup).
func (rs *RepoScanner) RunScan(reg *LaneRegistry, root, operator string) int {
	candidates := rs.Scan(root)
	dp := NewDeltaProducer()
	ids := dp.SubmitCandidates(reg, candidates, operator)
	return len(ids)
}
