package memory

import (
	"path/filepath"
	"testing"
)

func TestLaneVectorStoreAndSearch(t *testing.T) {
	dir := t.TempDir()
	reg := NewLaneRegistryAt(filepath.Join(dir, "lanes.json"))
	defer reg.Close()

	a := reg.Lane(LaneDecision).Submit("alpha topic", "s:1")
	b := reg.Lane(LaneLesson).Submit("beta topic", "s:1")
	reg.Lane(LaneDecision).Approve(a.ID)
	reg.Lane(LaneLesson).Approve(b.ID)

	if err := reg.StoreVector(a.ID, []float32{1, 0, 0}); err != nil {
		t.Fatalf("store a: %v", err)
	}
	if err := reg.StoreVector(b.ID, []float32{0, 1, 0}); err != nil {
		t.Fatalf("store b: %v", err)
	}

	// Query near "a" should rank a first.
	hits, ok := reg.SemanticSearch([]float32{0.9, 0.1, 0}, 5, "")
	if !ok || len(hits) == 0 {
		t.Fatalf("semantic search returned nothing")
	}
	if hits[0].ID != a.ID {
		t.Fatalf("expected a first, got %s", hits[0].ID)
	}
}

func TestLaneVectorUnavailableForInMemory(t *testing.T) {
	reg := NewLaneRegistry()
	if err := reg.StoreVector("x", []float32{1}); err == nil {
		t.Fatalf("expected error storing vector without store")
	}
	if _, ok := reg.SemanticSearch([]float32{1}, 5, ""); ok {
		t.Fatalf("expected no semantic results without store")
	}
}
