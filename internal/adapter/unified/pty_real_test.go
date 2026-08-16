//go:build pty

package unified

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sounds-great-ai/internal/adapter/pool"
)

// writeScript writes a bash script and returns its path. The script reads a
// prompt from stdin (one per turn) and emits NDJSON, terminating each turn with
// a `result` event. If persist is true it loops (stays alive across turns);
// otherwise it exits after the first turn.
func writeScript(t *testing.T, persist bool) string {
	t.Helper()
	var body string
	if persist {
		body = `#!/bin/bash
while read -r prompt; do
  printf '{"type":"text","content":"got:%s"}\n' "$prompt"
  printf '{"type":"result"}\n'
done
`
	} else {
		body = `#!/bin/bash
read -r prompt
printf '{"type":"text","content":"got:%s"}\n' "$prompt"
printf '{"type":"result"}\n'
`
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fakecli.sh")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPtyTransportFeedsPromptAndStreams(t *testing.T) {
	script := writeScript(t, false)
	tr := NewPtyTransport(PtyConfig{}) // no grace: keep the test fast
	handle, err := tr.Spawn(context.Background(), &SpawnSpec{
		Command:    "bash",
		Args:       []string{script},
		StdinInput: "hello-pty",
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	var events []string
	sc := bufio.NewScanner(handle.Stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		events = append(events, line)
		if strings.Contains(line, `"type":"result"`) {
			break
		}
	}
	handle.Wait()

	foundText, foundResult := false, false
	for _, e := range events {
		if strings.Contains(e, `"content":"got:hello-pty"`) {
			foundText = true
		}
		if strings.Contains(e, `"type":"result"`) {
			foundResult = true
		}
	}
	if !foundText {
		t.Errorf("expected text event carrying the prompt, got %v", events)
	}
	if !foundResult {
		t.Errorf("expected a result terminal event, got %v", events)
	}
}

func TestPtyRunnerReusesWarmProcessAcrossTurns(t *testing.T) {
	script := writeScript(t, true)

	cfg := pool.DefaultWarmPoolConfig()
	p := pool.NewWarmPool(cfg, PtyWarmSpawnFunc("bash", []string{script}, "", PtyConfig{}))
	defer p.Close()

	key := pool.PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "fake"}
	runner := PtyRunner{}

	// Turn 1.
	wp1, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	h1, err := runner.RunTurn(context.Background(), wp1, &SpawnSpec{StdinInput: "turn-1"})
	if err != nil {
		t.Fatalf("runTurn1: %v", err)
	}
	assertTurnOutput(t, h1, "turn-1")
	h1.Wait()
	p.Release(wp1)

	// Turn 2 — same warm process should be reused (affinity + idle).
	wp2, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	defer p.Release(wp2)
	if wp2.PID() != wp1.PID() {
		t.Errorf("PtyRunner should reuse the same warm process across turns (pid %d vs %d)", wp2.PID(), wp1.PID())
	}
	h2, err := runner.RunTurn(context.Background(), wp2, &SpawnSpec{StdinInput: "turn-2"})
	if err != nil {
		t.Fatalf("runTurn2: %v", err)
	}
	assertTurnOutput(t, h2, "turn-2")
	h2.Wait()
}

// assertTurnOutput scans a turn's stream for the text event carrying want and
// the result terminal event, with a hard timeout.
func assertTurnOutput(t *testing.T, h *SpawnHandle, want string) {
	t.Helper()
	sc := bufio.NewScanner(h.Stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	got, done := false, false
	deadline := time.After(5 * time.Second)
	for {
		if got && done {
			return
		}
		ch := make(chan struct{})
		go func() {
			if sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line != "" {
					if strings.Contains(line, `"content":"got:`+want+`"`) {
						got = true
					}
					if strings.Contains(line, `"type":"result"`) {
						done = true
					}
				}
			}
			close(ch)
		}()
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for turn output (got=%v done=%v)", got, done)
		case <-ch:
		}
	}
}

// writeCancelScript writes a fake CLI that reads lines (cooked mode, like the
// bypass test) and exits (emitting an "interrupted" event) when it receives the
// sentinel "CANCEL" line. It proves the transport writes its interrupt key on
// context cancel. A real interactive TUI runs in RAW mode and reacts to ESC
// (PtyDriver.cancel() = send Escape); we use a newline-terminated sentinel here
// because the line discipline only delivers newline-terminated input in cooked
// mode, and the test harness cannot reliably force a raw TUI. The transport
// code path (cancel goroutine → write InterruptKey) is identical for any key.
func writeCancelScript(t *testing.T) string {
	t.Helper()
	body := `#!/bin/bash
while IFS= read -r line; do
  if [ "$line" = "CANCEL" ]; then
    printf '{"type":"interrupted"}\n'
    exit 0
  fi
done
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cancelcli.sh")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestPtyTransportCancelWritesInterruptKey proves R3 alignment with
// PtyDriver.cancel() = send the interrupt key: on context cancellation the
// transport must write its InterruptKey so the CLI aborts instead of the OS
// SIGKILLing it. The fake CLI exits on the sentinel "CANCEL" line, so Wait()
// must return promptly. (Production default InterruptKey is ESC for raw-mode
// TUIs; the code path is identical for any key value.)
func TestPtyTransportCancelWritesInterruptKey(t *testing.T) {
	script := writeCancelScript(t)
	tr := NewPtyTransport(PtyConfig{InterruptKey: "CANCEL\n"}) // sentinel key for cooked mode
	ctx, cancel := context.WithCancel(context.Background())
	handle, err := tr.Spawn(ctx, &SpawnSpec{Command: "bash", Args: []string{script}})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	// Let the script attach and block on read before cancelling, so the
	// interrupt key lands on a ready reader. This mirrors a mid-turn cancel,
	// where the CLI is already running and consuming input (a cancel that
	// fires before the CLI is ready would be lost — not a production path).
	time.Sleep(300 * time.Millisecond)
	cancel()
	// Drain the PTY output so the script's writes never block on a full buffer
	// (a classic PTY deadlock); the carrier does this in production.
	go func() { _, _ = io.Copy(io.Discard, handle.Stdout) }()
	done := make(chan struct{})
	go func() { handle.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handle.Wait() did not return after cancel — interrupt key not sent?")
	}
}

// TestPtyTransportBypassConfirmationScreen proves R3 alignment with
// PtyDriver lines 155-176: when the startup banner matches BypassPattern, the
// transport auto-accepts the consent screen by sending BypassKeys. The fake CLI
// prints a trust prompt, waits for an answer, and exits with `result` only if
// the answer is the bypass key ("accept").
func TestPtyTransportBypassConfirmationScreen(t *testing.T) {
	body := `#!/bin/bash
echo "Do you trust this project? 1. No  2. Yes, I accept"
read -r answer
if [ "$answer" = "accept" ]; then
  printf '{"type":"result"}\n'
  exit 0
fi
printf '{"type":"denied"}\n'
`
	dir := t.TempDir()
	script := filepath.Join(dir, "trustcli.sh")
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	// BypassPattern matches the trust prompt; BypassKeys = "accept\n".
	tr := NewPtyTransport(PtyConfig{
		ReadyGraceMs:  200,
		BypassPattern: "Do you trust this project",
		BypassKeys:    "accept\n",
	})
	handle, err := tr.Spawn(context.Background(), &SpawnSpec{Command: "bash", Args: []string{script}})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	var gotResult bool
	sc := bufio.NewScanner(handle.Stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	deadline := time.After(5 * time.Second)
	gotCh := make(chan string)
	go func() {
		for sc.Scan() {
			gotCh <- strings.TrimSpace(sc.Text())
		}
		close(gotCh)
	}()
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out; gotResult=%v", gotResult)
		case line, ok := <-gotCh:
			if !ok {
				goto done
			}
			if line == "" {
				continue
			}
			if strings.Contains(line, `"type":"result"`) {
				gotResult = true
			}
			if strings.Contains(line, `"type":"denied"`) {
				t.Fatalf("consent screen was NOT bypassed (got 'denied'): %s", line)
			}
		}
	}
done:
	handle.Wait()
	if !gotResult {
		t.Errorf("expected consent screen to be bypassed (result event), but it was not")
	}
}
