//go:build !windows

package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func syscallSIGTERM() os.Signal { return syscall.SIGTERM }
func syscallSIGKILL() os.Signal { return syscall.SIGKILL }

// newCmd builds the child command with its own process group (Setpgid) so the
// whole tree can be reaped with one signal to -pgid. Mirrors the
// detached: true child.
func newCmd(command string, args []string, workDir string, env []string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// killProcessGroup signals the child's process group (negative pid). A signal
// to -pgid reaches every member of the group, including grandchildren — exactly
// the process.kill(-child.pid, signal).
func killProcessGroup(pgid int, sig os.Signal) error {
	if pgid <= 0 {
		return nil
	}
	s, ok := sig.(syscall.Signal)
	if !ok {
		s = syscall.SIGKILL
	}
	return syscall.Kill(-pgid, s)
}

func pgidOf(cmd *exec.Cmd) int {
	if cmd.Process == nil {
		return 0
	}
	// With Setpgid, the child's pgid equals its own pid.
	return cmd.Process.Pid
}

func spawnInProcess(ctx context.Context, cfg Config, command string, args []string, workDir string, env []string) (*SupervisedProcess, error) {
	cmd := newCmd(command, args, workDir, env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &SupervisedProcess{
		cmd:    cmd,
		pgid:   pgidOf(cmd),
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
	// In-process monitor: reap the group if the supervising context is
	// cancelled (e.g. the SG server is shutting down). Handles graceful
	// shutdown; sidecar mode additionally covers a hard SIGKILL of the parent.
	go func() {
		<-ctx.Done()
		_ = p.Terminate(cfg.KillGrace)
	}()
	return p, nil
}

// resolveSidecar finds the sg-cli-supervisor binary next to the running server
// binary, falling back to a PATH lookup.
func resolveSidecar() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), SidecarBin)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return SidecarBin
}

func spawnSidecar(cfg Config, command string, args []string, workDir string, env []string) (*SupervisedProcess, error) {
	sidecarPath := resolveSidecar()
	fullArgs := append([]string{"--"}, append([]string{command}, args...)...)
	sidecar := exec.Command(sidecarPath, fullArgs...)
	if workDir != "" {
		sidecar.Dir = workDir
	}
	sidecar.Env = append(os.Environ(),
		fmt.Sprintf("%s=%d", EnvParentPID, os.Getpid()),
		fmt.Sprintf("%s=%d", EnvPollMs, int(cfg.PollInterval.Milliseconds())),
		fmt.Sprintf("%s=%d", EnvGraceMs, int(cfg.KillGrace.Milliseconds())),
	)
	if len(env) > 0 {
		sidecar.Env = append(sidecar.Env, env...)
	}
	stdin, err := sidecar.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := sidecar.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := sidecar.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := sidecar.Start(); err != nil {
		return nil, err
	}
	return &SupervisedProcess{
		sidecar: sidecar,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}, nil
}
