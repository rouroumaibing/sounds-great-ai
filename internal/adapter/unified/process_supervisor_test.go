//go:build !windows

package unified

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestProcessManagerSpawnViaSupervisor proves R8 is wired into the default
// spawn path: when a supervisor binary is configured, the CLI is launched
// through sg-cli-supervisor and its stdout is still captured, while the
// tracked PID is the real CLI (read from the sidecar's child-pid file) rather
// than the idle sidecar.
func TestProcessManagerSpawnViaSupervisor(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "sg-cli-supervisor")
	build := exec.Command("go", "build", "-o", bin, "sounds-great-ai/cmd/sg-cli-supervisor")
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("skip: could not build sidecar: %v\n%s", err, out)
	}

	pm := NewProcessManager()
	pm.SupervisorBinary = bin

	h, err := pm.Spawn(context.Background(), "echo", []string{"hello-supervised"}, "")
	if err != nil {
		t.Fatalf("spawn via supervisor: %v", err)
	}
	out, err := io.ReadAll(h.Stdout)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "hello-supervised" {
		t.Fatalf("stdout through sidecar = %q, want hello-supervised", got)
	}
	if h.PID <= 0 {
		t.Fatalf("tracked PID = %d, want > 0", h.PID)
	}
	h.Wait()
}

// TestProcessManagerSupervisorFallback ensures that if the configured sidecar
// fails to start, Spawn transparently falls back to a direct one-shot spawn
// (so a broken/missing binary never breaks CLI execution).
func TestProcessManagerSupervisorFallback(t *testing.T) {
	pm := NewProcessManager()
	pm.SupervisorBinary = filepath.Join(t.TempDir(), "does-not-exist-sg-cli-supervisor")

	h, err := pm.Spawn(context.Background(), "echo", []string{"fallback-ok"}, "")
	if err != nil {
		t.Fatalf("fallback spawn should succeed: %v", err)
	}
	out, err := io.ReadAll(h.Stdout)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "fallback-ok" {
		t.Fatalf("stdout after fallback = %q, want fallback-ok", got)
	}
	h.Wait()
}

func TestResolveSupervisor(t *testing.T) {
	pm := NewProcessManager()
	if got := pm.resolveSupervisor(); got != "" {
		t.Fatalf("expected empty when no binary configured, got %q", got)
	}
	pm.SupervisorBinary = "/explicit/path"
	if got := pm.resolveSupervisor(); got != "/explicit/path" {
		t.Fatalf("expected explicit path, got %q", got)
	}
}

func TestReadChildPIDFile(t *testing.T) {
	if _, ok := readChildPIDFile(""); ok {
		t.Fatalf("empty name should return false")
	}
	f := filepath.Join(t.TempDir(), "pid.txt")
	_ = os.WriteFile(f, []byte("  12345  \n"), 0o644)
	if pid, ok := readChildPIDFile(f); !ok || pid != 12345 {
		t.Fatalf("readChildPIDFile = %d,%v want 12345,true", pid, ok)
	}
}
