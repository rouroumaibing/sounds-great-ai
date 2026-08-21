package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// Recall consumption outcome (P0-1), 'used' | 'ignored'. SG's deterministic
// pipeline has no tool-consumption
// signal, so outcome is set by the operator (or a future verification hook)
// marking whether surfaced memory was actually useful. "" = unverified.
const (
	RecallOutcomeEmpty   = "" // not yet verified
	RecallOutcomeUsed    = "used"
	RecallOutcomeIgnored = "ignored"
)

// Three-axis semantic classification of a recall (有害消费 / 错失需求 /
// 注意力成本). SG maps the
// operator outcome onto these axes so the ledger reports recall *quality*, not
// just volume.
const (
	AxisBeneficial = "beneficial" // memory was used → useful, low attention cost
	AxisUnmet      = "unmet"      // memory unverified → potential unmet demand (estimated)
	AxisAttention  = "attention"  // memory surfaced but ignored → attention cost
)

// Maturity labels how trustworthy a three-axis measurement is
// (实测/估算/下界/无数据).
const (
	MaturityMeasured  = "measured"
	MaturityEstimated = "estimated"
	MaturityLower     = "lower_bound"
	MaturityNone      = "none"
)

// RecallEvent is a single memory-recall observation. It records that approved shared-memory truth was
// surfaced into a dog's prompt (a "push" recall) or pulled on demand (a "pull"
// recall) so the operator can see what memory was surfaced, how often, and —
// via Outcome / Axis / Maturity — the *quality* of that recall. Content-free
// metadata only: it stores entry IDs, not the recalled text.
type RecallEvent struct {
	ID        string `json:"id"`
	OperatorID string `json:"operator_id"`
	Timestamp int64  `json:"timestamp"` // unix milli
	Kind      string `json:"kind"`      // "push" (system injection) | "pull" (on-demand drill)
	Trigger   string `json:"trigger"`   // "session_bootstrap" | "cold_context" | "seal" | "manual"
	EntryIDs  []string `json:"entry_ids"` // truth entries that were recalled
	Count     int    `json:"count"`     // len(EntryIDs)
	Outcome   string `json:"outcome"`   // RecallOutcome*
	Axis      string `json:"axis"`      // Axis* (three-axis semantic)
	Maturity  string `json:"maturity"`  // Maturity*
}

// RecallWindowStat is the per-day-window consumption view returned by Ledger.
// It reports both the raw counts and the three-axis semantic breakdown with
// maturity labels.
type RecallWindowStat struct {
	Total      int            `json:"total"`
	Used       int            `json:"used"`
	Ignored    int            `json:"ignored"`
	Unverified int            `json:"unverified"`
	Rate       float64        `json:"rate"` // unverified / total, 0..1
	Beneficial int            `json:"beneficial"`
	Unmet      int            `json:"unmet"`
	Attention  int            `json:"attention"`
	Maturity   map[string]int `json:"maturity"`
}

// RecallStore persists recall events + lifecycle traces in the shared SQLite
// store (recall_entry / lifecycle_trace tables), and answers recent-event,
// ledger, and lifecycle queries. Backend for the frontend RecallFeed /
// RecallLedger / lifecycle trace.
type RecallStore struct {
	db *sql.DB
}

// NewRecallStore opens the shared memory DB at path (same file as the lane
// persister) and ensures the recall_entry + lifecycle_trace tables exist.
func NewRecallStore(path string) *RecallStore {
	db, err := openMemoryDB(path)
	if err != nil {
		log.Printf("memory: recall store unavailable: %v", err)
		return &RecallStore{}
	}
	return &RecallStore{db: db}
}

// Record appends a recall event and persists it. Missing ID/timestamp are
// filled in. axis/maturity default from outcome when empty.
func (s *RecallStore) Record(ev *RecallEvent) {
	if s.db == nil {
		return
	}
	if ev.ID == "" {
		ev.ID = uuid.NewString()
	}
	if ev.Timestamp == 0 {
		ev.Timestamp = time.Now().UnixMilli()
	}
	if ev.Count == 0 && len(ev.EntryIDs) > 0 {
		ev.Count = len(ev.EntryIDs)
	}
	if ev.Axis == "" {
		ev.Axis = outcomeAxis(ev.Outcome)
	}
	if ev.Maturity == "" {
		ev.Maturity = outcomeMaturity(ev.Outcome)
	}
	ids, _ := json.Marshal(ev.EntryIDs)
	_, err := s.db.Exec(
		`INSERT INTO recall_entry (id, operator_id, timestamp, kind, trigger, entry_ids, count, outcome, axis, maturity)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		ev.ID, ev.OperatorID, ev.Timestamp, ev.Kind, ev.Trigger, string(ids), ev.Count, ev.Outcome, ev.Axis, ev.Maturity,
	)
	if err != nil {
		log.Printf("memory: failed to record recall event: %v", err)
	}
}

func outcomeAxis(outcome string) string {
	switch outcome {
	case RecallOutcomeUsed:
		return AxisBeneficial
	case RecallOutcomeIgnored:
		return AxisAttention
	default:
		return AxisUnmet
	}
}

func outcomeMaturity(outcome string) string {
	switch outcome {
	case RecallOutcomeUsed, RecallOutcomeIgnored:
		return MaturityMeasured
	default:
		return MaturityEstimated
	}
}

// MarkOutcome sets the consumption outcome (used/ignored) of a recall event,
// completing the consumption-verification loop. axis/maturity override the
// defaults when provided.
// operator, when non-empty, re-affirms the acting operator who confirmed the
// outcome (multi-operator attribution); when empty the creation-time operator
// is preserved so a non-scoped mark never clobbers record ownership.
func (s *RecallStore) MarkOutcome(id, outcome, axis, maturity, operator string) error {
	if s.db == nil {
		return fmt.Errorf("recall store unavailable")
	}
	if outcome != RecallOutcomeUsed && outcome != RecallOutcomeIgnored {
		return fmt.Errorf("invalid outcome: %q", outcome)
	}
	if axis == "" {
		axis = outcomeAxis(outcome)
	}
	if maturity == "" {
		maturity = outcomeMaturity(outcome)
	}
	q := "UPDATE recall_entry SET outcome = ?, axis = ?, maturity = ?"
	args := []any{outcome, axis, maturity}
	if operator != "" {
		q += ", operator_id = ?"
		args = append(args, operator)
	}
	q += " WHERE id = ?"
	args = append(args, id)
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("recall event not found: %s", id)
	}
	return nil
}

// Recent returns the most recent up-to-limit recall events (oldest-first).
func (s *RecallStore) Recent(limit int) []*RecallEvent {
	if s.db == nil {
		return nil
	}
	rows, err := s.db.Query(
		`SELECT id, operator_id, timestamp, kind, trigger, entry_ids, count, outcome, axis, maturity
		 FROM recall_entry ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var all []*RecallEvent
	for rows.Next() {
		ev := &RecallEvent{}
		var ids string
		if err := rows.Scan(&ev.ID, &ev.OperatorID, &ev.Timestamp, &ev.Kind, &ev.Trigger, &ids, &ev.Count, &ev.Outcome, &ev.Axis, &ev.Maturity); err != nil {
			return all
		}
		_ = json.Unmarshal([]byte(ids), &ev.EntryIDs)
		all = append(all, ev)
	}
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all
}

// Ledger returns, for each requested day-window, the recall counts and the
// three-axis semantic breakdown (beneficial / unmet / attention) with maturity
// labels.
func (s *RecallStore) Ledger(windows []int) map[string]RecallWindowStat {
	res := make(map[string]RecallWindowStat)
	for _, w := range windows {
		res[fmt.Sprintf("%dd", w)] = RecallWindowStat{Maturity: map[string]int{}}
	}
	if s.db == nil {
		return res
	}
	rows, err := s.db.Query("SELECT timestamp, outcome, axis, maturity FROM recall_entry")
	if err != nil {
		return res
	}
	defer rows.Close()
	now := time.Now().UnixMilli()
	day := int64(24 * 60 * 60 * 1000)
	for rows.Next() {
		var ts int64
		var outcome, axis, maturity string
		if err := rows.Scan(&ts, &outcome, &axis, &maturity); err != nil {
			return res
		}
		age := now - ts
		for _, w := range windows {
			if age > int64(w)*day {
				continue
			}
			st := res[fmt.Sprintf("%dd", w)]
			st.Total++
			switch outcome {
			case RecallOutcomeUsed:
				st.Used++
			case RecallOutcomeIgnored:
				st.Ignored++
			default:
				st.Unverified++
			}
			if axis == "" {
				axis = outcomeAxis(outcome)
			}
			if maturity == "" {
				maturity = outcomeMaturity(outcome)
			}
			switch axis {
			case AxisBeneficial:
				st.Beneficial++
			case AxisUnmet:
				st.Unmet++
			case AxisAttention:
				st.Attention++
			}
			if st.Maturity == nil {
				st.Maturity = map[string]int{}
			}
			st.Maturity[maturity]++
			res[fmt.Sprintf("%dd", w)] = st
		}
	}
	for k, st := range res {
		if st.Total > 0 {
			st.Rate = float64(st.Unverified) / float64(st.Total)
		}
		res[k] = st
	}
	return res
}

// RecordLifecycle appends an append-only lifecycle-trace record. Used to track creation / consumption / correction
// of a memory entry across its whole lifecycle.
func (s *RecallStore) RecordLifecycle(axis LifecycleAxis, entryID, lane, detail, maturity string) {
	if s.db == nil {
		return
	}
	if maturity == "" {
		maturity = MaturityMeasured
	}
	_, err := s.db.Exec(
		`INSERT INTO lifecycle_trace (axis, entry_id, lane, detail, maturity, timestamp)
		 VALUES (?,?,?,?,?,?)`,
		string(axis), entryID, lane, detail, maturity, time.Now().UnixMilli())
	if err != nil {
		log.Printf("memory: failed to record lifecycle trace: %v", err)
	}
}

// RecentLifecycle returns the most recent lifecycle-trace records (newest first).
func (s *RecallStore) RecentLifecycle(limit int) []LifecycleRecord {
	if s.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT axis, entry_id, lane, detail, maturity, timestamp
		 FROM lifecycle_trace ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []LifecycleRecord
	for rows.Next() {
		var r LifecycleRecord
		var axis, lane, detail, maturity string
		var ts int64
		if err := rows.Scan(&axis, &r.EntryID, &lane, &detail, &maturity, &ts); err != nil {
			return out
		}
		r.Axis = LifecycleAxis(axis)
		r.Lane = LaneType(lane)
		r.Detail = detail
		r.Timestamp = ts
		out = append(out, r)
	}
	return out
}

// Count returns the total number of recorded recall events.
func (s *RecallStore) Count() int {
	if s.db == nil {
		return 0
	}
	var n int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM recall_entry").Scan(&n)
	return n
}
