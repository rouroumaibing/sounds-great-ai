package transport

import (
	"sync"
	"time"
)

const (
	rateWindow          = 1 * time.Second
	rateThreshold       = 200 // max events per window before warning
	rateDequeCapacity   = 256 // power-of-2 for fast modulo
	rateDequeMask       = rateDequeCapacity - 1
)

// RateMonitor tracks per-session broadcast rates using a sliding window
// with O(1) head-index deque eviction. When a session exceeds the threshold
// (200 events / 1s), it emits a warning via onWarn without blocking.
type RateMonitor struct {
	mu       sync.Mutex
	sessions map[string]*sessionDeque
	onWarn   func(sessionID string, count int)
}

type sessionDeque struct {
	buf     [rateDequeCapacity]time.Time
	head    int // index of oldest entry
	tail    int // index where next write goes
	count   int // number of valid entries
	warned  bool // already warned for current burst
}

func NewRateMonitor(onWarn func(sessionID string, count int)) *RateMonitor {
	return &RateMonitor{
		sessions: make(map[string]*sessionDeque),
		onWarn:   onWarn,
	}
}

// Record records an event for the given session. If the rate exceeds
// the threshold, onWarn is called (best-effort, recovered from panics).
func (m *RateMonitor) Record(sessionID string) {
	now := time.Now()

	m.mu.Lock()
	sdq, ok := m.sessions[sessionID]
	if !ok {
		sdq = &sessionDeque{}
		m.sessions[sessionID] = sdq
	}

	// Evict entries outside the sliding window
	cutoff := now.Add(-rateWindow)
	for sdq.count > 0 {
		if sdq.buf[sdq.head].After(cutoff) {
			break
		}
		sdq.head = (sdq.head + 1) & rateDequeMask
		sdq.count--
	}

	// Add current event
	sdq.buf[sdq.tail] = now
	sdq.tail = (sdq.tail + 1) & rateDequeMask
	if sdq.count < rateDequeCapacity {
		sdq.count++
	} else {
		sdq.head = (sdq.head + 1) & rateDequeMask
	}

	exceeded := sdq.count >= rateThreshold
	if exceeded && !sdq.warned {
		sdq.warned = true
	} else if !exceeded {
		sdq.warned = false
	}

	count := sdq.count
	m.mu.Unlock()

	if exceeded && m.onWarn != nil {
		// Best-effort: recover from panics in onWarn
		func() {
			defer func() { _ = recover() }()
			m.onWarn(sessionID, count)
		}()
	}
}

// RemoveSession cleans up the deque for a disconnected session.
func (m *RateMonitor) RemoveSession(sessionID string) {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}

// SessionCount returns the number of tracked sessions (for diagnostics).
func (m *RateMonitor) SessionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}
