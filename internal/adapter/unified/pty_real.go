package unified

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/creack/pty"

	"sounds-great-ai/internal/adapter/pool"
)

// PtyTransport is the interactive_pty (R3) carrier tier: launch the CLI inside
// a pseudo-terminal so it sees a real TTY. Some CLIs require this for billing
// identity or interactive attach. It is compiled in unconditionally (no `pty`
// build tag required; `github.com/creack/pty` is a regular dependency). Per
// ADR-002, R3 is reserved/opt-in and only enters a carrier chain when a CLI
// requires a real TTY; the warm-pool (R2 bg_daemon) tier is now DEFAULT-ON via
// platform.WireWarmPools.
//
// Alignment with F230 PtyDriver (R3 gap remediation, "不臆想，按实际代码"):
//   - ready probe: a fixed grace after pty.Start before injecting the prompt
//     (PtyDriver Note 1: "no screen scraping — grace is sufficient" — the TUI
//     reaches its ❯ prompt within 10-15s, so a grace beats scraping).
//   - bypass confirmation screen: a regex matched against a bounded read-ahead
//     of the startup banner; if it matches, BypassKeys are sent (PtyDriver
//     lines 155-176: the claude 2.1.170+ "trust this project?" screen →
//     Down+Enter → "Yes, I accept"). Empty pattern = disabled (most CLIs need
//     none), so the default never consumes output and NDJSON framing is intact.
//   - cancel(): on context cancellation we send the InterruptKey (default ESC)
//     so the CLI can emit "[Request interrupted by user]" and KEEP the session
//     alive for resume, instead of being SIGKILLed. We therefore use
//     exec.Command (not CommandContext) so the OS does not hard-kill on cancel.
//   - resume: when ResumeSupported, a `--resume <id>` is appended so multi-turn
//     context is preserved via the CLI's own session mechanism (PtyDriver
//     `--resume <id>`).
//
// The tmux-transcript + Hook side-channel parts of PtyDriver are architectural
// to the upstream design (it drives claude's TUI and reads transcript files). SG reads the
// CLI's NDJSON stream directly off the PTY master, so those parts do not map
// onto this model; they are documented as design gaps in
// docs/cli-adapter-diff.md rather than copied.
type PtyTransport struct {
	cfg PtyConfig
}

// NewPtyTransport builds the interactive_pty transport. The optional PtyConfig
// tunes ready-probe / bypass / cancel behavior; zero config uses defaults.
// The signature is variadic so it matches pty_stub.go under both build tags
// and the default (no-arg) registration in platform.go.
func NewPtyTransport(cfg ...PtyConfig) *PtyTransport {
	c := defaultPtyConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &PtyTransport{cfg: c}
}

// Kind implements Transport.
func (t *PtyTransport) Kind() TransportKind { return TransportInteractivePTY }

// Spawn launches the CLI attached to a pty (real TTY) instead of a plain
// stdout pipe, waits for readiness, optionally bypasses a consent screen, feeds
// the prompt, and returns a handle whose stdout carries the CLI's NDJSON stream.
func (t *PtyTransport) Spawn(ctx context.Context, spec *SpawnSpec) (*SpawnHandle, error) {
	// R3/tmux: when enabled for claude and tmux is present, drive claude through
	// a detached tmux session + transcript/Hook side-channels. Any failure here
	// falls back to the direct pty.Start path below (the caller's carrier chain
	// also still has print_sdk as a further fallback).
	if t.cfg.TmuxMode && extractCLIType(spec.Command) == "claude" && tmuxAvailable() {
		if h, err := t.spawnViaTmux(ctx, spec); err == nil {
			return h, nil
		} else {
			log.Printf("R3/tmux: spawnViaTmux failed (%v); falling back to direct pty", err)
		}
	}

	args := append([]string{}, spec.Args...)
	if t.cfg.ResumeSupported && spec.ResumeSessionID != "" {
		args = append(args, "--resume", spec.ResumeSessionID)
	}
	cmd := exec.Command(spec.Command, args...)
	if spec.WorkDir != "" {
		cmd.Dir = spec.WorkDir
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	// Ready probe + optional consent-screen bypass (PtyDriver.start()).
	if err := readyAndBypass(ptmx, t.cfg); err != nil {
		_ = ptmx.Close()
		return nil, err
	}

	// Feed the prompt into the PTY so the CLI (reading from the TTY's stdin)
	// receives it. For CLIs that take the prompt as a CLI arg (e.g. `-p`),
	// this is a harmless no-op; for REPL-style CLIs it is required.
	if spec.StdinInput != "" {
		_, _ = io.WriteString(ptmx, spec.StdinInput)
		_, _ = io.WriteString(ptmx, "\n")
	}

	stderrBuf := &bytes.Buffer{}
	handle := &SpawnHandle{
		Stdout: ptmx,
		PID:    cmd.Process.Pid,
		stderr: stderrBuf,
		exitCh: make(chan struct{}),
	}

	// On context cancellation, send the interrupt key (default ESC) so the CLI
	// can abort the turn gracefully while keeping the session alive for resume
	// (PtyDriver.cancel()). We deliberately avoid exec.CommandContext so the OS
	// does not SIGKILL the process on cancel.
	go func() {
		<-ctx.Done()
		_, _ = ptmx.Write([]byte(t.cfg.InterruptKey))
	}()

	go func() {
		waitErr := cmd.Wait()
		exitCode := 0
		signal := ""
		if waitErr != nil {
			if ee, ok := waitErr.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				exitCode = -1
			}
		}
		handle.mu.Lock()
		handle.exitCode = &exitCode
		handle.signal = signal
		handle.mu.Unlock()
		close(handle.exitCh)
		handle.runOnExit()
	}()

	return handle, nil
}

// PtyRunner drives one user turn through a warm (persistent) PTY process,
// implementing unified.WarmRunner. It writes the prompt to the process's TTY
// stdin and frames its NDJSON stdout into a SpawnHandle stream. Wait() returns
// when the turn completes — either the CLI exits (one-shot) or it emits a
// terminal NDJSON event (`result`/`done`), so the process can be released back
// to the pool and reused for the next turn without being killed.
type PtyRunner struct {
	cfg PtyConfig
}

// NewPtyRunner builds a warm-runner with optional PTY config (default ESC
// interrupt key). PtyRunner{} also works, falling back to defaults per-turn.
func NewPtyRunner(cfg ...PtyConfig) PtyRunner {
	c := defaultPtyConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return PtyRunner{cfg: c}
}

// RunTurn implements WarmRunner.
func (r PtyRunner) RunTurn(ctx context.Context, wp *pool.WarmProcess, spec *SpawnSpec) (*SpawnHandle, error) {
	if wp == nil || !wp.Alive() {
		return nil, fmt.Errorf("pty runner: warm process not alive")
	}

	// On context cancellation, send the interrupt key to the shared TTY so the
	// in-progress turn is aborted but the warm process survives for reuse.
	interruptKey := r.cfg.InterruptKey
	if interruptKey == "" {
		interruptKey = defaultPtyConfig().InterruptKey
	}
	go func() {
		<-ctx.Done()
		_, _ = io.WriteString(wp.Stdin(), interruptKey)
	}()

	// Inject the prompt into the PTY (TTY stdin).
	if spec.StdinInput != "" {
		if _, err := io.WriteString(wp.Stdin(), spec.StdinInput); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(wp.Stdin(), "\n"); err != nil {
			return nil, err
		}
	}

	pr, pw := io.Pipe()
	exitCh := make(chan struct{})
	handle := &SpawnHandle{
		Stdout: pr,
		PID:    wp.PID(),
		stderr: &bytes.Buffer{},
		exitCh: exitCh,
	}

	// Stream the PTY's NDJSON stdout, stopping at turn end (terminal event or
	// EOF). A fresh reader is created per turn; the PTY master is a stream, so
	// the next RunTurn resumes reading from the current offset.
	go func() {
		defer pw.Close()
		reader := bufio.NewReader(wp.Stdout())
		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				var obj map[string]any
				if jErr := json.Unmarshal([]byte(line), &obj); jErr == nil {
					if t, _ := obj["type"].(string); t == "result" || t == "done" {
						_, _ = pw.Write([]byte(line))
						close(exitCh)
						return
					}
				}
				_, _ = pw.Write([]byte(line))
			}
			if err != nil {
				// EOF or read error → turn (and likely the process) is done.
				close(exitCh)
				return
			}
		}
	}()

	return handle, nil
}

// PtyWarmSpawnFunc returns a pool.WarmSpawnFunc that starts `command args` in a
// pseudo-terminal (so it sees a real TTY) and wraps it as a reusable warm
// process. Use it to build a PTY-backed WarmPool for RegisterWarmPool:
//
//	pool := pool.NewWarmPool(cfg, pty.PtyWarmSpawnFunc("claude", args, dir, pty.PtyConfig{ResumeSupported: true}))
//	platform.RegisterWarmPool(pool, pty.NewPtyRunner())
//
// The PTY master is both the process's stdin and stdout, so WarmProcess.Stdin()
// and Stdout() both return it. Ready-probe + consent bypass run once here (at
// process start), mirroring PtyDriver.start().
func PtyWarmSpawnFunc(command string, args []string, workDir string, cfg PtyConfig) pool.WarmSpawnFunc {
	return func(key pool.PoolKey) (*pool.WarmProcess, error) {
		fullArgs := append([]string{}, args...)
		if cfg.ResumeSupported && cfg.ResumeSessionID != "" {
			fullArgs = append(fullArgs, "--resume", cfg.ResumeSessionID)
		}
		cmd := exec.Command(command, fullArgs...)
		if workDir != "" {
			cmd.Dir = workDir
		}
		ptmx, err := pty.Start(cmd)
		if err != nil {
			return nil, err
		}
		// Readiness probe + consent bypass once at process start.
		if err := readyAndBypass(ptmx, cfg); err != nil {
			_ = ptmx.Close()
			return nil, err
		}
		// ptmx is the master: writing sends to the CLI's stdin, reading receives
		// its stdout/stderr. As a WarmProcess it is both stdin and stdout.
		return pool.NewWarmProcess(cmd, ptmx, ptmx, key, workDir), nil
	}
}

// readyAndBypass implements the PtyDriver.start() readiness + consent
// bypass. It first waits a fixed grace (ReadyGraceMs) for the CLI to reach its
// REPL prompt, then performs a bounded read-ahead of the startup banner; if
// BypassPattern matches, it sends BypassKeys (e.g. Down+Enter to accept claude's
// trust screen). The read-ahead only runs when BypassPattern is set; with the
// default empty pattern the function returns after the grace without consuming
// any output, so NDJSON framing downstream is never disturbed. ptmx is the PTY
// master (*os.File), which supports SetReadDeadline for the bounded read.
func readyAndBypass(ptmx *os.File, cfg PtyConfig) error {
	if cfg.ReadyGraceMs > 0 {
		time.Sleep(time.Duration(cfg.ReadyGraceMs) * time.Millisecond)
	}
	if cfg.BypassPattern == "" {
		return nil
	}
	re, err := regexp.Compile(cfg.BypassPattern)
	if err != nil {
		return fmt.Errorf("pty: invalid bypass pattern: %w", err)
	}
	// Bounded read-ahead of the startup banner. The consent screen appears
	// here (never after the first prompt is injected), so consuming these bytes
	// is safe: they are not NDJSON and would be ignored by the framing reader.
	buf := make([]byte, 8192)
	_ = ptmx.SetReadDeadline(time.Now().Add(800 * time.Millisecond))
	n, _ := ptmx.Read(buf)
	_ = ptmx.SetReadDeadline(time.Time{})
	if n > 0 && re.Match(buf[:n]) {
		if _, err := ptmx.Write([]byte(cfg.BypassKeys)); err != nil {
			return err
		}
		// Grace for the TUI to register the selection (PtyDriver bypassGraceMs).
		if cfg.ReadyGraceMs > 0 {
			time.Sleep(time.Duration(cfg.ReadyGraceMs) * time.Millisecond / 3)
		}
	}
	return nil
}
