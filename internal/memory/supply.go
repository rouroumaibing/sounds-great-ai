package memory

import (
	"strings"
	"time"
)

// SessionMessage is a minimal message representation for delta detection.
// The platform does NOT interpret semantics — it uses typed signals.
type SessionMessage struct {
	Role    string // "user" | "assistant" | "system"
	Content string
	Time    int64
}

// SessionDelta is the typed signal extracted from a session close.
// These are deterministic pattern matches, NOT LLM judgments (VISION §3).
type SessionDelta struct {
	SessionID  string
	Decisions  []string // "decided to X" patterns
	Corrections []string // "actually", "correction", "no, it should be" patterns
	IdentityChanges []string // "I am", "my role is", "I prefer" patterns
	Preferences []string // "I like", "I prefer", "use X style" patterns
	ClosedAt   int64
}

// DeltaCandidate is a LaneEntry candidate produced by the delta producer.
type DeltaCandidate struct {
	Lane    LaneType
	Content string
	Source  string
}

// DeltaProducer detects deltas at session close and produces LaneEntry candidates.
// It uses typed pattern matching, NOT LLM inference (VISION §3 compliance).
type DeltaProducer struct{}

// NewDeltaProducer creates a new DeltaProducer.
func NewDeltaProducer() *DeltaProducer {
	return &DeltaProducer{}
}

// Detect scans session messages for typed delta signals.
// This is pattern-based, not semantic — no LLM is called.
func (dp *DeltaProducer) Detect(sessionID string, messages []SessionMessage) *SessionDelta {
	delta := &SessionDelta{
		SessionID: sessionID,
		ClosedAt:  time.Now().UnixMilli(),
	}
	for _, msg := range messages {
		content := msg.Content
		delta.Decisions = append(delta.Decisions, extractDecisions(content)...)
		delta.Corrections = append(delta.Corrections, extractCorrections(content)...)
		delta.IdentityChanges = append(delta.IdentityChanges, extractIdentityChanges(content)...)
		delta.Preferences = append(delta.Preferences, extractPreferences(content)...)
	}
	return delta
}

// Produce converts a SessionDelta into LaneEntry candidates for approval.
// Candidates are pending — human disposition is required before they become truth.
func (dp *DeltaProducer) Produce(delta *SessionDelta) []DeltaCandidate {
	var candidates []DeltaCandidate
	source := "session:" + delta.SessionID

	for _, d := range delta.Decisions {
		candidates = append(candidates, DeltaCandidate{
			Lane:    LaneDecision,
			Content: d,
			Source:  source,
		})
	}
	for _, c := range delta.Corrections {
		candidates = append(candidates, DeltaCandidate{
			Lane:    LaneLesson,
			Content: c,
			Source:  source,
		})
	}
	for _, id := range delta.IdentityChanges {
		candidates = append(candidates, DeltaCandidate{
			Lane:    LaneProfile,
			Content: id,
			Source:  source,
		})
	}
	for _, p := range delta.Preferences {
		candidates = append(candidates, DeltaCandidate{
			Lane:    LaneTaste,
			Content: p,
			Source:  source,
		})
	}
	return candidates
}

// SubmitCandidates submits all candidates to the LaneRegistry as pending entries.
// Returns the IDs of submitted entries.
func (dp *DeltaProducer) SubmitCandidates(reg *LaneRegistry, candidates []DeltaCandidate) []string {
	var ids []string
	for _, c := range candidates {
		lane := reg.Lane(c.Lane)
		if lane == nil {
			continue
		}
		entry := lane.Submit(c.Content, c.Source)
		ids = append(ids, entry.ID)
	}
	return ids
}

// DetectAndSubmit is a convenience method: detect + produce + submit.
func (dp *DeltaProducer) DetectAndSubmit(reg *LaneRegistry, sessionID string, messages []SessionMessage) []string {
	delta := dp.Detect(sessionID, messages)
	candidates := dp.Produce(delta)
	return dp.SubmitCandidates(reg, candidates)
}

// --- Typed pattern extractors (deterministic, no LLM) ---

func extractDecisions(content string) []string {
	var result []string
	lower := strings.ToLower(content)
	markers := []string{"decided to ", "decision: ", "we decided ", "chose to "}
	for _, m := range markers {
		idx := strings.Index(lower, m)
		for idx >= 0 {
			rest := content[idx+len(m):]
			line := firstSentence(rest)
			if line != "" {
				result = append(result, m+line)
			}
			next := strings.Index(lower[idx+len(m):], m)
			if next < 0 {
				break
			}
			idx = idx + len(m) + next
		}
	}
	return result
}

func extractCorrections(content string) []string {
	var result []string
	lower := strings.ToLower(content)
	markers := []string{"actually, ", "correction: ", "no, it should be ", "wait, ", "fix: "}
	for _, m := range markers {
		idx := strings.Index(lower, m)
		for idx >= 0 {
			rest := content[idx+len(m):]
			line := firstSentence(rest)
			if line != "" {
				result = append(result, m+line)
			}
			next := strings.Index(lower[idx+len(m):], m)
			if next < 0 {
				break
			}
			idx = idx + len(m) + next
		}
	}
	return result
}

func extractIdentityChanges(content string) []string {
	var result []string
	lower := strings.ToLower(content)
	markers := []string{"i am ", "my role is ", "i work as "}
	for _, m := range markers {
		idx := strings.Index(lower, m)
		for idx >= 0 {
			rest := content[idx+len(m):]
			line := firstSentence(rest)
			if line != "" {
				result = append(result, m+line)
			}
			next := strings.Index(lower[idx+len(m):], m)
			if next < 0 {
				break
			}
			idx = idx + len(m) + next
		}
	}
	return result
}

func extractPreferences(content string) []string {
	var result []string
	lower := strings.ToLower(content)
	markers := []string{"i prefer ", "i like ", "use ", "i'd rather "}
	for _, m := range markers {
		idx := strings.Index(lower, m)
		for idx >= 0 {
			rest := content[idx+len(m):]
			line := firstSentence(rest)
			if line != "" {
				result = append(result, m+line)
			}
			next := strings.Index(lower[idx+len(m):], m)
			if next < 0 {
				break
			}
			idx = idx + len(m) + next
		}
	}
	return result
}

// firstSentence extracts up to the first sentence boundary (., !, ?, or newline).
func firstSentence(s string) string {
	if s == "" {
		return ""
	}
	end := len(s)
	for i, ch := range s {
		if ch == '.' || ch == '!' || ch == '?' || ch == '\n' {
			end = i
			break
		}
	}
	result := strings.TrimSpace(s[:end])
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}


