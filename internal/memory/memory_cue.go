package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (shared with lane persister)
)

// laneWeight biases cue selection toward higher-leverage memory types
// (decisions + lessons matter most for a fresh task; taste least).
func laneWeight(t LaneType) float64 {
	switch t {
	case LaneDecision, LaneLesson:
		return 1.0
	case LaneEvent:
		return 0.8
	case LaneProfile, LanePerson:
		return 0.7
	case LaneEntity:
		return 0.6
	case LaneTaste:
		return 0.5
	}
	return 0.5
}

func tokenize(s string) []string {
	return strings.Fields(strings.ToLower(strings.TrimSpace(s)))
}

// keywordOverlap returns the fraction of hint tokens present in content
// (case-insensitive). A 0 hint yields 0 (no topical signal).
func keywordOverlap(content string, hintTokens []string) float64 {
	if len(hintTokens) == 0 {
		return 0
	}
	low := strings.ToLower(content)
	hits := 0
	for _, tok := range hintTokens {
		if strings.Contains(low, tok) {
			hits++
		}
	}
	return float64(hits) / float64(len(hintTokens))
}

// CueHit is a ranked, opportunity-scored approved-truth entry (Gap4 cue-plane).
// Carries the
// deterministic score so the injection path can also append a consumption
// ledger event (mem_cue_events).
type CueHit struct {
	Entry *LaneEntry `json:"entry"`
	Score float64    `json:"score"`
	Rank  int        `json:"rank"`
}

// CueMemory returns a token-bounded markdown block of approved truth, ranked by
// a deterministic "opportunity" score rather than raw insertion order
// (surface the most
// relevant approved truth for the current context, not a flat dump).
//
// The score blends three signals, all deterministic (no LLM — VISION §3):
//   - recency: newer truth ranks higher (30-day half-life decay)
//   - leverage: decision/lesson truth ranks higher than taste
//   - relevance: overlap between the hint (e.g. breed id / task topic) and the
//     truth content ranks higher
//
// hint "" disables the relevance signal (pure recency+leverage ranking).
func (r *LaneRegistry) CueMemory(maxLines int, operator, hint string) (string, bool, error) {
	hits, ok, err := r.CueMemoryRanked(maxLines, operator, hint)
	if err != nil || !ok {
		return "", false, err
	}
	lines := make([]string, 0, len(hits))
	for _, h := range hits {
		lines = append(lines, fmt.Sprintf("- [%s] %s", h.Entry.Type, h.Entry.Content))
	}
	return strings.Join(lines, "\n"), true, nil
}

// CueMemoryRanked is the scored variant of CueMemory (Gap4 + cue ledger). It
// returns up to maxLines approved, visible entries ranked by opportunity score
// with Rank populated. The caller (prompt builder) formats the block and may
// append the hits to the consumption ledger via RecordCueEvents. Deterministic,
// no LLM (VISION §3).
func (r *LaneRegistry) CueMemoryRanked(maxLines int, operator, hint string) ([]CueHit, bool, error) {
	if maxLines <= 0 {
		maxLines = 20
	}
	// Pull a wider candidate pool, then rank and truncate to maxLines.
	pool, ok := r.RecallEntries(maxLines*3, operator)
	if !ok {
		return nil, false, nil
	}
	hintTokens := tokenize(hint)
	now := time.Now().UnixMilli()

	type scored struct {
		e *LaneEntry
		s float64
	}
	scoredEntries := make([]scored, 0, len(pool))
	for _, e := range pool {
		ageDays := float64(now-e.Timestamp) / (1000 * 3600 * 24)
		recency := 1.0 / (1.0 + ageDays/30.0)
		score := 0.4*recency + 0.3*laneWeight(e.Type) + 0.3*keywordOverlap(e.Content, hintTokens)
		scoredEntries = append(scoredEntries, scored{e, score})
	}
	sort.SliceStable(scoredEntries, func(i, j int) bool {
		return scoredEntries[i].s > scoredEntries[j].s
	})
	if len(scoredEntries) > maxLines {
		scoredEntries = scoredEntries[:maxLines]
	}
	hits := make([]CueHit, 0, len(scoredEntries))
	for i, se := range scoredEntries {
		hits = append(hits, CueHit{Entry: se.e, Score: se.s, Rank: i + 1})
	}
	return hits, true, nil
}

// ---- cue consumption ledger (mem_cue_events) ----

// CueEvent is one append-only record that a cue was injected for an entry.
type CueEvent struct {
	ID      string  `json:"id"`
	EntryID string  `json:"entry_id"`
	Lane    string  `json:"lane"`
	Rank    int     `json:"rank"`
	Score   float64 `json:"score"`
	Consumed bool   `json:"consumed"` // true once surfaced into a prompt
	OperatorID string `json:"operator_id"`
	Timestamp int64  `json:"timestamp"`
}

type cueStore struct {
	db *sql.DB
}

func openCueDB(path string) (*cueStore, error) {
	if path == "" {
		return nil, fmt.Errorf("memory cue: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+".cue.db")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(3)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS mem_cue_event (
		id TEXT PRIMARY KEY, entry_id TEXT, lane TEXT, rank INTEGER,
		score REAL, consumed INTEGER DEFAULT 1, operator_id TEXT, timestamp INTEGER)`); err != nil {
		db.Close()
		return nil, err
	}
	return &cueStore{db: db}, nil
}

// RecordCueEvents appends a cue-consumption ledger entry for each injected hit.
// Fail-open: errors are logged, never block.
func (r *LaneRegistry) RecordCueEvents(hits []CueHit, operator string) {
	if r.cue == nil {
		return
	}
	for _, h := range hits {
		_, err := r.cue.db.Exec(
			`INSERT INTO mem_cue_event (id, entry_id, lane, rank, score, consumed, operator_id, timestamp)
			 VALUES (?,?,?,?,?,1,?,?)`,
			uuid.NewString(), h.Entry.ID, string(h.Entry.Type), h.Rank, h.Score, operator, time.Now().UnixMilli())
		if err != nil {
			// best-effort observability; never block injection
			continue
		}
	}
}

// CueEvents returns recent cue-consumption ledger entries (newest first).
func (r *LaneRegistry) CueEvents(limit int) []CueEvent {
	if r.cue == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.cue.db.Query(
		`SELECT id, entry_id, lane, rank, score, consumed, operator_id, timestamp
		 FROM mem_cue_event ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []CueEvent
	for rows.Next() {
		var e CueEvent
		var consumed int
		if err := rows.Scan(&e.ID, &e.EntryID, &e.Lane, &e.Rank, &e.Score, &consumed, &e.OperatorID, &e.Timestamp); err != nil {
			return out
		}
		e.Consumed = consumed != 0
		out = append(out, e)
	}
	return out
}

func (c *cueStore) close() { _ = c.db.Close() }
