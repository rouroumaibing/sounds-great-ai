package orchestrator

import (
	"fmt"
	"strings"
)

// WorklistEntry represents a queued task
type WorklistEntry struct {
	TaskID       string
	ContextID    string
	FromAgent    string
	ToAgent      string
	VisitedChain []string // e.g. ["AgentA", "AgentB"]
	Content      string
	Status       string // "pending" | "processing" | "done" | "suspended"
}

// Worklist manages dynamic @mention routing with loop detection
type Worklist struct {
	entries   []WorklistEntry
	depth     int
	maxDepth  int
	streak    int
	maxStreak int
}

// NewWorklist creates a new Worklist
func NewWorklist() *Worklist {
	return &Worklist{
		maxDepth:  10,
		maxStreak: 4,
	}
}

// Add adds an entry with loop detection
func (w *Worklist) Add(entry WorklistEntry) error {
	w.depth++
	if w.depth > w.maxDepth {
		return fmt.Errorf("max depth %d exceeded", w.maxDepth)
	}

	// Loop detection via VisitedChain
	count := 0
	for _, a := range entry.VisitedChain {
		if a == entry.ToAgent {
			count++
		}
	}
	if count >= 2 {
		return fmt.Errorf("loop detected: %s visited %d times in chain %v", entry.ToAgent, count, entry.VisitedChain)
	}

	// Ping-pong detection
	if len(w.entries) > 0 {
		last := w.entries[len(w.entries)-1]
		if last.FromAgent == entry.ToAgent && last.ToAgent == entry.FromAgent {
			w.streak++
			if w.streak > w.maxStreak {
				return fmt.Errorf("ping-pong streak %d exceeds max %d", w.streak, w.maxStreak)
			}
		} else {
			w.streak = 0
		}
	}

	entry.Status = "pending"
	w.entries = append(w.entries, entry)
	return nil
}

// Next returns the next pending entry
func (w *Worklist) Next() *WorklistEntry {
	for i := range w.entries {
		if w.entries[i].Status == "pending" {
			w.entries[i].Status = "processing"
			return &w.entries[i]
		}
	}
	return nil
}

// MarkDone marks an entry as done
func (w *Worklist) MarkDone(taskID string) {
	for i := range w.entries {
		if w.entries[i].TaskID == taskID {
			w.entries[i].Status = "done"
		}
	}
}

// HasPending returns true if there are pending entries
func (w *Worklist) HasPending() bool {
	for _, e := range w.entries {
		if e.Status == "pending" {
			return true
		}
	}
	return false
}

// parseMentions extracts @AgentX mentions from text
func parseMentions(text string) []string {
	var mentions []string
	words := strings.Fields(text)
	for _, word := range words {
		if strings.HasPrefix(word, "@Agent") {
			mentions = append(mentions, strings.TrimPrefix(word, "@"))
		}
	}
	return mentions
}
