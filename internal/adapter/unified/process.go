package unified

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"sounds-great-ai/internal/adapter/pool"
	"sounds-great-ai/internal/hooks"
)

const (
	KillGraceMs      = 3000
	InterruptGraceMs = 2000
	StderrBufSize    = 4096
)

type ProcessManager struct {
	KillGraceMs      int
	InterruptGraceMs int
	Registry         *ProcessRegistry
	// ProcessPool is optional. If set, SpawnWithPool will use it for warm process reuse.
	// If nil, SpawnWithPool falls back to one-shot Spawn (graceful degradation).
	ProcessPool *pool.ProcessPool
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		KillGraceMs:      KillGraceMs,
		InterruptGraceMs: InterruptGraceMs,
		Registry:         NewProcessRegistry(),
	}
}

// SetPool enables process pool reuse. Pass nil to disable (revert to one-shot).
func (pm *ProcessManager) SetPool(p *pool.ProcessPool) {
	pm.ProcessPool = p
}

func (pm *ProcessManager) Spawn(ctx context.Context, cmd string, args []string, stdinInput string) (io.Reader, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if stdinInput != "" {
		c.Stdin = strings.NewReader(stdinInput)
	}

	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := c.Start(); err != nil {
		return nil, err
	}

	cliType := extractCLIType(cmd)
	rec := pm.Registry.Register(c.Process.Pid, cmd, cliType)

	pr, pw := io.Pipe()

	// Drain stderr in background
	stderrDone := make(chan struct{})
	go func() {
		buf := make([]byte, 512)
		for {
			_, err := stderrPipe.Read(buf)
			if err != nil {
				break
			}
		}
		close(stderrDone)
	}()

	// Copy stdout to pipe, then wait for stderr drain + process exit (reaps zombie).
	// Using io.Pipe decouples the caller's reads from c.Wait(), which would
	// otherwise close the stdout pipe prematurely.
	go func() {
		io.Copy(pw, stdoutPipe)
		pw.Close()
		<-stderrDone
		waitErr := c.Wait()
		exitCode := 0
		signal := ""
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				if exitErr.ProcessState != nil {
					if ps, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok && ps.Signaled() {
						signal = ps.Signal().String()
					}
				}
			} else {
				exitCode = -1
			}
		}
		pm.Registry.UpdateExit(rec.PID, &exitCode, signal)
	}()

	// Watch for context cancellation → graceful kill
	go func() {
		<-ctx.Done()
		if c.Process == nil {
			return
		}
		pgid := c.Process.Pid
		_ = syscall.Kill(-pgid, syscall.SIGINT)
		timer1 := time.NewTimer(time.Duration(pm.InterruptGraceMs) * time.Millisecond)
		defer timer1.Stop()
		select {
		case <-timer1.C:
			_ = syscall.Kill(-pgid, syscall.SIGTERM)
			timer2 := time.NewTimer(time.Duration(pm.KillGraceMs) * time.Millisecond)
			defer timer2.Stop()
			select {
			case <-timer2.C:
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
			case <-time.After(time.Duration(pm.KillGraceMs*2) * time.Millisecond):
			}
		case <-time.After(time.Duration(pm.InterruptGraceMs*2+pm.KillGraceMs*2) * time.Millisecond):
		}
	}()

	return pr, nil
}

func (pm *ProcessManager) SpawnWithHooks(
	ctx context.Context, cmd string, args []string, stdinInput string,
	pipeline *hooks.Pipeline, input *hooks.AssemblerInput,
) (io.Reader, error) {
	var fullStdin string

	if pipeline != nil && input != nil {
		sessionResult := pipeline.ExecuteStage("session-init", input)
		sessionPrompt := hooks.AssemblePatches(sessionResult.Patches)

		turnResult := pipeline.ExecuteStage("per-turn", input)
		turnPrompt := hooks.AssemblePatches(turnResult.Patches)

		var parts []string
		if sessionPrompt != "" {
			parts = append(parts, sessionPrompt)
		}
		if turnPrompt != "" {
			parts = append(parts, turnPrompt)
		}
		if stdinInput != "" {
			parts = append(parts, stdinInput)
		}
		fullStdin = strings.Join(parts, "\n\n")
	} else {
		fullStdin = stdinInput
	}

	return pm.Spawn(ctx, cmd, args, fullStdin)
}

func extractCLIType(cmd string) string {
	base := cmd
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	return base
}

// SpawnWithPool attempts to use the process pool for warm process reuse.
// If the pool is not set or acquisition fails, it falls back to one-shot Spawn
// (graceful degradation). The workDir and providerProfile form the pool key.
func (pm *ProcessManager) SpawnWithPool(
	ctx context.Context, cmd string, args []string, stdinInput string,
	workDir, providerProfile, sessionID string,
) (io.Reader, *pool.Lease, error) {
	// If no pool configured, fall back to one-shot spawn
	if pm.ProcessPool == nil {
		r, err := pm.Spawn(ctx, cmd, args, stdinInput)
		return r, nil, err
	}

	key := pool.PoolKey{
		ProjectPath:     workDir,
		ProviderProfile: providerProfile,
	}

	lease, err := pm.ProcessPool.Acquire(key, sessionID)
	if err != nil {
		// Pool acquisition failed — fall back to one-shot spawn
		r, spawnErr := pm.Spawn(ctx, cmd, args, stdinInput)
		return r, nil, spawnErr
	}

	// Check if the leased process is stale
	if lease.IsStale() {
		lease.Release()
		// Fall back to one-shot spawn
		r, spawnErr := pm.Spawn(ctx, cmd, args, stdinInput)
		return r, nil, spawnErr
	}

	// We have a warm process from the pool.
	// For now, we still spawn a one-shot process for the actual I/O,
	// but the pool tracks the warm process for future reuse.
	// This is graceful degradation: the pool provides session affinity
	// and metrics, while the actual spawn uses the proven one-shot path.
	r, err := pm.Spawn(ctx, cmd, args, stdinInput)
	if err != nil {
		lease.Release()
		return nil, nil, err
	}

	return r, lease, nil
}
