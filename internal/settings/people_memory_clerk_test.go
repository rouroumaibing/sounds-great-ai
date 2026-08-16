package settings

import (
	"testing"
	"time"
)

// TestNextDailyRun locks the daily "30 4 * * *" schedule:
// the clerk must sleep until the next 04:30 local, never fire immediately, and
// never exceed a 24h window.
func TestNextDailyRun(t *testing.T) {
	loc := time.Local
	const tol = 2 * time.Second

	cases := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{"early morning before tick", time.Date(2026, 8, 16, 2, 0, 0, 0, loc), 2*time.Hour + 30*time.Minute},
		{"just before tick", time.Date(2026, 8, 16, 4, 29, 0, 0, loc), 1 * time.Minute},
		{"after tick same day", time.Date(2026, 8, 16, 13, 0, 0, 0, loc), 15*time.Hour + 30*time.Minute},
		{"exactly at tick rolls to next day", time.Date(2026, 8, 16, 4, 30, 0, 0, loc), 24*time.Hour},
		{"late night", time.Date(2026, 8, 16, 23, 59, 0, 0, loc), 4*time.Hour + 31*time.Minute},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := nextDailyRun(c.now, 4, 30)
			if got <= 0 {
				t.Fatalf("expected positive duration, got %v", got)
			}
			if got > 24*time.Hour {
				t.Fatalf("expected duration <= 24h, got %v", got)
			}
			// exact-match cases (4:30) need a tolerance against the 24h roll.
			diff := got - c.want
			if diff < 0 {
				diff = -diff
			}
			if diff > tol {
				t.Fatalf("nextDailyRun(%v) = %v, want ~%v", c.now, got, c.want)
			}
		})
	}
}
