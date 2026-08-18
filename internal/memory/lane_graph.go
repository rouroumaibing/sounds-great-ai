package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (shared with lane persister)
)

// LaneRelation is the typed relationship between two entries (homologous
// clowder edges). clowder defines 10 relations; SG now matches the full set so
// relationship semantics are as rich as clowder's knowledge graph.
type LaneRelation string

const (
	RelationEvolvedFrom  LaneRelation = "evolved_from"
	RelationBlockedBy    LaneRelation = "blocked_by"
	RelationSupersedes   LaneRelation = "supersedes"
	RelationInvalidates  LaneRelation = "invalidates"
	RelationRelated      LaneRelation = "related"
	RelationRelatedTo    LaneRelation = "related_to"
	RelationPromotedFrom LaneRelation = "promoted_from"
	RelationWikilink     LaneRelation = "wikilink"
	RelationDocLink      LaneRelation = "doc_link"
	RelationFeatureRef   LaneRelation = "feature_ref"
)

// ValidRelation reports whether rel is a known edge relation (all 10).
func ValidRelation(rel LaneRelation) bool {
	switch rel {
	case RelationEvolvedFrom, RelationBlockedBy, RelationSupersedes, RelationInvalidates,
		RelationRelated, RelationRelatedTo, RelationPromotedFrom, RelationWikilink,
		RelationDocLink, RelationFeatureRef:
		return true
	}
	return false
}

// LaneEdge is a directed typed link between two entries (clowder edge). It
// carries edge-level sensitivity + provenance + traversal telemetry (clowder
// V18 edge columns), so a private link can stay hidden even when both endpoints
// are visible, and traversal frequency surfaces the most-traveled paths.
type LaneEdge struct {
	ID             string      `json:"id"`
	FromID         string      `json:"from_id"`
	ToID           string      `json:"to_id"`
	Relation       LaneRelation `json:"relation"`
	EdgeSensitivity string     `json:"edge_sensitivity"` // "" = inherit; else public/internal/private/restricted
	Provenance     string      `json:"provenance"`       // e.g. "session_seal" | "manual" | "import"
	TraversalCount int         `json:"traversal_count"`
	LastTraversedAt int64      `json:"last_traversed_at"`
	OperatorID     string      `json:"operator_id"`
	Timestamp      int64       `json:"timestamp"`
}

// LaneMarker is a normalized signal attached to an entry (homologous clowder
// marker: captured/normalized/approved/rejected). It records *why* an entry
// matters (a decision signal, a lesson signal, a correction) without promoting
// the marker to a full lane entry.
type LaneMarker struct {
	ID         string `json:"id"`
	EntryID    string `json:"entry_id"`
	MarkerType string `json:"marker_type"` // e.g. decision/lesson/correction
	Content    string `json:"content"`
	Status     string `json:"status"` // captured/normalized/approved/rejected
	OperatorID string `json:"operator_id"`
	Timestamp  int64  `json:"timestamp"`
}

// graphStore persists edges + markers in a dedicated SQLite DB next to the lane
// store (path + ".graph.db"). It is intentionally a separate file so the graph
// schema evolves independently of the lane schema (no migration coupling).
type graphStore struct {
	db *sql.DB
}

func openGraphDB(path string) (*graphStore, error) {
	if path == "" {
		return nil, fmt.Errorf("memory graph: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+".graph.db")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(3)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS lane_edge (
		id TEXT PRIMARY KEY, from_id TEXT, to_id TEXT, relation TEXT,
		edge_sensitivity TEXT DEFAULT '', provenance TEXT DEFAULT '',
		traversal_count INTEGER DEFAULT 0, last_traversed_at INTEGER DEFAULT 0,
		operator_id TEXT, timestamp INTEGER)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS lane_marker (
		id TEXT PRIMARY KEY, entry_id TEXT, marker_type TEXT, content TEXT,
		status TEXT, operator_id TEXT, timestamp INTEGER)`); err != nil {
		db.Close()
		return nil, err
	}
	// Best-effort migration for stores created before edge telemetry columns
	// existed (old .graph.db files). Idempotent.
	ensureGraphColumn(db, "lane_edge", "edge_sensitivity", "TEXT DEFAULT ''")
	ensureGraphColumn(db, "lane_edge", "provenance", "TEXT DEFAULT ''")
	ensureGraphColumn(db, "lane_edge", "traversal_count", "INTEGER DEFAULT 0")
	ensureGraphColumn(db, "lane_edge", "last_traversed_at", "INTEGER DEFAULT 0")
	return &graphStore{db: db}, nil
}

func ensureGraphColumn(db *sql.DB, table, col, colType string) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return
		}
		if name == col {
			return
		}
	}
	_, _ = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, colType))
}

func (g *graphStore) addEdge(e *LaneEdge) error {
	_, err := g.db.Exec(`INSERT OR REPLACE INTO lane_edge
		(id, from_id, to_id, relation, edge_sensitivity, provenance, traversal_count, last_traversed_at, operator_id, timestamp)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.FromID, e.ToID, string(e.Relation), e.EdgeSensitivity, e.Provenance,
		e.TraversalCount, e.LastTraversedAt, e.OperatorID, e.Timestamp)
	return err
}

func (g *graphStore) edges(fromID string) []*LaneEdge {
	rows, err := g.db.Query(`SELECT id, from_id, to_id, relation, edge_sensitivity, provenance,
		traversal_count, last_traversed_at, operator_id, timestamp
		FROM lane_edge WHERE from_id=? ORDER BY timestamp`, fromID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []*LaneEdge
	for rows.Next() {
		var e LaneEdge
		var rel, sens, prov string
		if err := rows.Scan(&e.ID, &e.FromID, &e.ToID, &rel, &sens, &prov,
			&e.TraversalCount, &e.LastTraversedAt, &e.OperatorID, &e.Timestamp); err != nil {
			return out
		}
		e.Relation = LaneRelation(rel)
		e.EdgeSensitivity = sens
		e.Provenance = prov
		out = append(out, &e)
	}
	return out
}

// touchEdge bumps traversal telemetry (homologous clowder last_traversed_at).
func (g *graphStore) touchEdge(id string) {
	now := time.Now().UnixMilli()
	_, _ = g.db.Exec(`UPDATE lane_edge SET traversal_count = traversal_count + 1, last_traversed_at = ? WHERE id = ?`, now, id)
}

func (g *graphStore) addMarker(m *LaneMarker) error {
	_, err := g.db.Exec(`INSERT OR REPLACE INTO lane_marker
		(id, entry_id, marker_type, content, status, operator_id, timestamp)
		VALUES (?,?,?,?,?,?,?)`,
		m.ID, m.EntryID, m.MarkerType, m.Content, m.Status, m.OperatorID, m.Timestamp)
	return err
}

func (g *graphStore) markers(entryID string) []*LaneMarker {
	rows, err := g.db.Query(`SELECT id, entry_id, marker_type, content, status, operator_id, timestamp
		FROM lane_marker WHERE entry_id=? ORDER BY timestamp`, entryID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []*LaneMarker
	for rows.Next() {
		var m LaneMarker
		if err := rows.Scan(&m.ID, &m.EntryID, &m.MarkerType, &m.Content, &m.Status, &m.OperatorID, &m.Timestamp); err != nil {
			return out
		}
		out = append(out, &m)
	}
	return out
}

func (g *graphStore) close() { _ = g.db.Close() }

// ---- LaneRegistry graph API (edges + markers) ----

// AddEdge links from→to with a typed relation (5-arg convenience, edge-sensitivity
// inherited). See AddEdgeFull for edge-level sensitivity/provenance.
func (r *LaneRegistry) AddEdge(from, to string, rel LaneRelation, operator string) (*LaneEdge, error) {
	return r.AddEdgeFull(from, to, rel, "", "manual", operator)
}

// AddEdgeFull links from→to with a typed relation plus edge-level sensitivity
// and provenance (homologous clowder V18 edge columns). Both IDs must be known
// entries; the relation must be one of the 10 LaneRelation constants.
func (r *LaneRegistry) AddEdgeFull(from, to string, rel LaneRelation, edgeSensitivity, provenance, operator string) (*LaneEdge, error) {
	if r.graph == nil {
		return nil, fmt.Errorf("memory graph: store unavailable")
	}
	if !ValidRelation(rel) {
		return nil, fmt.Errorf("memory graph: unknown relation %q", rel)
	}
	if from == "" || to == "" {
		return nil, fmt.Errorf("memory graph: from/to required")
	}
	if edgeSensitivity != "" && !ValidSensitivity(edgeSensitivity) {
		return nil, fmt.Errorf("memory graph: invalid edge_sensitivity %q", edgeSensitivity)
	}
	if provenance == "" {
		provenance = "manual"
	}
	e := &LaneEdge{
		ID:             uuid.NewString(),
		FromID:         from,
		ToID:           to,
		Relation:       rel,
		EdgeSensitivity: edgeSensitivity,
		Provenance:     provenance,
		TraversalCount: 0,
		LastTraversedAt: 0,
		OperatorID:     operator,
		Timestamp:      time.Now().UnixMilli(),
	}
	if err := r.graph.addEdge(e); err != nil {
		return nil, err
	}
	return e, nil
}

// Edges returns outgoing edges from a given entry.
func (r *LaneRegistry) Edges(from string) []*LaneEdge {
	if r.graph == nil {
		return nil
	}
	return r.graph.edges(from)
}

// TouchEdge records that an edge was traversed (recall/inspection), updating
// traversal telemetry (homologous clowder last_traversed_at).
func (r *LaneRegistry) TouchEdge(id string) {
	if r.graph == nil {
		return
	}
	r.graph.touchEdge(id)
}

// AddMarker attaches a normalized signal to an entry (status starts "captured").
func (r *LaneRegistry) AddMarker(entryID, markerType, content, operator string) (*LaneMarker, error) {
	if r.graph == nil {
		return nil, fmt.Errorf("memory graph: store unavailable")
	}
	if entryID == "" || markerType == "" {
		return nil, fmt.Errorf("memory graph: entry_id/marker_type required")
	}
	m := &LaneMarker{
		ID:         uuid.NewString(),
		EntryID:    entryID,
		MarkerType: markerType,
		Content:    content,
		Status:     "captured",
		OperatorID: operator,
		Timestamp:  time.Now().UnixMilli(),
	}
	if err := r.graph.addMarker(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Markers returns the markers attached to an entry.
func (r *LaneRegistry) Markers(entryID string) []*LaneMarker {
	if r.graph == nil {
		return nil
	}
	return r.graph.markers(entryID)
}

// Graph returns the outgoing edges and markers for an entry (homologous
// clowder edge/marker inspection). Used by the frontend to render the
// relationship graph around a memory entry.
func (r *LaneRegistry) Graph(id string) (edges []*LaneEdge, markers []*LaneMarker) {
	return r.Edges(id), r.Markers(id)
}
