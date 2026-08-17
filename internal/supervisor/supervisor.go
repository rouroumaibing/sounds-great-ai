// Package supervisor provides orphan-process cleanup for long-running CLI
// children, faithfully mirroring the cli-supervisor sidecar (R8).
//
// Why this exists: on macOS/Linux a detached child CLI is NOT reaped when its
// parent (the SG server) is SIGKILLed or force-restarted. SG solves this
// with a sidecar process that sits between the API parent and each long-running
// CLI, then terminates the supervised process GROUP if the original parent
// disappears. SG's Go equivalent:
//
//   - The child is spawned in its OWN process group (Setpgid on Unix, a job
//     object on Windows) so the whole tree can be reaped with one signal.
//   - In-process mode (default): a monitor goroutine reaps the group when the
//     supervising context is cancelled (graceful shutdown / ctx cancel).
//   - Sidecar mode (opt-in): re-execs the sg-cli-supervisor binary, which polls
//     the original parent PID and kills the group even if THIS process is
//     SIGKILLed (the case where no in-process goroutine can run).
//
// The mechanism is opt-in and adds no external dependencies (pure stdlib).
package supervisor

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"
)

// Environment keys passed to the sidecar, prefixed SG_ (same shape as the
// SG_SUPERVISOR_* supervisor keys).
const (
	EnvParentPID = "SG_SUPERVISOR_PARENT_PID"
	EnvPollMs    = "SG_SUPERVISOR_POLL_MS"
	EnvGraceMs   = "SG_SUPERVISOR_KILL_GRACE_MS"
	// SidecarBin is the re-executable supervisor sidecar (built from
	// cmd/sg-cli-supervisor). Resolved next to the running server binary.
	SidecarBin = "sg-cli-supervisor"
)

// Config tunes the supervisor.
type Config struct {
	// PollInterval is how often the sidecar checks parent liveness. Default 1s.
	PollInterval time.Duration
	// KillGrace is the SIGTERM→SIGKILL grace after a terminate. Default 3s.
	KillGrace time.Duration
	// Sidecar, when true, re-execs sg-cli-supervisor so orphan cleanup survives
	// a hard SIGKILL of the supervising process. When false, an in-process
	// monitor reaps the group on ctx cancel (graceful shutdown).
	Sidecar bool
}

// DefaultConfig returns safe defaults (1s poll, 3s grace, in-process).
func DefaultConfig() Config {
	return Config{PollInterval: time.Second, KillGrace: 3 * time.Second}
}

// SupervisedProcess is a CLI launched under supervisor control. Its child runs
// in its own process group (Unix) / job (Windows) so the whole tree can be
// reaped with one signal — the "kill the process group" semantic.
type SupervisedProcess struct {
	cmd     *exec.Cmd // in-process mode
	sidecar *exec.Cmd // sidecar mode (non-nil when Sidecar)
	pgid    int       // child process group id (Unix)
	Stdin   io.WriteCloser
	Stdout  io.Reader
	Stderr  io.Reader
}

// PID returns the supervised child's pid in in-process mode, or the sidecar's
// pid in sidecar mode (the true child pid is owned by the sidecar).
func (p *SupervisedProcess) PID() int {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	if p.sidecar != nil && p.sidecar.Process != nil {
		return p.sidecar.Process.Pid
	}
	return 0
}

// PGID returns the child process group id (Unix). 0 on unsupported platforms.
func (p *SupervisedProcess) PGID() int { return p.pgid }

// KillGroup sends sig to the supervised child's entire process group (Unix:
// -pgid; Windows: the job object). Mirrors signalChild on the detached
// child's process group.
func (p *SupervisedProcess) KillGroup(sig os.Signal) error {
	return killProcessGroup(p.pgid, sig)
}

// Terminate performs a graceful-then-forced teardown: SIGTERM, then SIGKILL
// after KillGrace. Mirrors terminateChild (SIGTERM → grace → SIGKILL).
func (p *SupervisedProcess) Terminate(grace time.Duration) error {
	_ = p.KillGroup(syscallSIGTERM())
	if grace <= 0 {
		return p.KillGroup(syscallSIGKILL())
	}
	done := make(chan error, 1)
	go func() {
		done <- p.Wait()
	}()
	select {
	case <-done:
	case <-time.After(grace):
		return p.KillGroup(syscallSIGKILL())
	}
	return nil
}

// Wait blocks until the supervised process (or its sidecar) exits.
func (p *SupervisedProcess) Wait() error {
	if p.cmd != nil {
		return p.cmd.Wait()
	}
	if p.sidecar != nil {
		return p.sidecar.Wait()
	}
	return nil
}

// Spawn launches command under supervisor control. workDir and env are applied.
// When cfg.Sidecar is true it re-execs the sg-cli-supervisor binary (faithful
// to the sidecar); otherwise it spawns directly and runs an in-process
// monitor that reaps the group on ctx cancellation.
func Spawn(ctx context.Context, cfg Config, command string, args []string, workDir string, env []string) (*SupervisedProcess, error) {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.KillGrace <= 0 {
		cfg.KillGrace = 3 * time.Second
	}
	if cfg.Sidecar {
		return spawnSidecar(cfg, command, args, workDir, env)
	}
	return spawnInProcess(ctx, cfg, command, args, workDir, env)
}
