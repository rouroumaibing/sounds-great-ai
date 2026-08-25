package convention

import "testing"

func TestGraph_BidirectionalQuery(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{ID: "conv-a", Kind: "convention", Label: "naming"})
	g.AddNode(Node{ID: "conv-b", Kind: "convention", Label: "errors"})
	g.AddNode(Node{ID: "art-x", Kind: "artifact", Label: "handler.go"})
	g.AddEdge(Edge{From: "art-x", To: "conv-a", Why: "follows naming"})
	g.AddEdge(Edge{From: "art-x", To: "conv-b", Why: "follows errors"})

	cons := g.ConventionsFor("art-x")
	if len(cons) != 2 {
		t.Fatalf("expected 2 conventions for art-x, got %d", len(cons))
	}
	arts := g.ArtifactsObserving("conv-a")
	if len(arts) != 1 || arts[0].ID != "art-x" {
		t.Fatalf("expected art-x observing conv-a, got %+v", arts)
	}
}
