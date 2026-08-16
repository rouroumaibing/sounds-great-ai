package unified

import (
	"context"
	"fmt"

	"sounds-great-ai/internal/adapter/pool"
)

// WarmRunner drives one user turn through a warm (persistent) CLI process.
// Adapters implement this per-CLI because framing differs (claude stream-json
// keepalive, codex repl, ...). BgDaemonTransport calls RunTurn instead of
// spawning a fresh one-shot process, enabling warm reuse (R2 / ADR-002).
//
// RunTurn must return a *SpawnHandle whose stdout carries this turn's NDJSON
// stream and whose Wait() returns when the turn (not the process) is done, so
// BgDaemonTransport can release the warm process back to the pool.
type WarmRunner interface {
	RunTurn(ctx context.Context, wp *pool.WarmProcess, spec *SpawnSpec) (*SpawnHandle, error)
}

// BgDaemonTransport is the R2 transport tier: reuse a warm persistent process
// from the pool instead of a cold one-shot spawn. On failure it records a
// health degradation (so the registry falls back to the next tier) and returns
// an error. The transport is only registered when a warm pool + runner are
// configured; otherwise the default carrier chain stays one-shot (print_sdk),
// preserving pre-R2 behavior exactly.
//
// To support per-provider long sessions (claude/codex/gemini
// each with their own warm process pool), the transport holds a map of pools
// keyed by provider (the carrier/provider id, e.g. "claude"). The legacy
// single-pool constructor populates t.pool for the original claude-only path;
// the multi-pool constructor populates t.pools. Spawn selects the pool for the
// current provider and falls back to the legacy single pool when no per-provider
// pool is registered.
type BgDaemonTransport struct {
	pool   *pool.WarmPool
	pools  map[string]*pool.WarmPool
	runner WarmRunner
	health CarrierHealth
}

// NewBgDaemonTransport builds the bg_daemon (warm-pool) transport for a single
// (legacy claude-only) pool.
func NewBgDaemonTransport(p *pool.WarmPool, r WarmRunner, h CarrierHealth) *BgDaemonTransport {
	return &BgDaemonTransport{pool: p, runner: r, health: h}
}

// NewBgDaemonTransportMulti builds the bg_daemon transport backed by a map of
// per-provider warm pools (claude/codex/gemini long-session tiers). The runner
// is provider-agnostic (PtyRunner drives any warm PTY process).
func NewBgDaemonTransportMulti(pools map[string]*pool.WarmPool, r WarmRunner, h CarrierHealth) *BgDaemonTransport {
	return &BgDaemonTransport{pools: pools, runner: r, health: h}
}

// Kind implements Transport.
func (t *BgDaemonTransport) Kind() TransportKind { return TransportBgDaemon }

// selectPool returns the warm pool for the current provider, preferring the
// per-provider map and falling back to the legacy single pool.
func (t *BgDaemonTransport) selectPool(provider string) *pool.WarmPool {
	if t.pools != nil {
		if wp, ok := t.pools[provider]; ok && wp != nil {
			return wp
		}
	}
	return t.pool
}

// Spawn acquires a warm process, runs a turn through it, and releases the
// process back to the pool when the turn completes.
func (t *BgDaemonTransport) Spawn(ctx context.Context, spec *SpawnSpec) (*SpawnHandle, error) {
	if t.runner == nil {
		return nil, fmt.Errorf("bg_daemon transport not configured")
	}
	carrierKey := spec.SessionID // also the health key, e.g. "claude"
	wp0 := t.selectPool(carrierKey)
	if wp0 == nil {
		return nil, fmt.Errorf("bg_daemon transport not configured for provider %q", carrierKey)
	}
	key := pool.PoolKey{ProjectPath: spec.WorkDir, ProviderProfile: spec.SessionID}
	wp, err := wp0.Acquire(key, spec.SessionID)
	if err != nil {
		if t.health != nil {
			t.health.RecordFailure(ctx, carrierKey, ReasonSpawnFailed)
		}
		return nil, err
	}
	h, err := t.runner.RunTurn(ctx, wp, spec)
	if err != nil {
		wp0.Release(wp)
		if t.health != nil {
			t.health.RecordFailure(ctx, carrierKey, ReasonSpawnFailed)
		}
		return nil, err
	}
	t.releaseOnExit(wp, h)
	return h, nil
}

// releaseOnExit returns the warm process to the pool once the turn's handle
// signals completion. Crucially it does NOT kill the underlying CLI process —
// keeping it alive is what makes warm reuse possible.
func (t *BgDaemonTransport) releaseOnExit(wp *pool.WarmProcess, h *SpawnHandle) {
	if h == nil {
		t.pool.Release(wp)
		return
	}
	go func() {
		h.Wait()
		t.pool.Release(wp)
	}()
}
