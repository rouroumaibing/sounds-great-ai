//go:build !windows

// Command sg-cli-supervisor is the R8 orphan-cleanup sidecar. It sits between the SG
// server (the "parent") and a long-running CLI child, spawns the child in its
// own process group, forwards stdio, and — crucially — terminates the child's
// ENTIRE process group if the original parent disappears (macOS/Linux do not
// reap detached children when the parent is SIGKILLed). This survives even a
// hard SIGKILL of the SG server, where an in-process goroutine cannot run.
//
// Usage (invoked by internal/supervisor in Sidecar mode):
//
//	sg-cli-supervisor -- <command> [args...]
//
// with env SG_SUPERVISOR_PARENT_PID / SG_SUPERVISOR_POLL_MS /
// SG_SUPERVISOR_KILL_GRACE_MS supplied by the caller.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	envParentPID = "SG_SUPERVISOR_PARENT_PID"
	envPollMs    = "SG_SUPERVISOR_POLL_MS"
	envGraceMs   = "SG_SUPERVISOR_KILL_GRACE_MS"
	// envChildPIDFile, when set, makes the sidecar write its child's PID to the
	// given file immediately after spawn. The caller (SG ProcessManager) reads
	// it to probe the real CLI for liveness instead of the mostly-idle sidecar.
	envChildPIDFile = "SG_SUPERVISOR_CHILD_PID_FILE"

	defaultPollMs    = 1000
	defaultKillGrace = 3000
)

func atoiDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// parentGone reports whether the original parent PID has vanished: either its
// pid changed (reparented to init) or kill(pid,0) fails (ESRCH).
func parentGone(parentPID int) bool {
	if parentPID <= 0 {
		return false
	}
	if os.Getppid() != parentPID {
		return true
	}
	if err := syscall.Kill(parentPID, 0); err != nil {
		return true
	}
	return false
}

func main() {
	sep := -1
	for i, a := range os.Args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep < 0 || sep+1 >= len(os.Args) {
		fmt.Fprintln(os.Stderr, "sg-cli-supervisor: missing command after --")
		os.Exit(64)
	}
	command := os.Args[sep+1]
	args := os.Args[sep+2:]

	parentPID := atoiDefault(os.Getenv(envParentPID), os.Getppid())
	pollMs := atoiDefault(os.Getenv(envPollMs), defaultPollMs)
	killGraceMs := atoiDefault(os.Getenv(envGraceMs), defaultKillGrace)

	child := exec.Command(command, args...)
	// Detach into its own process group so we can kill -pgid later.
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := child.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sg-cli-supervisor: stdin pipe: %v\n", err)
		os.Exit(1)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sg-cli-supervisor: stdout pipe: %v\n", err)
		os.Exit(1)
	}
	stderr, err := child.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sg-cli-supervisor: stderr pipe: %v\n", err)
		os.Exit(1)
	}
	if err := child.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "sg-cli-supervisor: spawn failed: %v\n", err)
		if ee, ok := err.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
			os.Exit(127)
		}
		os.Exit(1)
	}
	pgid := child.Process.Pid

	// Expose the real child PID to the caller for accurate liveness probing.
	if pidFile := os.Getenv(envChildPIDFile); pidFile != "" {
		_ = os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0o644)
		defer os.Remove(pidFile)
	}

	// Forward stdio: supervisor stdin (the prompt from the parent) → child
	// stdin; child stdout/stderr → supervisor stdout/stderr.
	go func() { _, _ = io.Copy(stdin, os.Stdin) }()
	go func() { _, _ = io.Copy(os.Stdout, stdout) }()
	go func() { _, _ = io.Copy(os.Stderr, stderr) }()

	var (
		mu          sync.Mutex
		terminating bool
		childExited bool
	)
	signalChild := func(sig os.Signal) {
		s, ok := sig.(syscall.Signal)
		if !ok {
			s = syscall.SIGKILL
		}
		_ = syscall.Kill(-pgid, s) // negative pid = process group
	}
	terminateChild := func() {
		mu.Lock()
		if terminating || childExited {
			mu.Unlock()
			return
		}
		terminating = true
		mu.Unlock()
		signalChild(syscall.SIGTERM)
		time.AfterFunc(time.Duration(killGraceMs)*time.Millisecond, func() {
			signalChild(syscall.SIGKILL)
		})
	}
	// Codex treats SIGINT as cooperative cancellation: forward
	// it unchanged without entering the terminate state, so a later SIGTERM can
	// still escalate the same group if the CLI ignores the interrupt.
	interruptChild := func() {
		mu.Lock()
		gone := childExited
		mu.Unlock()
		if !gone {
			signalChild(syscall.SIGINT)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for sig := range sigCh {
			if sig == syscall.SIGINT {
				interruptChild()
			} else {
				terminateChild()
			}
		}
	}()

	// Parent-liveness poll: if the original parent disappears, reap the group.
	go func() {
		ticker := time.NewTicker(time.Duration(pollMs) * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if parentGone(parentPID) {
				terminateChild()
				return
			}
		}
	}()

	// On supervisor exit, force-kill the child group if it is still alive.
	defer signalChild(syscall.SIGKILL)

	_ = child.Wait()
	mu.Lock()
	childExited = true
	mu.Unlock()

	os.Exit(child.ProcessState.ExitCode())
}
