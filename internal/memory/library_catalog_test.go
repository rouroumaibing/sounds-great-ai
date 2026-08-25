package memory

import (
	"strings"
	"testing"
)

// newTestCollection builds a collection backed by an in-memory registry with a
// couple of lanes pre-loaded, for use in catalog/scanner tests. Seeded entries
// are promoted to canonical truth (approved) so L1 scans return them.
func newTestCollection(id, name string, level ScanLevel) *Collection {
	reg := NewLaneRegistry()
	e1 := reg.Lane(LaneTaste).Submit("user prefers warm tonal palettes", "seed")
	e2 := reg.Lane(LaneProfile).Submit("operator is a sound designer", "seed")
	// Promote to canonical truth so L1 scans return approved entries.
	reg.Lane(LaneTaste).Approve(e1.ID)
	reg.Lane(LaneProfile).Approve(e2.ID)
	return &Collection{
		ID:        id,
		Name:      name,
		ACLRef:    string(SensInternal),
		ScanLevel: level,
		Registry:  reg,
	}
}

// Invariant 1: cross-Collection federated search returns merged results that
// retain the source Collection marker for each hit.
func TestFederatedSearch_MergesWithSource(t *testing.T) {
	cat := NewLibraryCatalog()
	cA := newTestCollection("colA", "Alpha", LevelL2)
	cB := newTestCollection("colB", "Beta", LevelL2)
	if err := cat.Register(cA); err != nil {
		t.Fatalf("register A: %v", err)
	}
	if err := cat.Register(cB); err != nil {
		t.Fatalf("register B: %v", err)
	}

	// Query matching entries in BOTH collections.
	results, err := cat.FederatedSearch("operator", []string{"colA", "colB"}, "")
	if err != nil {
		t.Fatalf("federated search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one merged result, got none")
	}

	// Every result must carry a source marker, and both collections must be
	// represented (i.e. the merge actually crossed collections).
	seen := map[string]bool{}
	for _, r := range results {
		if r.SourceID == "" {
			t.Errorf("result missing source id: %+v", r)
		}
		if r.Entry == nil {
			t.Errorf("result missing entry for source %q", r.SourceID)
		}
		seen[r.SourceID] = true
	}
	if !seen["colA"] || !seen["colB"] {
		t.Errorf("merge did not span both collections; saw sources=%v", seen)
	}
}

// Invariant 1b: an unknown collection id is a hard error (no silent shard miss).
func TestFederatedSearch_UnknownCollectionErrors(t *testing.T) {
	cat := NewLibraryCatalog()
	cat.Register(newTestCollection("colA", "Alpha", LevelL2))
	if _, err := cat.FederatedSearch("x", []string{"colA", "ghost"}, ""); err == nil {
		t.Fatal("expected error for unknown collection, got nil")
	}
}

// Invariant 2: the secret scan gate is fail-closed — content containing a
// credential must be intercepted and denied (no partial results leak).
func TestSecretScanGate_FailClosed(t *testing.T) {
	s := NewScanner()
	if err := s.ScanContent("api_key=AKIAIOSFODNN7EXAMPLE"); err == nil {
		t.Fatal("gate must deny api_key content, got nil")
	}
	if err := s.ScanContent("here is my password: hunter2hunter2hunter2hunter2"); err == nil {
		t.Fatal("gate must deny password content, got nil")
	}
	// Clean content passes.
	if err := s.ScanContent("user prefers warm tonal palettes"); err != nil {
		t.Fatalf("gate must allow clean content, got %v", err)
	}
}

// Invariant 2b: a Scan over a collection whose entries contain a secret is
// denied wholesale (fail-closed), never returning the leaking entry.
func TestScanner_ScanFailClosedOnSecret(t *testing.T) {
	reg := NewLaneRegistry()
	reg.Lane(LaneTaste).Submit("normal preference: likes reverb", "seed")
	reg.Lane(LaneProfile).Submit("token=sk_test_abcdefghijklmnopqrstuvwxyz012345", "seed")
	col := &Collection{ID: "secretCol", Name: "Secret", ACLRef: string(SensRestricted), ScanLevel: LevelL3, Registry: reg}

	s := NewScanner()
	got, err := s.Scan(col, LevelL3, "")
	if err == nil {
		t.Fatalf("scan must be denied on secret content, got %d entries", len(got))
	}
	if len(got) != 0 {
		t.Errorf("fail-closed scan must return no entries, got %d", len(got))
	}
	// The secret message must mention the gate so callers know why it failed.
	if !strings.Contains(err.Error(), "secret scan gate") {
		t.Errorf("error should reference the secret gate: %v", err)
	}
}

// Sanity: a clean collection scans successfully at the permitted level.
func TestScanner_ScanClean(t *testing.T) {
	col := newTestCollection("clean", "Clean", LevelL2)
	s := NewScanner()
	got, err := s.Scan(col, LevelL1, "")
	if err != nil {
		t.Fatalf("clean scan should succeed: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected approved truth entries at L1, got none")
	}
}
