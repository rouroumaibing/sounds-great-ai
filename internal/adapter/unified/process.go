package unified

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	KillGraceMs      = 3000 // SIGTERM → SIGKILL grace period (from clowder-ai)
	InterruptGraceMs = 2000 // SIGINT cooperative window
)

// ProcessManager handles CLI process spawning with process group isolation
// and graceful termination.
type ProcessManager struct {
	KillGraceMs      int
	InterruptGraceMs int
}

// NewProcessManager creates a ProcessManager with clowder-ai defaults.
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		KillGraceMs:      KillGraceMs,
		InterruptGraceMs: InterruptGraceMs,
	}
}

// Spawn starts a CLI process with process group isolation.
// The prompt is passed via stdin (not argv) to prevent cross-process leakage.
// Returns a reader for stdout. The process is killed (entire process group)
// when ctx is cancelled.
func (pm *ProcessManager) Spawn(ctx context.Context, cmd string, args []string, stdinInput string) (io.Reader, error) {
	c := exec.CommandContext(ctx, cmd, args...)

	// Process group isolation — kill entire process tree on cancellation
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Prompt via stdin, NOT argv (prevents ps/proc leakage)
	if stdinInput != "" {
		c.Stdin = strings.NewReader(stdinInput)
	}

	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := c.Start(); err != nil {
		return nil, err
	}

	// Watch for context cancellation → graceful kill
	go func() {
		<-ctx.Done()
		if c.Process == nil {
			return
		}
		pgid := c.Process.Pid
		// SIGTERM to entire process group
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		// Wait for grace period, then SIGKILL
		timer := time.NewTimer(time.Duration(pm.KillGraceMs) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		case <-time.After(time.Duration(pm.KillGraceMs*2) * time.Millisecond):
			// Process exited, no need for SIGKILL
		}
	}()

	return stdoutPipe, nil
}
