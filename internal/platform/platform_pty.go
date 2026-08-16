//go:build pty

package platform

import (
	"sounds-great-ai/internal/adapter/pool"
	"sounds-great-ai/internal/adapter/unified"
)

// longSessionProviders are the CLIs that support long (warm) sessions.
// Each gets its own warm process pool + PtyRunner under -tags pty. opencode and
// kimi are intentionally excluded; they stay one-shot.
var longSessionProviders = []string{"claude", "codex", "gemini"}

// WireWarmPools wires the R2 warm-pool (bg_daemon) transport for the three
// long-session providers (claude/codex/gemini), using the PTY-backed persistent
// process + PtyRunner. This is the "per-provider long session" the user
// requested: each provider's carrier chain (set in New) leads with bg_daemon,
// and here we supply the per-provider warm pools + runner that make those tiers
// live.
//
// Compiled only under -tags pty because the PTY driver (github.com/creack/pty)
// and the CLIs' TTY semantics are required. Under the default build,
// WireWarmPools is a no-op stub and all three CLIs gracefully fall back to
// one-shot print_sdk — zero new dependency.
//
// Best-effort note: real warm reuse is not validated in this repo. If a warm
// spawn fails (binary missing, TTY unavailable), the BgDaemonTransport records
// a health failure and the carrier chain automatically falls back to print_sdk
// for that invocation. The persistent process's cwd is the platform WorkspaceDir
// (the warm pool's spawn func does not yet vary cwd by per-turn ProjectPath — a
// known infra limitation, not a regression of one-shot behavior).
func (p *Platform) WireWarmPools() {
	if p.CarrierRegistry == nil || p.WorkspaceDir == "" {
		return
	}
	cfg := pool.DefaultWarmPoolConfig()
	runner := unified.NewPtyRunner(unified.PtyConfig{ResumeSupported: true})
	pools := make(map[string]*pool.WarmPool, len(longSessionProviders))
	for _, provider := range longSessionProviders {
		args := longSessionSpawnArgs(provider, p.WorkspaceDir)
		spawn := unified.PtyWarmSpawnFunc(provider, args, p.WorkspaceDir, unified.PtyConfig{ResumeSupported: true})
		pools[provider] = pool.NewWarmPool(cfg, spawn)
	}
	p.RegisterWarmPoolForProviders(longSessionProviders, pools, runner)
}

// longSessionSpawnArgs returns the base CLI args for a provider's warm process
// (model/system prompt are per-turn and injected via RunTurn's stdin). The
// persistent process keeps its TTY open across turns for warm reuse.
func longSessionSpawnArgs(provider, workDir string) []string {
	wd := []string{"--cwd", workDir}
	switch provider {
	case "claude":
		return append([]string{"--output-format", "stream-json"}, wd...)
	case "codex":
		return append([]string{"exec", "--json"}, wd...)
	case "gemini":
		return append([]string{"--output-format", "stream-json"}, wd...)
	default:
		return wd
	}
}
