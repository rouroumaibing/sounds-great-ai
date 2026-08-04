package unified

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestProcessManagerSpawnAndRead(t *testing.T) {
	pm := NewProcessManager()
	r, err := pm.Spawn(context.Background(), "echo", []string{"hello"}, "")
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("expected 'hello' in output, got %q", buf.String())
	}
}

func TestProcessManagerStdinInjection(t *testing.T) {
	pm := NewProcessManager()
	r, err := pm.Spawn(context.Background(), "cat", nil, "injected-via-stdin")
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "injected-via-stdin") {
		t.Fatalf("expected stdin content in output, got %q", buf.String())
	}
}

func TestProcessManagerContextCancellation(t *testing.T) {
	pm := NewProcessManager()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _ = pm.Spawn(ctx, "sleep", []string{"10"}, "")
	// The process should be killed when ctx expires
	// We just verify Spawn doesn't hang
}

func TestProcessManagerDefaults(t *testing.T) {
	pm := NewProcessManager()
	if pm.KillGraceMs != 3000 {
		t.Errorf("KillGraceMs = %d, want 3000", pm.KillGraceMs)
	}
	if pm.InterruptGraceMs != 2000 {
		t.Errorf("InterruptGraceMs = %d, want 2000", pm.InterruptGraceMs)
	}
}
