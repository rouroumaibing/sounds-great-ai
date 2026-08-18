package memory

import (
	"path/filepath"
	"testing"
)

func TestLaneGraphEdgeAndMarker(t *testing.T) {
	dir := t.TempDir()
	reg := NewLaneRegistryAt(filepath.Join(dir, "lanes.json"))
	defer reg.Close()

	// Submit two candidates and approve them so they are real entries.
	a := reg.Lane(LaneDecision).Submit("decided to use Postgres", "session:1")
	b := reg.Lane(LaneLesson).Submit("lesson: avoid N+1 queries", "session:1")
	if !reg.Lane(LaneDecision).Approve(a.ID) {
		t.Fatalf("approve a failed")
	}
	if !reg.Lane(LaneLesson).Approve(b.ID) {
		t.Fatalf("approve b failed")
	}

	// Link a -> b as supersedes.
	edge, err := reg.AddEdge(a.ID, b.ID, RelationSupersedes, "operator")
	if err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if edge.Relation != RelationSupersedes {
		t.Fatalf("expected supersedes, got %q", edge.Relation)
	}

	// Unknown relation rejected.
	if _, err := reg.AddEdge(a.ID, b.ID, LaneRelation("bogus"), "operator"); err == nil {
		t.Fatalf("expected error for unknown relation")
	}

	edges := reg.Edges(a.ID)
	if len(edges) != 1 || edges[0].ToID != b.ID {
		t.Fatalf("Edges mismatch: %+v", edges)
	}

	// Marker.
	m, err := reg.AddMarker(a.ID, "decision", "chose Postgres over Mongo", "operator")
	if err != nil {
		t.Fatalf("AddMarker: %v", err)
	}
	if m.Status != "captured" {
		t.Fatalf("expected captured, got %q", m.Status)
	}
	markers := reg.Markers(a.ID)
	if len(markers) != 1 {
		t.Fatalf("Markers mismatch: %+v", markers)
	}

	// Graph persists across reopen.
	reg.Close()
	reopened := NewLaneRegistryAt(filepath.Join(dir, "lanes.json"))
	defer reopened.Close()
	if len(reopened.Edges(a.ID)) != 1 {
		t.Fatalf("edge not persisted")
	}
	if len(reopened.Markers(a.ID)) != 1 {
		t.Fatalf("marker not persisted")
	}
}

func TestLaneGraphUnavailableForInMemoryRegistry(t *testing.T) {
	reg := NewLaneRegistry()
	if _, err := reg.AddEdge("x", "y", RelationRelated, "op"); err == nil {
		t.Fatalf("expected error when graph store unavailable")
	}
	if reg.Edges("x") != nil {
		t.Fatalf("expected nil edges for in-memory registry")
	}
}
