package ops

import (
	"sync"
	"time"
)

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// LogBuffer is a fixed-capacity circular ring buffer for log entries.
// Add() is O(1) — no slice reallocation or shifting.
// Recent(n) is O(n) in the requested count, not the buffer capacity.
type LogBuffer struct {
	mu       sync.Mutex
	entries  []LogEntry // fixed-size ring
	head     int        // index of oldest entry
	tail     int        // index where next write goes
	count    int        // number of valid entries (0..capacity)
	capacity int
}

func NewLogBuffer(capacity int) *LogBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &LogBuffer{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
	}
}

// Add appends a log entry to the ring buffer in O(1) time.
func (lb *LogBuffer) Add(level, msg string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.entries[lb.tail] = LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
	}
	lb.tail = (lb.tail + 1) % lb.capacity
	if lb.count < lb.capacity {
		lb.count++
	} else {
		// Buffer full: advance head (overwrite oldest)
		lb.head = (lb.head + 1) % lb.capacity
	}
}

// Recent returns up to n most recent log entries in chronological order
// (oldest first, newest last). O(n) in the result size.
func (lb *LogBuffer) Recent(n int) []LogEntry {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if n <= 0 || n > lb.count {
		n = lb.count
	}
	if n == 0 {
		return []LogEntry{}
	}

	result := make([]LogEntry, n)
	// Start from (tail - n) mod capacity, which is the oldest of the n entries
	start := lb.tail - n
	if start < 0 {
		start += lb.capacity
	}
	for i := 0; i < n; i++ {
		result[i] = lb.entries[(start+i)%lb.capacity]
	}
	return result
}

// Len returns the number of entries currently in the buffer.
func (lb *LogBuffer) Len() int {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.count
}
