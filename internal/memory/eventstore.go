package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (shared with lane/graph/vector/cue stores)
)

// EventType is the semantic class of a cognitive-state transition recorded in
// Event Memory. The kernel is event-level recall (F227), not a magic-word
// panel — magic words are only the first lane. SG records these deterministically;
// no classifier, no LLM (VISION §3).
type EventType string

const (
	// EventTypeMagicWord is the first lane: an operator pulling the brake via a
	// governance magic word (e.g. "脚手架"). Source of truth for the word list
	// lives in the L0 house-rules, not here (KD-5/F114).
	EventTypeMagicWord EventType = "magic_word"
	// EventTypeMarked is a cat-declared turning point (MarkEvent). SG indexes it
	// verbatim; it never infers which message is an "aha" (no-classifier red line).
	EventTypeMarked EventType = "marked"
	// EventTypeResolution links an event to a harness change (commit/hook/skill/rule)
	// so the flywheel has a black box + dashboard (Phase C).
	EventTypeResolution EventType = "resolution"
)

// EventConfidence encodes the detection confidence tier for an event. Low
// confidence defaults to collapsed in the timeline (F227 Phase A detection
// strategy: magic word + @cat = high; discussing house-rules = low/non-mark).
type EventConfidence string

const (
	ConfHigh   EventConfidence = "high"
	ConfMedium EventConfidence = "medium"
	ConfLow    EventConfidence = "low"
)

// MemoryEvent is a single first-class cognitive-state transition. The schema is
// shaped for the v5 end-state (KD-5): it carries the cognitive transition as a
// core field, not the magic word. ownerUserId is a permission/scope boundary,
// not an 11th cognitive field — events are isolated per owner with no
// unknown/default fallback (fail-closed against cross-owner leakage).
type MemoryEvent struct {
	ID                 string           `json:"id"`
	Type               EventType        `json:"type"`
	Trigger            string           `json:"trigger"` // magic word text / marked trigger
	Cat                string           `json:"cat"`     // acting breed id
	ThreadID           string           `json:"thread_id"`
	MessageID          string           `json:"message_id"`
	Timestamp          int64            `json:"timestamp"` // unix milli
	Summary            string           `json:"summary"`
	CognitiveTransition string          `json:"cognitive_transition,omitempty"`
	RelatedHarness     string           `json:"related_harness,omitempty"` // commit/hook/skill/rule ref
	Confidence         EventConfidence  `json:"confidence"`
	OwnerUserID        string           `json:"owner_user_id"`
}

// Teleport is a precise jump coordinate resolved from an event: jump to the
// exact (thread, message) the event references (F227 R1/AC-A4 — "say 'I said
// scaffold' then instantly jump to that thread's message").
type Teleport struct {
	ThreadID  string `json:"thread_id"`
	MessageID string `json:"message_id"`
	EventID   string `json:"event_id"`
}

// EventStore persists memory events in a dedicated SQLite file (path +
// ".events.db") so the schema evolves independently of the lane store. It is
// owner-scoped: every read/write is gated by OwnerUserID with no fallback.
type EventStore struct {
	db *sql.DB
}

// NewEventStore opens (or creates) the event store at path. A nil/empty path or
// an open failure yields a nil store — callers must treat a nil store as
// "Event Memory unavailable" and fail closed (no events synthesized).
func NewEventStore(path string) *EventStore {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil
	}
	db, err := sql.Open("sqlite", path+".events.db")
	if err != nil {
		return nil
	}
	db.SetMaxOpenConns(3)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS memory_event (
		id TEXT PRIMARY KEY,
		type TEXT,
		trigger TEXT,
		cat TEXT,
		thread_id TEXT,
		message_id TEXT,
		timestamp INTEGER,
		summary TEXT,
		cognitive_transition TEXT DEFAULT '',
		related_harness TEXT DEFAULT '',
		confidence TEXT,
		owner_user_id TEXT)`); err != nil {
		db.Close()
		return nil
	}
	// Idempotent uniqueness constraint: one event per (owner, thread, message, type)
	// so a re-scanned magic word does not duplicate (F227 AC-A1 ownership map).
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_event_owner
		ON memory_event (owner_user_id, thread_id, message_id, type)`); err != nil {
		db.Close()
		return nil
	}
	return &EventStore{db: db}
}

// Record persists a cat-declared or operator-pulled event. Missing id/timestamp
// are filled. owner_user_id is REQUIRED and failing to supply it returns an
// error rather than storing under a default scope (fail-closed).
func (s *EventStore) Record(ev *MemoryEvent) error {
	if s.db == nil {
		return fmt.Errorf("event store unavailable")
	}
	if ev.OwnerUserID == "" {
		return fmt.Errorf("event store: owner_user_id required (fail-closed)")
	}
	if ev.ID == "" {
		ev.ID = uuid.NewString()
	}
	if ev.Timestamp == 0 {
		ev.Timestamp = time.Now().UnixMilli()
	}
	if ev.Confidence == "" {
		ev.Confidence = ConfMedium
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO memory_event
		 (id, type, trigger, cat, thread_id, message_id, timestamp, summary,
		  cognitive_transition, related_harness, confidence, owner_user_id)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		ev.ID, string(ev.Type), ev.Trigger, ev.Cat, ev.ThreadID, ev.MessageID,
		ev.Timestamp, ev.Summary, ev.CognitiveTransition, ev.RelatedHarness,
		string(ev.Confidence), ev.OwnerUserID)
	return err
}

// LinkResolution attaches a harness-change reference (commit/hook/skill/rule) to
// an existing event, forming a resolution chain (F227 Phase C — flywheel
// black box). Returns error if the event is unknown for this owner.
func (s *EventStore) LinkResolution(eventID, ownerUserID, relatedHarness string) error {
	if s.db == nil {
		return fmt.Errorf("event store unavailable")
	}
	res, err := s.db.Exec(
		`UPDATE memory_event SET related_harness = ? WHERE id = ? AND owner_user_id = ?`,
		relatedHarness, eventID, ownerUserID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("event not found for owner: %s", eventID)
	}
	return nil
}

// Timeline returns events for an owner, newest-first, optionally filtered by
// type and minimum confidence. owner is required; an empty owner returns nil
// (fail-closed — never cross-owner leakage).
func (s *EventStore) Timeline(ownerUserID string, typ EventType, minConf EventConfidence) []*MemoryEvent {
	if s.db == nil || ownerUserID == "" {
		return nil
	}
	minRank := confRank(minConf)
	rows, err := s.db.Query(
		`SELECT id, type, trigger, cat, thread_id, message_id, timestamp, summary,
		        cognitive_transition, related_harness, confidence, owner_user_id
		 FROM memory_event WHERE owner_user_id = ? ORDER BY timestamp DESC`, ownerUserID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []*MemoryEvent
	for rows.Next() {
		ev := &MemoryEvent{}
		var t, c, conf string
		if err := rows.Scan(&ev.ID, &t, &ev.Trigger, &ev.Cat, &ev.ThreadID, &ev.MessageID,
			&ev.Timestamp, &ev.Summary, &ev.CognitiveTransition, &ev.RelatedHarness,
			&conf, &ev.OwnerUserID); err != nil {
			return out
		}
		ev.Type = EventType(t)
		ev.Cat = c
		ev.Confidence = EventConfidence(conf)
		if typ != "" && ev.Type != typ {
			continue
		}
		if confRank(ev.Confidence) < minRank {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// ResolveTeleport returns the precise (thread, message) coordinate for an event
// so the UI can jump to the exact message (F227 R1/AC-A4). Fails closed: unknown
// event or owner mismatch → nil.
func (s *EventStore) ResolveTeleport(eventID, ownerUserID string) *Teleport {
	if s.db == nil || ownerUserID == "" {
		return nil
	}
	var threadID, messageID string
	err := s.db.QueryRow(
		`SELECT thread_id, message_id FROM memory_event WHERE id = ? AND owner_user_id = ?`,
		eventID, ownerUserID).Scan(&threadID, &messageID)
	if err != nil {
		return nil
	}
	return &Teleport{ThreadID: threadID, MessageID: messageID, EventID: eventID}
}

// Count returns the number of recorded events for an owner (trend signal; must
// be paired with a resolution chain — frequency decline alone is NOT evidence
// of self-evolution per Maine Coon push-back, F227 KD-7).
func (s *EventStore) Count(ownerUserID string) int {
	if s.db == nil || ownerUserID == "" {
		return 0
	}
	var n int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM memory_event WHERE owner_user_id = ?", ownerUserID).Scan(&n)
	return n
}

func (s *EventStore) close() { _ = s.db.Close() }

func confRank(c EventConfidence) int {
	switch c {
	case ConfHigh:
		return 3
	case ConfMedium:
		return 2
	case ConfLow:
		return 1
	default:
		// Empty confidence means "no minimum" when used as a filter floor
		// (Timeline with minConf="" must return all tiers, not just medium+).
		return 0
	}
}

// sortEventsChrono is a small helper used by tests/aggregation to order events
// ascending by timestamp (used for cross-thread dedup grouping).
func sortEventsChrono(evs []*MemoryEvent) {
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].Timestamp < evs[j].Timestamp })
}
