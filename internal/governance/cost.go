// Package governance holds control-plane governance data structures that are
// not themselves policy engines but support them: per-dog cost/quota tracking
// for the cost dashboard (roadmap P1-D, README#8). Admission checks are
// fail-closed: a dog without a configured quota is denied spend (never silently
// admitted).
package governance

import (
	"errors"
	"sync"
	"time"
)

// DogID identifies a dog (agent) whose cost is tracked.
type DogID string

// Quota caps a dog's spend in a window (cost units, e.g. tokens*cost).
type Quota struct {
	DogID DogID
	Limit int64
	// Window labels the budgeting window (e.g. "daily").
	Window string
}

// DogCost is a dashboard row for one dog.
type DogCost struct {
	DogID   DogID
	Used    int64
	Limit   int64
	Ratio   float64
	Exceeds bool
}

// ErrNoQuota is returned by admission checks for a dog without a configured
// quota (fail-closed).
var ErrNoQuota = errors.New("governance: no quota configured (fail-closed)")

// CostLedger tracks per-dog quotas and spend. Goroutine-safe.
type CostLedger struct {
	mu     sync.Mutex
	quotas map[DogID]Quota
	spent  map[DogID]int64
}

// NewCostLedger creates an empty ledger.
func NewCostLedger() *CostLedger {
	return &CostLedger{
		quotas: make(map[DogID]Quota),
		spent:  make(map[DogID]int64),
	}
}

// SetQuota configures (or replaces) a dog's quota. A non-positive limit means
// "ungoverned" and causes fail-closed denial on admission.
func (l *CostLedger) SetQuota(q Quota) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.quotas[q.DogID] = q
}

// Record accrues spend for accounting. It does NOT gate admission; use
// AuthorizeSpend for that. Unknown dogs accrue without a quota (admission still
// denied via AuthorizeSpend).
func (l *CostLedger) Record(dog DogID, amount int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.spent[dog] += amount
}

// Used returns the accrued spend for a dog (0 if unseen).
func (l *CostLedger) Used(dog DogID) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.spent[dog]
}

// AuthorizeSpend is the fail-closed admission check: it returns whether a dog
// may spend `amount` now. A missing or non-positive quota denies (ErrNoQuota);
// exceeding the remaining limit denies (ErrQuotaExceeded).
func (l *CostLedger) AuthorizeSpend(dog DogID, amount int64) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	q, ok := l.quotas[dog]
	if !ok || q.Limit <= 0 {
		return false, ErrNoQuota
	}
	if l.spent[dog]+amount > q.Limit {
		return false, ErrQuotaExceeded
	}
	return true, nil
}

// ErrQuotaExceeded is returned when a spend would pass the limit.
var ErrQuotaExceeded = errors.New("governance: quota exceeded")

// Dashboard returns a sorted snapshot of per-dog cost rows.
func (l *CostLedger) Dashboard() []DogCost {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]DogCost, 0, len(l.quotas))
	for dog, q := range l.quotas {
		used := l.spent[dog]
		row := DogCost{DogID: dog, Used: used, Limit: q.Limit}
		if q.Limit > 0 {
			row.Ratio = float64(used) / float64(q.Limit)
		}
		row.Exceeds = q.Limit > 0 && used >= q.Limit
		out = append(out, row)
	}
	return out
}

// SnapshotAt is a no-op placeholder kept for future time-windowed dashboards;
// it returns the current dashboard. (Retained so callers can pass a timestamp
// without API churn.)
func (l *CostLedger) SnapshotAt(_ time.Time) []DogCost {
	return l.Dashboard()
}
