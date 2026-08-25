package github

import (
	"sync"
	"time"
)

// PRState is a tracked pull request.
type PRState struct {
	Repo      string
	Number    int
	State     string // open | closed | merged
	Title     string
	UpdatedAt time.Time
}

// PRTracker maintains PR state keyed by repo#number. It is the consumer side
// the task board (P0-5) reads to show PR progress. Goroutine-safe.
type PRTracker struct {
	mu sync.Mutex
	prs map[string]*PRState
}

// NewPRTracker creates an empty tracker.
func NewPRTracker() *PRTracker {
	return &PRTracker{prs: make(map[string]*PRState)}
}

func key(repo string, num int) string {
	return repo + "#" + itoa(num)
}

// Apply upserts PR state from a webhook event. Non-PR events are ignored.
func (t *PRTracker) Apply(ev *WebhookEvent) {
	if ev == nil || ev.Type != "pull_request" || ev.PRNumber == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(ev.Repo, ev.PRNumber)
	t.prs[k] = &PRState{
		Repo:      ev.Repo,
		Number:    ev.PRNumber,
		State:     ev.PRState,
		Title:     ev.Title,
		UpdatedAt: time.Now(),
	}
}

// Get returns a PR by repo+number.
func (t *PRTracker) Get(repo string, num int) (*PRState, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.prs[key(repo, num)]
	return p, ok
}

// OpenForRepo returns PRs for a repo whose state is "open".
func (t *PRTracker) OpenForRepo(repo string) []PRState {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []PRState
	for _, p := range t.prs {
		if p.Repo == repo && p.State == "open" {
			out = append(out, *p)
		}
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
