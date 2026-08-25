// Package convention extracts and queries the convention-layer association
// graph (F242): which artifacts reference which conventions.
package convention

import (
	"sort"
	"sync"
)

// Node is a convention-layer entity: a convention or a referencing artifact.
type Node struct {
	ID    string
	Kind  string // "convention" | "artifact"
	Label string
}

// Edge links a referencing artifact to a convention it observes.
type Edge struct {
	From string // artifact id
	To   string // convention id
	Why  string
}

// Graph is the convention-layer association graph (F242). Single source of truth.
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]Node
	edges []Edge
}

// NewGraph creates an empty graph.
func NewGraph() *Graph { return &Graph{nodes: make(map[string]Node)} }

// AddNode registers a node.
func (g *Graph) AddNode(n Node) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[n.ID] = n
}

// AddEdge records that an artifact references a convention.
func (g *Graph) AddEdge(e Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.edges = append(g.edges, e)
}

// ConventionsFor returns the convention nodes an artifact references.
func (g *Graph) ConventionsFor(artifactID string) []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []Node
	for _, e := range g.edges {
		if e.From == artifactID {
			if n, ok := g.nodes[e.To]; ok {
				out = append(out, n)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ArtifactsObserving returns artifacts that reference a convention.
func (g *Graph) ArtifactsObserving(conventionID string) []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []Node
	for _, e := range g.edges {
		if e.To == conventionID {
			if n, ok := g.nodes[e.From]; ok {
				out = append(out, n)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
