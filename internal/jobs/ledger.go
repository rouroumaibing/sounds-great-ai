package jobs

import "sync"

// Bucket is one of the three attribution buckets a job's consumption is split
// into (F275): the input it consumed, the compute it spent, and the output it
// produced.
type Bucket string

const (
	// BucketInput attributes ingested context / tokens.
	BucketInput Bucket = "input"
	// BucketCompute attributes execution cost (e.g. model calls).
	BucketCompute Bucket = "compute"
	// BucketOutput attributes produced artifacts / tokens.
	BucketOutput Bucket = "output"
)

// AttributionLedger records per-bucket consumption attributed to a WorkID. It
// is goroutine-safe.
type AttributionLedger struct {
	mu      sync.Mutex
	WorkID  string
	entries map[Bucket]int64
}

// NewAttributionLedger creates a ledger for a work identity.
func NewAttributionLedger(workID string) *AttributionLedger {
	return &AttributionLedger{WorkID: workID, entries: make(map[Bucket]int64)}
}

// Add accrues amount to a bucket. amount may be negative (refund).
func (l *AttributionLedger) Add(b Bucket, amount int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[b] += amount
}

// Get returns the current balance of a bucket.
func (l *AttributionLedger) Get(b Bucket) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.entries[b]
}

// Total returns the sum across all three buckets.
func (l *AttributionLedger) Total() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var sum int64
	for _, v := range l.entries {
		sum += v
	}
	return sum
}

// Buckets returns a snapshot copy of all bucket balances.
func (l *AttributionLedger) Buckets() map[Bucket]int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[Bucket]int64, len(l.entries))
	for k, v := range l.entries {
		out[k] = v
	}
	return out
}

func (l *AttributionLedger) clone() *AttributionLedger {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := &AttributionLedger{WorkID: l.WorkID, entries: make(map[Bucket]int64, len(l.entries))}
	for k, v := range l.entries {
		cp.entries[k] = v
	}
	return cp
}
