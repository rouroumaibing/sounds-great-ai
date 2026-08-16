package unified

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"sounds-great-ai/internal/adapter/pool"
)

func spawnCatWarm(key pool.PoolKey) (*pool.WarmProcess, error) {
	cmd := exec.Command("cat")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return pool.NewWarmProcess(cmd, stdin, stdout, key, ""), nil
}

type mockRunner struct {
	err error
	h   *SpawnHandle
}

func (m mockRunner) RunTurn(_ context.Context, _ *pool.WarmProcess, _ *SpawnSpec) (*SpawnHandle, error) {
	return m.h, m.err
}

func TestBgDaemonTransportSuccess(t *testing.T) {
	p := pool.NewWarmPool(pool.DefaultWarmPoolConfig(), spawnCatWarm)
	defer p.Close()
	tr := NewBgDaemonTransport(p, mockRunner{h: &SpawnHandle{exitCh: make(chan struct{})}}, NewMemoryHealth())
	spec := &SpawnSpec{Command: "cat", SessionID: "claude", WorkDir: "/tmp"}
	h, err := tr.Spawn(context.Background(), spec)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil handle")
	}
}

func TestBgDaemonTransportFailureDegradesHealth(t *testing.T) {
	p := pool.NewWarmPool(pool.DefaultWarmPoolConfig(), spawnCatWarm)
	defer p.Close()
	health := NewMemoryHealth()
	tr := NewBgDaemonTransport(p, mockRunner{err: errors.New("boom")}, health)
	spec := &SpawnSpec{Command: "cat", SessionID: "claude", WorkDir: "/tmp"}
	if _, err := tr.Spawn(context.Background(), spec); err == nil {
		t.Fatal("expected error from failing runner")
	}
	info := health.Info(context.Background(), "claude")
	if info.Level == "online" {
		t.Errorf("expected degraded health after failure, got %q", info.Level)
	}
}

func TestBgDaemonTransportNotConfigured(t *testing.T) {
	tr := NewBgDaemonTransport(nil, nil, NewMemoryHealth())
	if _, err := tr.Spawn(context.Background(), &SpawnSpec{}); err == nil {
		t.Fatal("expected error when bg_daemon transport not configured")
	}
}
