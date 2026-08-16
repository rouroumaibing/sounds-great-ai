//go:build !pty

package unified

import (
	"context"
	"fmt"
)

// PtyTransport is the interactive_pty (R3) carrier tier. Under the default
// build (no `pty` tag) it is a no-op placeholder: it reports unavailable so the
// registry falls back to the next tier (print_sdk). The real PTY driver lives
// in pty_real.go behind the `pty` build tag (requires github.com/creack/pty).
//
// R3 is a reserved/opt-in carrier per ADR-002: the default three CLIs
// (claude/codex/gemini) + kimi run one-shot, and PTY is enabled only for CLIs
// that require a real TTY (billing identity / interactive attach).
type PtyTransport struct{}

// NewPtyTransport returns the no-op placeholder PTY transport. The optional
// PtyConfig is accepted for parity with the real (pty-tagged) implementation.
func NewPtyTransport(_ ...PtyConfig) *PtyTransport { return &PtyTransport{} }

// Kind implements Transport.
func (t *PtyTransport) Kind() TransportKind { return TransportInteractivePTY }

// Spawn always errors under the default build, signalling the registry to
// fall back.
func (t *PtyTransport) Spawn(_ context.Context, _ *SpawnSpec) (*SpawnHandle, error) {
	return nil, fmt.Errorf("interactive_pty transport not enabled (build without `pty` tag)")
}
