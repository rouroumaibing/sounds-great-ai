//go:build !pty

package platform

// WireWarmPools is a no-op under the default build: the PTY-backed warm pools
// require the `pty` build tag (github.com/creack/pty). Without it, claude/
// codex/gemini's default carrier chains still lead with bg_daemon, but the
// registry finds no bg_daemon transport and falls back to one-shot print_sdk —
// zero new dependency, behavior identical to pre-R2.
func (p *Platform) WireWarmPools() {}
