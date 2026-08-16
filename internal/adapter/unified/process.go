package unified

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"sounds-great-ai/internal/adapter/pool"
	"sounds-great-ai/internal/hooks"
)

const (
	KillGraceMs      = 3000
	InterruptGraceMs = 2000
	// StderrBufSize caps captured stderr (G2). Diagnostics only needs enough
	// context to classify failures; we keep the head and stop growing past this.
	StderrBufSize = 16384
	// Liveness probe defaults (G4): poll every second; flag a child as
	// "idle but silent" after 30s of no output and no CPU growth, and as a
	// hard stall after 120s.
	ProbePollInterval = 1 * time.Second
	ProbeSoftWarnMs   = 30000
	ProbeStallWarnMs  = 120000
)

// SpawnHandle is the result of ProcessManager.Spawn. It exposes the child's
// stdout stream plus the bookkeeping needed for diagnostics (G2) and lifecycle
// hooks (G5): the captured stderr, the exit code/signal, and OnExit callbacks
// that run after the process has fully reaped.
type SpawnHandle struct {
	Stdout io.Reader
	PID    int

	stderr *bytes.Buffer
	exitCh chan struct{}
	onExit []func()
	mu     sync.Mutex
	exitCode *int
	signal   string
	probe    *LivenessProbe
	// onStallCb is set by adapters to surface liveness warnings to the client
	// (R8). It is invoked (under lock) when the liveness probe transitions.
	onStallCb func(state ProbeState, hard bool)
	// stalled is set when the probe observed an idle_silent (alive but no
	// output/CPU) period; used by diagnostics to classify cli_stall_timeout
	// (R7) when the child ultimately fails.
	stalled bool
	// streamErrText records the last NDJSON-level error line (R5 dual-source
	// diagnostics) so classification can fall back to it when stderr is empty.
	streamErrText string
}

// SetOnStall registers a callback invoked when the liveness probe detects a
// stalled (alive-but-silent) child or recovers. Optional; defaults to none.
func (h *SpawnHandle) SetOnStall(cb func(state ProbeState, hard bool)) {
	h.mu.Lock()
	h.onStallCb = cb
	h.mu.Unlock()
}

// SetStreamError records the last NDJSON-level error line seen while streaming
// (R5). It is used as a secondary classification source in diagnostics when
// stderr is empty. Only the first error is kept (later lines are usually noise
// following the root cause).
func (h *SpawnHandle) SetStreamError(text string) {
	h.mu.Lock()
	if h.streamErrText == "" {
		h.streamErrText = text
	}
	h.mu.Unlock()
}

// stalledFlag reports whether a stall was observed during the child's lifetime.
func (h *SpawnHandle) stalledFlag() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stalled
}

// streamErrTextSafe returns the recorded NDJSON-level error text (R5).
func (h *SpawnHandle) streamErrTextSafe() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.streamErrText
}

// Wait blocks until the child process has exited and been reaped. A nil
// exitCh (e.g. a synthetic handle used in tests) is treated as already-exited
// so diagnostics can run without a real process behind the handle.
func (h *SpawnHandle) Wait() {
	if h.exitCh == nil {
		return
	}
	<-h.exitCh
}

// ExitInfo returns the exit code and terminating signal (if any) once the
// process has exited. Before exit it returns (nil, "").
func (h *SpawnHandle) ExitInfo() (exitCode *int, signal string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitCode, h.signal
}

// StderrString returns the captured stderr output (caller must still sanitize
// before surfacing — see diagnostics.BuildDiagnostics).
func (h *SpawnHandle) StderrString() string {
	if h.stderr == nil {
		return ""
	}
	return h.stderr.String()
}

// OnExit registers a callback to run once the process has exited. Used for
// temporary-file cleanup (G5: remove the ephemeral MCP config).
func (h *SpawnHandle) OnExit(f func()) {
	h.mu.Lock()
	h.onExit = append(h.onExit, f)
	h.mu.Unlock()
}

func (h *SpawnHandle) runOnExit() {
	h.mu.Lock()
	cbs := h.onExit
	h.mu.Unlock()
	for _, f := range cbs {
		f()
	}
}

// cappedWriter is a bytes.Buffer wrapper that stops growing past `limit`,
// keeping the head of the stream (sufficient for error classification).
type cappedWriter struct {
	w      *bytes.Buffer
	limit  int
}

func (c *cappedWriter) Write(p []byte) (int, error) {
	if c.w.Len() >= c.limit {
		return len(p), nil
	}
	return c.w.Write(p)
}

type ProcessManager struct {
	KillGraceMs      int
	InterruptGraceMs int
	Registry         *ProcessRegistry
	// ProcessPool is optional. If set, SpawnWithPool will use it for warm
	// process reuse. If nil, SpawnWithPool falls back to one-shot Spawn.
	// NOTE (G3): the pool currently records session affinity + metrics only;
	// the actual CLI invocation remains one-shot. True warm reuse requires a
	// persistent-process transport (VISION-level decision) and is not wired.
	ProcessPool *pool.ProcessPool
	// OnStall is notified when the liveness probe detects a stalled (alive but
	// silent) child. Optional; defaults to a no-op logger.
	OnStall func(pid int, state ProbeState)
	// SupervisorBinary, when non-empty, is the path to sg-cli-supervisor. When
	// set (or auto-resolved), Spawn wraps the CLI in the supervisor sidecar so a
	// hard SIGKILL of the SG server still reaps the CLI's process group. Empty
	// = direct one-shot spawn (legacy behavior, zero extra process).
	SupervisorBinary string
}

// resolveSupervisor returns the supervisor binary to use, or "" if none is
// configured/available. When SupervisorBinary is empty we look next to the
// running executable and then in PATH, matching the bundled-sidecar
// model: production bundles it (active), dev/test without it fall back to
// direct spawn.
func (pm *ProcessManager) resolveSupervisor() string {
	if pm.SupervisorBinary != "" {
		return pm.SupervisorBinary
	}
	if p, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(p), "sg-cli-supervisor")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if p, err := exec.LookPath("sg-cli-supervisor"); err == nil {
		return p
	}
	return ""
}

// readChildPIDFile polls the sidecar's child-pid file for up to ~1.5s and
// returns the CLI PID once the sidecar has written it.
func readChildPIDFile(name string) (int, bool) {
	if name == "" {
		return 0, false
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(name); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
				return pid, true
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return 0, false
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

// Spawn launches the CLI command as the leader of its own process group
// (Setpgid) so the entire child subtree can be signalled together. It returns a
// SpawnHandle exposing stdout plus diagnostics/lifecycle hooks.
//
// R8: when a supervisor sidecar is available, the CLI is launched THROUGH
// sg-cli-supervisor (a separate process) so that a hard SIGKILL of the SG
// server still reaps the CLI's process group via the sidecar's parent-poll.
// This mirrors the cli-supervisor: production bundles the sidecar
// (active by default), while dev/test without it fall back to a direct
// one-shot spawn (zero extra process). The sidecar writes its child's real PID
// to a temp file so liveness probing targets the actual CLI, not the idle
// sidecar.
func (pm *ProcessManager) Spawn(ctx context.Context, cmd string, args []string, stdinInput string) (*SpawnHandle, error) {
	sup := pm.resolveSupervisor()
	var pidFileName string
	if sup != "" {
		// Disable CommandContext's default SIGKILL-on-cancel: cancellation is
		// handled by the goroutine below, which SIGTERMs the sidecar so it can
		// clean its own child group (a direct SIGKILL would orphan the CLI).
		c := exec.CommandContext(ctx, sup, append([]string{"--", cmd}, args...)...)
		c.Env = append(os.Environ(),
			"SG_SUPERVISOR_PARENT_PID="+strconv.Itoa(os.Getpid()),
			"SG_SUPERVISOR_POLL_MS=1000",
			"SG_SUPERVISOR_KILL_GRACE_MS="+strconv.Itoa(pm.KillGraceMs),
		)
		if tf, err := os.CreateTemp("", "sg-sup-pid-*.txt"); err == nil {
			_ = tf.Close()
			pidFileName = tf.Name()
			c.Env = append(c.Env, "SG_SUPERVISOR_CHILD_PID_FILE="+pidFileName)
			defer os.Remove(pidFileName)
		}
		c.Cancel = func() error { return nil }
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
			log.Printf("R8: supervisor start failed (%v); falling back to direct spawn", err)
			sup = ""
			c = exec.CommandContext(ctx, cmd, args...)
			c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			if stdinInput != "" {
				c.Stdin = strings.NewReader(stdinInput)
			}
			stdoutPipe, err = c.StdoutPipe()
			if err != nil {
				return nil, err
			}
			stderrPipe, err = c.StderrPipe()
			if err != nil {
				return nil, err
			}
			if err := c.Start(); err != nil {
				return nil, err
			}
		}
		log.Printf("R8: spawning %q via supervisor (parent pid %d)", cmd, os.Getpid())
		return pm.finishSpawn(ctx, cmd, c, pidFileName, sup, stdoutPipe, stderrPipe)
	}

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
	return pm.finishSpawn(ctx, cmd, c, "", "", stdoutPipe, stderrPipe)
}

// finishSpawn builds the SpawnHandle and wires diagnostics/lifecycle/stall
// hooks around an already-started command. pidFileName is non-empty only when
// the CLI runs under the supervisor sidecar (so liveness probes the real CLI).
func (pm *ProcessManager) finishSpawn(ctx context.Context, cmd string, c *exec.Cmd, pidFileName, sup string, stdoutPipe, stderrPipe io.Reader) (*SpawnHandle, error) {
	trackPID := c.Process.Pid
	if sup != "" {
		if cliPID, ok := readChildPIDFile(pidFileName); ok {
			trackPID = cliPID
		}
	}

	cliType := extractCLIType(cmd)
	rec := pm.Registry.Register(trackPID, cmd, cliType)

	stderrBuf := &bytes.Buffer{}
	handle := &SpawnHandle{
		PID:    trackPID,
		stderr: stderrBuf,
		exitCh: make(chan struct{}),
	}

	pr, pw := io.Pipe()

	// Capture stderr into a bounded buffer (G2) instead of discarding it, so
	// failures can be classified + sanitized rather than silently lost.
	go func() {
		_, _ = io.Copy(&cappedWriter{w: stderrBuf, limit: StderrBufSize}, stderrPipe)
	}()

	// Copy stdout to the pipe, then wait for process exit (reaps zombie) and
	// records diagnostics before signalling completion + OnExit hooks.
	go func() {
		_, _ = io.Copy(pw, stdoutPipe)
		pw.Close()
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
		handle.mu.Lock()
		handle.exitCode = &exitCode
		handle.signal = signal
		handle.mu.Unlock()
		if handle.probe != nil {
			handle.probe.Stop()
		}
		close(handle.exitCh)
		handle.runOnExit()
	}()

	// Liveness probe (G4): detect stalled children. Wired before returning so
	// its goroutine is always stopped by the wait goroutine above. When running
	// under the supervisor, probe the real CLI pid (trackPID) rather than the
	// mostly-idle sidecar that owns the process group.
	probe := NewLivenessProbe(trackPID, ProbePollInterval, ProbeSoftWarnMs, ProbeStallWarnMs)
	probe.OnStall = func(state ProbeState, hard bool) {
		if pm.OnStall != nil {
			pm.OnStall(trackPID, state)
		}
		handle.mu.Lock()
		cb := handle.onStallCb
		if state == ProbeIdleSilent {
			handle.stalled = true
		}
		handle.mu.Unlock()
		if cb != nil {
			cb(state, hard)
		}
	}
	handle.probe = probe
	probe.Start()

	// G8: process-group leadership (Setpgid above) lets ctx cancellation kill
	// the entire child group via -pgid. A genuine orphan watchdog against a
	// SIGKILL of the API process would require a separate supervisor process
	// (out of scope here); the ctx-based kill covers the normal shutdown path,
	// and Setpgid ensures no child is left attached to a controlling terminal.
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

	handle.Stdout = pr
	return handle, nil
}

// SpawnWithHooks composes session-init + per-turn hook patches into stdin and
// delegates to Spawn.
func (pm *ProcessManager) SpawnWithHooks(
	ctx context.Context, cmd string, args []string, stdinInput string,
	pipeline *hooks.Pipeline, input *hooks.AssemblerInput,
) (*SpawnHandle, error) {
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

// SpawnWithPool records the warm-process lease (session affinity + metrics) but
// performs the actual CLI invocation as a one-shot Spawn — matching the
// current one-shot CLI adapter model (G3). True warm reuse (persistent process
// + request/response framing) is a separate transport effort and is not wired.
func (pm *ProcessManager) SpawnWithPool(
	ctx context.Context, cmd string, args []string, stdinInput string,
	workDir, providerProfile, sessionID string,
) (*SpawnHandle, *pool.Lease, error) {
	if pm.ProcessPool == nil {
		h, err := pm.Spawn(ctx, cmd, args, stdinInput)
		return h, nil, err
	}

	key := pool.PoolKey{
		ProjectPath:     workDir,
		ProviderProfile: providerProfile,
	}

	lease, err := pm.ProcessPool.Acquire(key, sessionID)
	if err != nil {
		h, spawnErr := pm.Spawn(ctx, cmd, args, stdinInput)
		return h, nil, spawnErr
	}

	if lease.IsStale() {
		lease.Release()
		h, spawnErr := pm.Spawn(ctx, cmd, args, stdinInput)
		return h, nil, spawnErr
	}

	h, err := pm.Spawn(ctx, cmd, args, stdinInput)
	if err != nil {
		lease.Release()
		return nil, nil, err
	}

	return h, lease, nil
}
