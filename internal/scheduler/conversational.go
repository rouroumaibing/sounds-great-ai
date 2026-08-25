package scheduler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sounds-great-ai/internal/domains/missions"
)

// ConversationalDraft is the parsed result of a natural-language task request.
type ConversationalDraft struct {
	Title    string
	CronExpr string // empty => one-shot
	ThreadID string
	CreatedBy string
}

var (
	reDayAt   = regexp.MustCompile(`(?i)every\s+day\s+at\s+(\d{1,2})(?::(\d{2}))?\s*(am|pm)?`)
	reDailyAt = regexp.MustCompile(`(?i)daily\s+at\s+(\d{1,2})(?::(\d{2}))?\s*(am|pm)?`)
	reEveryN  = regexp.MustCompile(`(?i)every\s+(\d+)\s*(m|min|mins|minute|minutes|h|hr|hrs|hour|hours)`)
	reAtTime  = regexp.MustCompile(`(?i)\bat\s+(\d{1,2})(?::(\d{2}))?\s*(am|pm)?`)
)

// ParseConversational converts a free-text task request into a draft the
// caller can register via Scheduler.ScheduleCron. It is intentionally
// heuristic: it extracts a schedule when present and treats the remainder as
// the task title. A request with no recognizable schedule becomes a one-shot
// task (CronExpr == "").
func ParseConversational(text, threadID, createdBy string) ConversationalDraft {
	raw := strings.TrimSpace(text)
	lower := strings.ToLower(raw)
	draft := ConversationalDraft{ThreadID: threadID, CreatedBy: createdBy}

	cron, remainder := extractSchedule(lower, raw)
	draft.CronExpr = cron

	// The title is the remainder with schedule crud removed, or the whole text
	// if nothing was extracted.
	title := strings.TrimSpace(remainder)
	if title == "" {
		title = raw
	}
	// Strip leading filler verbs.
	title = regexp.MustCompile(`(?i)^(please\s+|kindly\s+)?(remind\s+me\s+to\s+|remember\s+to\s+|tell\s+me\s+to\s+)`).ReplaceAllString(title, "")
	draft.Title = title
	return draft
}

// extractSchedule returns (cronExpr, remainingText). remainingText keeps the
// human-readable remainder after removing the matched schedule phrase.
func extractSchedule(lower, raw string) (string, string) {
	remaining := raw

	if m := reEveryN.FindStringSubmatch(lower); m != nil {
		n, _ := strconv.Atoi(m[1])
		unit := m[2]
		cron := fmt.Sprintf("every %d%s", n, unitMap(unit))
		return cron, stripMatch(raw, reEveryN)
	}
	if m := reDayAt.FindStringSubmatch(lower); m != nil {
		h := atoiDefault(m[1], 9)
		mi := atoiDefault(m[2], 0)
		h = applyMeridiem(h, m[3])
		return fmt.Sprintf("daily@%02d:%02d", h, mi), stripMatch(raw, reDayAt)
	}
	if m := reDailyAt.FindStringSubmatch(lower); m != nil {
		h := atoiDefault(m[1], 9)
		mi := atoiDefault(m[2], 0)
		h = applyMeridiem(h, m[3])
		return fmt.Sprintf("daily@%02d:%02d", h, mi), stripMatch(raw, reDailyAt)
	}
	if m := reAtTime.FindStringSubmatch(lower); m != nil {
		h := atoiDefault(m[1], 9)
		mi := atoiDefault(m[2], 0)
		h = applyMeridiem(h, m[3])
		return fmt.Sprintf("daily@%02d:%02d", h, mi), stripMatch(raw, reAtTime)
	}
	return "", remaining
}

func unitMap(u string) string {
	switch strings.ToLower(u) {
	case "h", "hr", "hrs", "hour", "hours":
		return "h"
	default:
		return "m"
	}
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func applyMeridiem(h int, mer string) int {
	switch strings.ToLower(mer) {
	case "pm":
		if h < 12 {
			return h + 12
		}
	case "am":
		if h == 12 {
			return 0
		}
	}
	return h
}

// stripMatch removes the first match of re from s (case-insensitive) so the
// schedule phrase does not leak into the title.
func stripMatch(s string, re *regexp.Regexp) string {
	loc := re.FindStringIndex(strings.ToLower(s))
	if loc == nil {
		return s
	}
	return strings.TrimSpace(s[:loc[0]] + s[loc[1]:])
}

// RegisterConversational parses text and registers a scheduled task, returning
// the created spec. This is the "conversational registration writes to the task
// pool" path required by the roadmap gate.
func (s *Scheduler) RegisterConversational(text, threadID, createdBy string) (*missions.TaskSpec, error) {
	d := ParseConversational(text, threadID, createdBy)
	id := "conv-" + time.Now().Format("20060102-150405.000")
	t := missions.NewTaskSpec(id, d.Title, createdBy)
	t.ThreadID = threadID
	return t, s.ScheduleCron(t, d.CronExpr)
}
