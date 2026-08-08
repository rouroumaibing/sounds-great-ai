package pool

import "testing"

func TestPooledProcess_Properties(t *testing.T) {
	p := NewPooledProcess(12345, "claude", []string{"--json"}, "/tmp")
	if p.PID() != 12345 {
		t.Errorf("PID = %d, want 12345", p.PID())
	}
	if p.Command() != "claude" {
		t.Errorf("Command = %q, want %q", p.Command(), "claude")
	}
	if !p.IsAlive() {
		t.Error("IsAlive = false, want true")
	}
	p.MarkDead()
	if p.IsAlive() {
		t.Error("IsAlive = true after MarkDead, want false")
	}
}

func TestPooledProcess_NilLease(t *testing.T) {
	var l *Lease
	if l.IsStale() != true {
		t.Error("nil lease should be stale")
	}
	l.Release()
	if l.HealthCheck() != false {
		t.Error("nil lease HealthCheck should be false")
	}
}
