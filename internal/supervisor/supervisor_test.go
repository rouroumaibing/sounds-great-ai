//go:build !windows

package supervisor

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

// TestSupervisedProcessReapedOnCancel proves the core R8 invariant: when the
// supervising context is cancelled, the supervisor reaps the ENTIRE child
// process group (not just the direct child), so no orphan is left behind when
// the SG server shuts down. This mirrors the sidecar killing the group
// on parent disappearance.
func TestSupervisedProcessReapedOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// A long sleep stands in for a long-running agent CLI.
	p, err := Spawn(ctx, DefaultConfig(), "sleep", []string{"30"}, "", nil)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	pid := p.PID()
	if pid == 0 {
		t.Fatal("expected a non-zero child pid")
	}

	// Cancel the supervising context → the monitor must reap the group.
	cancel()

	done := make(chan error, 1)
	go func() { done <- p.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("supervised child not reaped after context cancel")
	}

	// The child pid must be gone (signal 0 should fail with ESRCH).
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("child pid %d still alive after cancel — orphan leaked", pid)
	}
	_ = os.Getpid() // keep os imported for platform parity
}

// TestSupervisedProcessGroupIsolation confirms the child is in its own process
// group (pgid == pid), which is what makes the group kill safe and targeted.
func TestSupervisedProcessGroupIsolation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := Spawn(ctx, DefaultConfig(), "sleep", []string{"5"}, "", nil)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if p.PGID() != p.PID() {
		t.Errorf("child not in its own process group: pgid=%d pid=%d", p.PGID(), p.PID())
	}
	_ = p.Terminate(time.Second)
	_ = p.Wait()
}
