package transport

import (
	"sync/atomic"
	"testing"
)

func TestRateMonitor_RecordAndSessionCount(t *testing.T) {
	m := NewRateMonitor(nil)

	m.Record("session-1")
	m.Record("session-1")
	m.Record("session-2")

	if got := m.SessionCount(); got != 2 {
		t.Fatalf("SessionCount = %d, want 2", got)
	}
}

func TestRateMonitor_RemoveSession(t *testing.T) {
	m := NewRateMonitor(nil)

	m.Record("session-1")
	m.Record("session-2")
	if got := m.SessionCount(); got != 2 {
		t.Fatalf("SessionCount = %d, want 2", got)
	}

	m.RemoveSession("session-1")
	if got := m.SessionCount(); got != 1 {
		t.Fatalf("SessionCount after remove = %d, want 1", got)
	}

	m.RemoveSession("session-2")
	if got := m.SessionCount(); got != 0 {
		t.Fatalf("SessionCount after remove all = %d, want 0", got)
	}
}

func TestRateMonitor_WarningCallback(t *testing.T) {
	var warned atomic.Int32
	m := NewRateMonitor(func(sessionID string, count int) {
		warned.Add(1)
	})

	// Threshold is 200 events per 1s window. Record enough to exceed it.
	for i := 0; i < rateThreshold; i++ {
		m.Record("burst-session")
	}

	if got := warned.Load(); got < 1 {
		t.Fatalf("expected warning callback to fire, got %d calls", got)
	}
}
