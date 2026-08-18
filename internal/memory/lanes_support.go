package memory

import (
	"fmt"
	"log"
	"strings"
)

// loadEntry inserts a pre-existing entry (used by persistence load).
func (l *Lane) loadEntry(e *LaneEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[e.ID] = e
	l.order = append(l.order, e.ID)
}

// HasContent reports whether the lane already holds a non-forgotten entry with
// the given content+source. Used by the supply path for idempotent submission
// (a re-sealed session must not produce duplicate candidates).
func (l *Lane) HasContent(content, source string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, id := range l.order {
		e := l.entries[id]
		if e.Status == StatusForgotten {
			continue
		}
		if e.Content == content && e.Source == source {
			return true
		}
	}
	return false
}

func allLaneTypes() []LaneType {
	return []LaneType{LaneTaste, LaneProfile, LaneEntity, LanePerson, LaneEvent, LaneDecision, LaneLesson}
}

// FindLaneOf returns the lane type that owns the given entry ID, or false when
// the entry is unknown. Used by the HTTP disposition endpoints which address
// entries by ID only (the lane type is derived here).
func (r *LaneRegistry) FindLaneOf(id string) (LaneType, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, l := range r.lanes {
		if _, ok := l.Get(id); ok {
			return l.Type(), true
		}
	}
	return "", false
}

// SetSensitivity re-classifies an entry's sensitivity level (Gap2, homologous
// clowder edge/marker source_sensitivity). Rejected when the level is unknown.
// The new level takes effect on the next recall filter via EntryVisible.
func (r *LaneRegistry) SetSensitivity(id, level string) bool {
	if !ValidSensitivity(level) {
		return false
	}
	laneType, ok := r.FindLaneOf(id)
	if !ok {
		return false
	}
	l := r.Lane(laneType)
	l.mu.Lock()
	e, ok := l.entries[id]
	if !ok {
		l.mu.Unlock()
		return false
	}
	e.Sensitivity = level
	l.mu.Unlock()
	l.onMutated()
	return true
}

// SensitivityOf returns an entry's current sensitivity level ("" if the entry
// is unknown). Used by the visibility-widening guardrail (Task #33).
func (r *LaneRegistry) SensitivityOf(id string) (string, bool) {
	laneType, ok := r.FindLaneOf(id)
	if !ok {
		return "", false
	}
	l := r.Lane(laneType)
	l.mu.RLock()
	defer l.mu.RUnlock()
	if e, ok := l.entries[id]; ok {
		return e.Sensitivity, true
	}
	return "", false
}

// operatorMatches reports whether an entry is visible to the given operator.
// "" operator sees everything; a named operator sees entries it owns plus
// shared (""-operator) entries — homologous clowder ownerUserId partitioning.
func operatorMatches(e *LaneEntry, operator string) bool {
	if operator == "" {
		return true
	}
	return e.OperatorID == operator || e.OperatorID == ""
}

// RecallEntries returns the approved (canonical) entries visible to operator,
// capped at maxLines. Used both by the prompt injector (which builds the
// markdown block) and by the recall-event recorder (which needs the entry IDs
// that were actually injected). Returns (nil, false) when there is no truth.
func (r *LaneRegistry) RecallEntries(maxLines int, operator string) ([]*LaneEntry, bool) {
	if maxLines <= 0 {
		maxLines = 20
	}
	var out []*LaneEntry
	for _, t := range r.LaneTypes() {
		for _, e := range r.Lane(t).Truth() {
			if EntryVisible(e, operator) {
				out = append(out, e)
			}
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	if len(out) > maxLines {
		out = out[:maxLines]
	}
	return out, true
}

// Search returns lane entries whose content matches query (FTS5 on SQLite,
// substring on JSON), visible to operator. Homologous clowder FTS5 full-text
// recall search. Returns nil when there is no match or no persister.
func (r *LaneRegistry) Search(query, operator string) []*LaneEntry {
	if r.persister == nil {
		return nil
	}
	matches, err := r.persister.search(query, operator)
	if err != nil {
		return nil
	}
	out := make([]*LaneEntry, 0, len(matches))
	for _, e := range matches {
		if EntryVisible(e, operator) {
			out = append(out, e)
		}
	}
	return out
}

// SharedMemoryTruth returns a token-bounded markdown block of all approved
// (canonical) entries visible to operator, for injection into a dog's system
// prompt (Persistent Identity, homologous clowder F296 context presentation).
// Pending candidates and retired/forgotten entries are excluded (M5 submission
// boundary: only human-approved truth is recalled). Returns ("", false, nil)
// when there is no approved truth. maxLines caps the block so the identity
// section stays bounded (M10 zero-distortion: keyword lines, no embedding).
func (r *LaneRegistry) SharedMemoryTruth(maxLines int, operator string) (string, bool, error) {
	entries, ok := r.RecallEntries(maxLines, operator)
	if !ok {
		return "", false, nil
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("- [%s] %s", e.Type, e.Content))
	}
	return strings.Join(lines, "\n"), true, nil
}

// flush persists the entire registry. No-op when non-persistent.
// Safe to call from a lane mutation hook: the lane that triggered the flush
// has already released its write lock before notify runs.
func (r *LaneRegistry) flush() {
	if r.persister == nil {
		return
	}
	r.mu.RLock()
	doc := &laneDocument{}
	for _, l := range r.lanes {
		for _, e := range l.All() {
			doc.Entries = append(doc.Entries, e)
		}
	}
	r.mu.RUnlock()
	if err := r.persister.save(doc); err != nil {
		log.Printf("memory: failed to persist lanes: %v", err)
	}
}

// Close releases the persistent store (if any).
func (r *LaneRegistry) Close() {
	if r.vector != nil {
		r.vector.close()
	}
	if r.cue != nil {
		r.cue.close()
	}
	if r.graph != nil {
		r.graph.close()
	}
	if r.persister != nil {
		r.persister.close()
	}
}
