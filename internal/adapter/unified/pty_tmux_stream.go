package unified

// pty_tmux_stream.go wires the tmux launch (pty_tmux.go) into PtyTransport and
// merges the transcript + hook side-channels into the SpawnHandle's stdout
// stream. See pty_tmux.go for the architecture rationale.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// tmuxStreamCoordinator fans the two claude side-channels into a single NDJSON
// pipe (consumed downstream by the claude adapter's ParseNDJSON + parseClaudeEvent)
// and decides when the turn is over.
//
// Terminal detection (whichever first):
//   - a Hook `Stop` event (claude >= 2.1.172), or
//   - the tmux session exiting (covers crashed / older claude without hooks), or
//   - the parent context being cancelled.
type tmuxStreamCoordinator struct {
	pw     *io.PipeWriter
	exitCh chan struct{}
	ctx    context.Context
	cancel context.CancelFunc

	handle    *SpawnHandle
	session   string
	hookInfra *HookInfrastructureResult

	once sync.Once

	mu                  sync.Mutex
	finished           bool
	emittedDone        bool
	sessionID          string
	sawTranscriptText  bool
	lastTranscriptText string
	hookFinal          string // Stop hook's last_assistant_message (final answer)
}

// newTmuxStreamCoordinator builds the pipe + handle and returns both so the
// caller (spawnViaTmux) can start the tailer goroutines.
func newTmuxStreamCoordinator(ctx context.Context, session string, hookInfra *HookInfrastructureResult, panePID int) (*tmuxStreamCoordinator, *SpawnHandle) {
	cctx, cancel := context.WithCancel(ctx)
	pr, pw := io.Pipe()
	handle := &SpawnHandle{
		Stdout: pr,
		PID:    panePID,
		stderr: &bytes.Buffer{},
		exitCh: make(chan struct{}),
	}
	if panePID > 0 {
		probe := NewLivenessProbe(panePID, ProbePollInterval, ProbeSoftWarnMs, ProbeStallWarnMs)
		// No adapter-level OnStall bridge here; liveness is best-effort and the
		// probe tolerates a nil callback (see probe.go).
		handle.probe = probe
		probe.Start()
	}
	c := &tmuxStreamCoordinator{
		pw:        pw,
		exitCh:    handle.exitCh,
		ctx:       cctx,
		cancel:    cancel,
		handle:    handle,
		session:   session,
		hookInfra: hookInfra,
	}
	return c, handle
}

// writeLine marshals a synthesized claude-style event and writes it as one NDJSON
// line. Writes after finish() are best-effort (the pipe may be closed).
func (c *tmuxStreamCoordinator) writeLine(obj map[string]any) {
	b, err := json.Marshal(obj)
	if err != nil {
		return
	}
	_, _ = c.pw.Write(append(b, '\n'))
}

func (c *tmuxStreamCoordinator) setSessionID(sid string) {
	c.mu.Lock()
	if c.sessionID == "" && sid != "" {
		c.sessionID = sid
	}
	c.mu.Unlock()
}

// writeTranscriptEvent emits a transcript-derived event in claude adapter's
// expected shape (assistant_response / tool_use / tool_result).
func (c *tmuxStreamCoordinator) writeTranscriptEvent(e transcriptEvent) {
	switch e.OutType {
	case "text":
		c.mu.Lock()
		c.sawTranscriptText = true
		c.lastTranscriptText = e.Content
		c.mu.Unlock()
		c.writeLine(map[string]any{"type": "assistant_response", "content": e.Content})
	case "tool_call":
		m := map[string]any{}
		for k, v := range e.Meta {
			m[k] = v
		}
		m["type"] = "tool_use"
		c.writeLine(m)
	case "tool_result":
		c.writeLine(map[string]any{"type": "tool_result", "message": e.Meta})
	}
}

// onHookStop handles the terminal Hook event. It records the final answer from
// the hook's last_assistant_message and schedules finish() after a short grace
// (see below). The decision of whether to EMIT that final text is deferred to
// finish(), by which point the transcript tailer has caught up; if the
// transcript already delivered text we skip the hook's copy to avoid a
// duplicate final message.
func (c *tmuxStreamCoordinator) onHookStop(e HookEvent) {
	c.mu.Lock()
	c.hookFinal = strings.TrimSpace(e.LastAssistantMessage)
	c.mu.Unlock()
	time.AfterFunc(400*time.Millisecond, c.finish)
}

// finish terminates the stream exactly once: flushes a terminal `result` event
// (so the downstream adapter ends cleanly even if no Stop was seen), closes the
// pipe, reaps state, restores the original .claude/settings.json, and kills the
// tmux session.
func (c *tmuxStreamCoordinator) finish() {
	c.once.Do(func() {
		c.mu.Lock()
		c.finished = true
		// Recover the final answer from the hook only if the transcript layer
		// delivered no text (avoids a duplicate final message).
		if !c.sawTranscriptText && c.hookFinal != "" {
			c.writeLine(map[string]any{"type": "assistant_response", "content": c.hookFinal})
		}
		if !c.emittedDone {
			c.writeLine(map[string]any{"type": "result", "session_id": c.sessionID})
			c.emittedDone = true
		}
		c.mu.Unlock()

		// Stop tailers + probe promptly.
		c.cancel()
		if c.handle.probe != nil {
			c.handle.probe.Stop()
		}
		_ = c.pw.Close()
		close(c.exitCh)

		// Restore .claude/settings.json and remove the capture script.
		if c.hookInfra != nil {
			c.hookInfra.Cleanup()
		}
		// Reap the detached tmux session (and thus claude) if still alive.
		if c.session != "" {
			_ = killTmuxSession(c.session)
		}
		// Record a clean exit for diagnostics (no stderr captured in tmux mode).
		c.handle.mu.Lock()
		code := 0
		c.handle.exitCode = &code
		c.handle.mu.Unlock()
	})
}

// spawnViaTmux launches claude inside a detached tmux session, sets up the Hook
// side-channel, and returns a SpawnHandle whose stdout is the merged transcript
// + hook stream. On any launch failure it returns an error so the caller can
// fall back to the direct pty path.
func (t *PtyTransport) spawnViaTmux(ctx context.Context, spec *SpawnSpec) (*SpawnHandle, error) {
	if spec == nil || spec.WorkDir == "" {
		return nil, fmt.Errorf("tmux spawn requires a work dir")
	}
	// Sidecar jsonl that the hook capture script appends events to.
	sf, err := os.CreateTemp("", "sg-hook-sidecar-*.jsonl")
	if err != nil {
		return nil, err
	}
	sidecarPath := sf.Name()
	_ = sf.Close()

	hookInfra, err := SetupHookInfrastructure(spec.WorkDir, sidecarPath)
	if err != nil {
		_ = os.Remove(sidecarPath)
		return nil, err
	}

	ts, err := spawnTmuxSession(ctx, spec.WorkDir, spec.Command, spec.Args, sidecarPath)
	if err != nil {
		hookInfra.Cleanup()
		_ = os.Remove(sidecarPath)
		return nil, err
	}

	// The transcript layer is best-effort: if it never appears, the hook layer
	// still drives termination (and recovers the final answer).
	transcriptPath, _ := watchForTranscriptFile(ptyTranscriptDir(spec.WorkDir), transcriptWatchTimeout)

	c, handle := newTmuxStreamCoordinator(ctx, ts.name, hookInfra, ts.panePID)

	// Tailer 1: claude transcript (main text + tool stream).
	if transcriptPath != "" {
		go tailFile(c.ctx, transcriptPath, func(line string) {
			sid, evs := parseTranscriptLine(line)
			if sid != "" {
				c.setSessionID(sid)
			}
			for _, e := range evs {
				c.writeTranscriptEvent(e)
			}
		}, nil)
	}

	// Tailer 2: hook side-channel (terminal Stop + session discovery).
	go tailFile(c.ctx, sidecarPath, func(line string) {
		var e HookEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return
		}
		if e.SessionID != "" {
			c.setSessionID(e.SessionID)
		}
		if e.EventType == "Stop" {
			c.onHookStop(e)
		}
	}, nil)

	// Watcher: end the turn when the tmux session (claude) exits, or on cancel.
	go func() {
		ticker := time.NewTicker(tmuxSessionPoll)
		defer ticker.Stop()
		for {
			select {
			case <-c.ctx.Done():
				c.finish()
				return
			case <-ticker.C:
				if !tmuxSessionAlive(ts.name) {
					c.finish()
					return
				}
			}
		}
	}()

	return handle, nil
}
