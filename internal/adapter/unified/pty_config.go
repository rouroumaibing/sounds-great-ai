package unified

// PtyConfig tunes the interactive_pty driver (R3 alignment with
// PtyDriver). All fields are opt-in; zero values fall back to safe defaults
// via defaultPtyConfig. The struct is defined in an un-tagged file so both
// pty_real.go (//go:build pty) and pty_stub.go (//go:build !pty) can reference
// it without pulling in github.com/creack/pty under the default build.
type PtyConfig struct {
	// ReadyGraceMs is a fixed grace after pty.Start before the prompt is
	// injected (PtyDriver Note 1: "no screen scraping — grace is
	// sufficient" because the TUI reaches its ❯ prompt within 10-15s). Default
	// 1500ms. Guards against racing the CLI's startup banner.
	ReadyGraceMs int
	// BypassPattern is a regex matched against a bounded read-ahead of the
	// startup banner. If it matches, BypassKeys are sent to accept the CLI's
	// trust/permission screen (PtyDriver lines 155-176: the claude
	// 2.1.170+ "trust this project?" menu → "Down+Enter" → "Yes, I accept").
	// Empty disables bypass detection (the default — most CLIs need none).
	BypassPattern string
	// BypassKeys are the bytes sent when BypassPattern matches. Default
	// "\x1b[B\n" (Down then Enter) for the claude 2.1.170+ consent layout.
	BypassKeys string
	// InterruptKey is sent on cancel() (context cancellation). Default "\x1b"
	// (Escape) — PtyDriver.cancel() = send Escape, leaving the session
	// alive for resume ("[Request interrupted by user]") rather than SIGKILL.
	InterruptKey string
	// ResumeSupported, when true, makes Spawn append `--resume <id>` (using
	// SpawnSpec.ResumeSessionID for one-shot, or PtyConfig.ResumeSessionID for
	// a warm spawn) so multi-turn context is preserved via the CLI's own
	// session mechanism (`--resume <id>`).
	ResumeSupported bool
	// ResumeSessionID is the fixed resume id used by a warm spawn (applied once
	// at process start). Ignored unless ResumeSupported is true.
	ResumeSessionID string
	// TmuxMode, when true, launches claude inside a detached tmux session and
	// reads its output via the transcript-file + Hook side-channel architecture,
	// instead of scraping the PTY master directly. It is only
	// effective under `-tags pty`, only when the command is `claude`, and only
	// when `tmux` is on PATH; otherwise Spawn transparently falls back to the
	// direct pty.Start path. Default false — keep the simpler direct path.
	TmuxMode bool
}

func defaultPtyConfig() PtyConfig {
	return PtyConfig{
		ReadyGraceMs: 1500,
		BypassKeys:   "\x1b[B\n",
		InterruptKey: "\x1b",
	}
}
