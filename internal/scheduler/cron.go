// Package scheduler provides cron-style scheduling and conversational task
// registration for the Mission Hub (roadmap P0-5). It is intentionally
// transport-free: the caller wires fired tasks into the dispatch queue / agent.
package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Spec is a schedule expression. Supported forms:
//   - interval : "every 30m", "every 2h"
//   - daily    : "daily@09:30", "every day at 09:30"
//   - cron     : standard 5-field "0 9 * * *" (min hour dom month dow)
type Spec struct {
	kind    string // "interval" | "daily" | "cron"
	interval time.Duration
	dailyH   int
	dailyM   int
	fields  [5]fieldSet // minute hour dom month dow
}

type fieldSet struct {
	steps map[int]bool
	all   bool
}

// Parse builds a Spec from an expression. It normalizes whitespace.
func Parse(expr string) (*Spec, error) {
	s := strings.TrimSpace(expr)
	if s == "" {
		return nil, fmt.Errorf("scheduler: empty schedule")
	}
	low := strings.ToLower(s)

	if strings.HasPrefix(low, "every ") {
		rest := strings.TrimSpace(low[len("every "):])
		if strings.HasPrefix(rest, "day at ") || rest == "day" {
			return parseDaily(rest)
		}
		if d, ok := parseInterval(rest); ok {
			return &Spec{kind: "interval", interval: d}, nil
		}
	}
	if strings.HasPrefix(low, "daily@") {
		return parseDaily("day at " + strings.TrimPrefix(low, "daily@"))
	}
	// default: try 5-field cron
	return parseCron(s)
}

func parseInterval(rest string) (time.Duration, bool) {
	if strings.HasSuffix(rest, "m") {
		if n, err := strconv.Atoi(strings.TrimSuffix(rest, "m")); err == nil && n > 0 {
			return time.Duration(n) * time.Minute, true
		}
	}
	if strings.HasSuffix(rest, "h") {
		if n, err := strconv.Atoi(strings.TrimSuffix(rest, "h")); err == nil && n > 0 {
			return time.Duration(n) * time.Hour, true
		}
	}
	return 0, false
}

func parseDaily(rest string) (*Spec, error) {
	// rest looks like "day at 09:30" or "day"
	rest = strings.TrimSpace(rest)
	rest = strings.TrimPrefix(rest, "day at ")
	rest = strings.TrimPrefix(rest, "at ")
	if rest == "" || rest == "day" {
		return &Spec{kind: "daily", dailyH: 9, dailyM: 0}, nil
	}
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("scheduler: bad daily time %q", rest)
	}
	h, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	mi, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || h < 0 || h > 23 || mi < 0 || mi > 59 {
		return nil, fmt.Errorf("scheduler: bad daily time %q", rest)
	}
	return &Spec{kind: "daily", dailyH: h, dailyM: mi}, nil
}

func parseCron(s string) (*Spec, error) {
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return nil, fmt.Errorf("scheduler: cron needs 5 fields, got %d in %q", len(fields), s)
	}
	sp := &Spec{kind: "cron"}
	defs := []struct {
		spec string
		min  int
		max  int
		idx  int
	}{
		{fields[0], 0, 59, 0},
		{fields[1], 0, 23, 1},
		{fields[2], 1, 31, 2},
		{fields[3], 1, 12, 3},
		{fields[4], 0, 6, 4},
	}
	for _, d := range defs {
		fs, err := parseField(d.spec, d.min, d.max)
		if err != nil {
			return nil, err
		}
		sp.fields[d.idx] = fs
	}
	return sp, nil
}

func parseField(s string, min, max int) (fieldSet, error) {
	if s == "*" {
		return fieldSet{all: true}, nil
	}
	out := fieldSet{steps: make(map[int]bool)}
	// iterate comma-separated parts
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		step := 1
		if si := strings.Index(part, "/"); si >= 0 {
			n, err := strconv.Atoi(part[si+1:])
			if err != nil || n <= 0 {
				return out, fmt.Errorf("scheduler: bad step in %q", part)
			}
			step = n
			part = part[:si]
		}
		if part == "*" {
			for i := min; i <= max; i += step {
				out.steps[i] = true
			}
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(bounds[0])
			hi, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil || lo < min || hi > max || lo > hi {
				return out, fmt.Errorf("scheduler: bad range %q", part)
			}
			for i := lo; i <= hi; i += step {
				out.steps[i] = true
			}
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil || v < min || v > max {
			return out, fmt.Errorf("scheduler: bad value %q", part)
		}
		out.steps[v] = true
	}
	return out, nil
}

func (f fieldSet) matches(v int) bool {
	return f.all || f.steps[v]
}

// Next returns the next time strictly after `after` that the spec fires.
// For interval/daily it is exact; for cron it scans up to a 4-year horizon.
func (sp *Spec) Next(after time.Time) time.Time {
	switch sp.kind {
	case "interval":
		return after.Add(sp.interval)
	case "daily":
		loc := after.Location()
		next := time.Date(after.Year(), after.Month(), after.Day(), sp.dailyH, sp.dailyM, 0, 0, loc)
		if !next.After(after) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	case "cron":
		return sp.nextCron(after)
	default:
		return after
	}
}

func (sp *Spec) nextCron(after time.Time) time.Time {
	// Start one minute after `after` (cron fires at minute granularity).
	t := after.Add(time.Minute).Truncate(time.Minute)
	horizon := t.AddDate(4, 0, 0)
	for t.Before(horizon) {
		if sp.matchesCron(t) {
			return t
		}
		t = t.Add(time.Minute)
	}
	return after // no match within horizon (should not happen for valid specs)
}

func (sp *Spec) matchesCron(t time.Time) bool {
	min, hour, dom, mon, dow := t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday())
	if !sp.fields[0].matches(min) || !sp.fields[1].matches(hour) || !sp.fields[3].matches(mon) {
		return false
	}
	domStar := sp.fields[2].all
	dowStar := sp.fields[4].all
	if domStar && dowStar {
		return true
	}
	if !domStar && !dowStar {
		return sp.fields[2].matches(dom) && sp.fields[4].matches(dow)
	}
	// one restricted: match if the restricted one matches.
	if domStar {
		return sp.fields[4].matches(dow)
	}
	return sp.fields[2].matches(dom)
}
