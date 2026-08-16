//go:build pty

package unified

// pty_tmux.go implements the "tmux-transcript + Hook side-channel"
// architecture (F230 PtyDriver, faithful to the actual code rather than
// guessed). Per the verified upstream design:
//
//   - claude is launched inside a DETACHED tmux session (a real TTY), not a
//     bare pty.Start. The PTY master is owned by tmux, so we do NOT scrape the
//     TUI's raw NDJSON; instead we read claude's own side channels.
//   - Layer 1 — transcript: claude writes `~/.claude/projects/<slug>/*.jsonl`
//     (slug = cwd with '/' replaced by '-'). We watch that dir for the newest
//     session file and tail it, turning `assistant`/`tool` messages into SG
//     StreamEvents. This is the main message stream.
//   - Layer 2 — Hook side-channel (claude >= 2.1.172): a scoped
//     `.claude/settings.json` (see pty_hook.go) registers Stop/PostToolUse
//     hooks whose capture script appends events to $SG_HOOK_SIDECAR. The Stop
//     event is the TERMINAL signal (carries the final assistant message) and
//     also yields the session_id; PostToolUse surfaces tool visibility.
//
// The two layers are merged: transcript = text + tool events; hook Stop =
// terminal (final text + done). If the transcript never appears (older claude
// or a launch quirk) the hook still delivers the final answer + done, so the
// turn always terminates.
//
// This file is compiled ONLY under `-tags pty`. The default build (no pty tag)
// keeps the one-shot print_sdk behavior and never references tmux. When tmux
// is absent, PtyTransport.Spawn transparently falls back to the direct
// pty.Start path (see pty_real.go).

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// tmuxSessionPrefix namespaces the detached sessions we create so they are
	// easy to spot / clean up.
	tmuxSessionPrefix = "sg-"

	// transcriptWatchTimeout bounds how long we wait for claude to create its
	// session jsonl before giving up on the transcript layer (the hook layer
	// still drives termination).
	transcriptWatchTimeout = 8 * time.Second

	// tmuxSessionPoll is how often we check whether the tmux session (and thus
	// claude) has exited.
	tmuxSessionPoll = 500 * time.Millisecond
)

// tmuxAvailable reports whether the `tmux` binary is on PATH.
func tmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// transcriptSlug mirrors claude's project directory naming: the absolute cwd
// with every path separator replaced by '-'. e.g. "/home/u/proj" -> "-home-u-proj".
func transcriptSlug(cwd string) string {
	return strings.ReplaceAll(cwd, "/", "-")
}

// ptyTranscriptDir returns the claude projects directory that holds the session
// jsonl files for cwd: `~/.claude/projects/<slug>`.
func ptyTranscriptDir(cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".claude", "projects", transcriptSlug(cwd))
}

// shellQuote single-quotes a string for safe inclusion in a shell command,
// escaping embedded single quotes the POSIX way (' -> '\'').
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// buildClaudeTmuxCommand assembles the exact command sent to the tmux pane.
// It strips the non-interactive `--output-format stream-json` flag (claude runs
// interactively inside tmux and we read its side channels, not stdout), keeps
// model/cwd/mcp-config/append-system-prompt, and passes the prompt (stdin in
// the one-shot path) as a single-quoted positional argument. The hook sidecar
// path is injected inline as an environment assignment so claude's hook
// subprocess inherits $SG_HOOK_SIDECAR.
func buildClaudeTmuxCommand(spec *SpawnSpec, sidecar string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("nil spec")
	}
	var b strings.Builder
	b.WriteString(EnvHookSidecar)
	b.WriteString("=")
	b.WriteString(shellQuote(sidecar))
	b.WriteString(" ")
	b.WriteString(shellQuote(spec.Command))

	args := spec.Args
	for i := 0; i < len(args); i++ {
		a := args[i]
		// Drop the output-format flag and its value: we want interactive claude.
		if a == "--output-format" {
			i++ // skip the following value too
			continue
		}
		if a == "--output-format=stream-json" {
			continue
		}
		b.WriteString(" ")
		b.WriteString(shellQuote(a))
	}
	if spec.StdinInput != "" {
		b.WriteString(" ")
		b.WriteString(shellQuote(spec.StdinInput))
	}
	return b.String(), nil
}

// tmuxSession describes a detached tmux session we launched.
type tmuxSession struct {
	name    string
	panePID int // pid of the pane's process (claude) for liveness probing
}

// spawnTmuxSession creates a detached tmux session rooted at cwd and sends the
// (already-built) command into it. It returns the session handle. On any
// failure the partially-created session is killed before returning the error.
func spawnTmuxSession(ctx context.Context, cwd, command string, args []string, sidecar string) (*tmuxSession, error) {
	if !tmuxAvailable() {
		return nil, fmt.Errorf("tmux not available on PATH")
	}
	name := tmuxSessionPrefix + randHex(8)

	newArgs := []string{"new-session", "-d", "-s", name, "-c", cwd}
	if err := exec.CommandContext(ctx, "tmux", newArgs...).Run(); err != nil {
		return nil, fmt.Errorf("tmux new-session: %w", err)
	}
	// Best-effort: also export the sidecar into the session env so shells
	// spawned later inherit it (the inline assignment above already covers the
	// claude invocation itself).
	_ = exec.CommandContext(ctx, "tmux", "set-environment", "-t", name, EnvHookSidecar, sidecar).Run()

	panePID := tmuxPanePID(ctx, name)

	cmd, err := buildClaudeTmuxCommand(&SpawnSpec{Command: command, Args: args}, sidecar)
	if err != nil {
		_ = killTmuxSession(name)
		return nil, err
	}
	send := exec.CommandContext(ctx, "tmux", "send-keys", "-t", name, cmd, "Enter")
	if err := send.Run(); err != nil {
		_ = killTmuxSession(name)
		return nil, fmt.Errorf("tmux send-keys: %w", err)
	}
	return &tmuxSession{name: name, panePID: panePID}, nil
}

// tmuxPanePID returns the pid of the shell/claude running in the session's
// single pane, or 0 if it cannot be determined.
func tmuxPanePID(ctx context.Context, name string) int {
	out, err := exec.CommandContext(ctx, "tmux", "list-panes", "-t", name, "-F", "#{pane_pid}").Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if n, err := parseIntStrict(line); err == nil {
				return n
			}
		}
	}
	return 0
}

// tmuxSessionAlive reports whether the detached session still exists.
func tmuxSessionAlive(name string) bool {
	err := exec.Command("tmux", "has-session", "-t", name).Run()
	return err == nil
}

// killTmuxSession forcibly removes a detached session (best-effort cleanup).
func killTmuxSession(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

// watchForTranscriptFile polls the claude projects dir for the newest non-empty
// *.jsonl session file, returning its path once found or an error on timeout.
// It skips transient / title-only files by requiring a minimum size.
func watchForTranscriptFile(dir string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p, ok := newestTranscriptFile(dir); ok {
			return p, nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return "", fmt.Errorf("transcript file not found under %s within %s", dir, timeout)
}

// newestTranscriptFile returns the most recently modified *.jsonl in dir that
// is at least 1 byte, or ("", false) if none qualifies.
func newestTranscriptFile(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	var best string
	var bestMod time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 {
			continue
		}
		if info.ModTime().After(bestMod) {
			bestMod = info.ModTime()
			best = filepath.Join(dir, e.Name())
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

// --- transcript parsing ----------------------------------------------------

// transcriptEvent is one parsed claude transcript line, normalized for SG.
type transcriptEvent struct {
	OutType string         // "text" | "tool_call" | "tool_result"
	Content string         // text payload (for text events)
	Meta    map[string]any // raw fields for tool events / session discovery
}

// parseTranscriptLine converts a single claude transcript jsonl line into zero
// or more SG-normalized events. It understands the `assistant` (text + tool_use
// blocks) and `tool` (tool_result) message shapes and extracts sessionId for
// session discovery. Unrecognized line shapes yield no events.
func parseTranscriptLine(line string) (sessionID string, events []transcriptEvent) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return "", nil
	}
	if sid, ok := obj["sessionId"].(string); ok {
		sessionID = sid
	}
	switch obj["type"] {
	case "assistant":
		msg, _ := obj["message"].(map[string]any)
		content, _ := msg["content"].([]any)
		for _, blk := range content {
			b, _ := blk.(map[string]any)
			if b == nil {
				continue
			}
			switch b["type"] {
			case "text":
				if t, _ := b["text"].(string); t != "" {
					events = append(events, transcriptEvent{OutType: "text", Content: t})
				}
			case "tool_use":
				events = append(events, transcriptEvent{
					OutType: "tool_call",
					Meta:    b,
				})
			}
		}
	case "tool":
		msg, _ := obj["message"].(map[string]any)
		events = append(events, transcriptEvent{OutType: "tool_result", Meta: msg})
	}
	return sessionID, events
}

// tailFile reads an existing file from the beginning and then polls for
// appended lines, invoking onLine for each complete line until stop is closed
// or the file errors. It is the shared engine behind both the transcript and
// hook-sidecar tailers.
func tailFile(ctx context.Context, path string, onLine func(string), onEOF func()) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			onLine(strings.TrimRight(line, "\n"))
		}
		if err != nil {
			if err == io.EOF {
				if onEOF != nil {
					onEOF()
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(300 * time.Millisecond):
					continue
				}
			}
			return
		}
	}
}

// randHex returns a short lowercase hex string for unique session names.
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		// Fall back to a time-based token; uniqueness is best-effort.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	const hex = "0123456789abcdef"
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = hex[b[i]&0x0f]
	}
	return string(out)
}

// parseIntStrict parses a base-10 integer, returning an error on any non-numeric
// content (unlike strconv.Atoi which tolerates leading/trailing whitespace).
func parseIntStrict(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}
