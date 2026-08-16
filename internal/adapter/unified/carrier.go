package unified

import (
	"context"
	"fmt"
	"sync"
)

// TransportKind enumerates the four-tier carrier transport chain, mirroring
// the four-tier fallback order: bg_daemon → interactive_pty → print_sdk → api_key.
// Each tier is a different way to launch/connect the same underlying CLI; if one
// is degraded (per CarrierHealth) or unavailable, the registry falls back.
type TransportKind string

const (
	// TransportBgDaemon: warm persistent process pool (R2, ACP-style). Fastest
	// (no cold start) but carries lease/zombie/MCP-rebuild cost.
	TransportBgDaemon TransportKind = "bg_daemon"
	// TransportInteractivePTY: pseudo-terminal interactive carrier (R3). Reserved
	// for CLIs that require a real TTY (billing identity / claude attach).
	TransportInteractivePTY TransportKind = "interactive_pty"
	// TransportPrintSDK: one-shot CLI subprocess over stdout pipe. This is the
	// current SG behavior and the default safe tier.
	TransportPrintSDK TransportKind = "print_sdk"
	// TransportAPIKey: direct API-key transport as a last-resort fallback.
	TransportAPIKey TransportKind = "api_key"
)

// DefaultTransports is the four-tier fallback order (best → last resort).
var DefaultTransports = []TransportKind{
	TransportBgDaemon,
	TransportInteractivePTY,
	TransportPrintSDK,
	TransportAPIKey,
}

// SpawnSpec is the transport-agnostic description of a CLI invocation. Adapters
// build this from a variant; transports translate it into their mechanism.
type SpawnSpec struct {
	Command    string
	Args       []string
	WorkDir    string
	MCPPath    string
	Env        []string
	SessionID  string
	StdinInput string
	// ResumeSessionID, when set and the transport supports resume (PTY with
	// ResumeSupported), is appended as `--resume <id>` so multi-turn context is
	// preserved via the CLI's own session mechanism (`--resume <id>`).
	ResumeSessionID string
}

// Transport executes a CLI spawn via a specific mechanism. Implementations are
// registered on the Registry; the registry tries them in carrier order and
// falls back on degradation or error.
type Transport interface {
	Kind() TransportKind
	// Spawn launches the CLI described by spec and returns a handle, or an error
	// if this transport cannot serve (caller falls back to the next tier).
	Spawn(ctx context.Context, spec *SpawnSpec) (*SpawnHandle, error)
}

// Carrier binds a provider to an ordered list of transports (its degradation chain).
type Carrier struct {
	ID         string
	Provider   string
	Transports []TransportKind
}

// Registry selects carriers/transports and applies health-based fallback.
type Registry struct {
	mu         sync.RWMutex
	carriers   map[string]*Carrier
	transports map[TransportKind]Transport
	health     CarrierHealth
	broadcaster HealthBroadcaster
}

// NewRegistry constructs a carrier registry. health may be nil (no fallback
// skipping); pass a CarrierHealth to enable R6-driven degradation avoidance.
func NewRegistry(health CarrierHealth) *Registry {
	return &Registry{
		carriers:   make(map[string]*Carrier),
		transports: make(map[TransportKind]Transport),
		health:     health,
		broadcaster: NoopBroadcaster{},
	}
}

// SetBroadcaster installs a HealthBroadcaster so that transport-tier skips
// during fallback are surfaced to clients (T25 / R6). The default is a no-op.
func (r *Registry) SetBroadcaster(b HealthBroadcaster) {
	if b == nil {
		return
	}
	r.mu.Lock()
	r.broadcaster = b
	r.mu.Unlock()
}

// RegisterTransport adds a transport implementation for a tier.
func (r *Registry) RegisterTransport(t Transport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transports[t.Kind()] = t
}

// RegisterCarrier adds a carrier with its degradation chain.
func (r *Registry) RegisterCarrier(c *Carrier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.carriers[c.ID] = c
}

// ResolveChain returns the ordered transport tiers for a carrier, or the
// default four-tier chain when the carrier is unregistered or has no explicit
// chain. Used by mid-stream fallback (R1) to iterate tiers and to avoid
// re-trying an already-failed tier.
func (r *Registry) ResolveChain(carrierID string) []TransportKind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if c, ok := r.carriers[carrierID]; ok && len(c.Transports) > 0 {
		chain := make([]TransportKind, len(c.Transports))
		copy(chain, c.Transports)
		return chain
	}
	chain := make([]TransportKind, len(DefaultTransports))
	copy(chain, DefaultTransports)
	return chain
}

// Execute runs spec through the carrier's transport chain from the first tier,
// skipping degraded tiers (per health) and falling back to the next available
// transport. Returns the first successful handle, or the last error if every
// tier fails. It is a thin wrapper over ExecuteFrom(0).
func (r *Registry) Execute(ctx context.Context, carrierID string, spec *SpawnSpec) (*SpawnHandle, error) {
	return r.ExecuteFrom(ctx, carrierID, spec, 0)
}

// ExecuteTier attempts EXACTLY ONE transport tier (chain[idx]) — it does not
// cascade to the next tier on spawn failure. This is what R1 mid-stream
// fallback (FallbackCarrierWrapper) needs: it wants to observe a single tier's
// failure and decide whether to perform a single fallback hop itself, rather
// than letting the registry transparently cascade (which would hide the
// failure and defeat the "one retry within this invocation" contract).
func (r *Registry) ExecuteTier(ctx context.Context, carrierID string, spec *SpawnSpec, idx int) (*SpawnHandle, error) {
	chain := r.ResolveChain(carrierID)
	if idx < 0 || idx >= len(chain) {
		return nil, fmt.Errorf("no transport available for carrier %q at index %d", carrierID, idx)
	}
	kind := chain[idx]
	if r.health != nil {
		if lvl := r.health.Level(ctx, carrierID+"/"+string(kind)); lvl != "online" {
			return nil, fmt.Errorf("transport %s degraded (%s)", kind, lvl)
		}
	}
	r.mu.RLock()
	t, ok := r.transports[kind]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("transport %s not available", kind)
	}
	return t.Spawn(ctx, spec)
}

// nextHealthyTierIndex returns the first chain index strictly after fromIdx
// whose tier is registered and (when health is configured) currently online.
// api_key is never returned as a fallback target — it is a no-op last resort
// (a fallback to api_key would emit a false carrier_fallback event).
// Returns ok=false when no further usable tier exists.
func (r *Registry) nextHealthyTierIndex(ctx context.Context, carrierID string, fromIdx int) (int, bool) {
	chain := r.ResolveChain(carrierID)
	for j := fromIdx + 1; j < len(chain); j++ {
		kind := chain[j]
		if kind == TransportAPIKey {
			return 0, false
		}
		if r.health != nil && r.health.Level(ctx, carrierID+"/"+string(kind)) != "online" {
			continue
		}
		r.mu.RLock()
		_, ok := r.transports[kind]
		r.mu.RUnlock()
		if ok {
			return j, true
		}
	}
	return 0, false
}

// ExecuteFrom runs spec through the carrier's transport chain starting at
// startIndex, skipping already-attempted (and degraded) tiers. This is what
// mid-stream fallback (R1 / FallbackCarrierWrapper) calls after a tier has
// started streaming but emitted a fatal error: it retries the same prompt on
// the *next* transport without re-trying the one that already failed.
func (r *Registry) ExecuteFrom(ctx context.Context, carrierID string, spec *SpawnSpec, startIndex int) (*SpawnHandle, error) {
	chain := r.ResolveChain(carrierID)
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex >= len(chain) {
		return nil, fmt.Errorf("no transports available for carrier %q beyond index %d", carrierID, startIndex)
	}

	var lastErr error
	for i := startIndex; i < len(chain); i++ {
		kind := chain[i]
		if r.health != nil {
			healthKey := carrierID + "/" + string(kind)
			if lvl := r.health.Level(ctx, healthKey); lvl != "online" {
				r.mu.RLock()
				b := r.broadcaster
				r.mu.RUnlock()
				if b != nil {
					info := r.health.Info(ctx, healthKey)
					b.BroadcastCarrierHealth(ctx, CarrierHealthEvent{
						Carrier:     carrierID,
						Transport:   string(kind),
						Level:       lvl,
						Reason:      string(info.Reason),
						RemainingMs: info.Remaining.Milliseconds(),
					})
				}
				lastErr = fmt.Errorf("transport %s degraded (%s)", kind, lvl)
				continue
			}
		}
		r.mu.RLock()
		t, ok := r.transports[kind]
		r.mu.RUnlock()
		if !ok {
			lastErr = fmt.Errorf("transport %s not available", kind)
			continue
		}
		h, err := t.Spawn(ctx, spec)
		if err == nil {
			return h, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no transports available for carrier %q", carrierID)
	}
	return nil, lastErr
}

// ProcessTransport adapts the existing one-shot ProcessManager to the Transport
// interface (TransportPrintSDK tier). It preserves SG's current behavior.
type ProcessTransport struct {
	pm *ProcessManager
}

// NewProcessTransport wraps a ProcessManager as the print_sdk transport.
func NewProcessTransport(pm *ProcessManager) *ProcessTransport {
	return &ProcessTransport{pm: pm}
}

// Kind implements Transport.
func (t *ProcessTransport) Kind() TransportKind { return TransportPrintSDK }

// Spawn implements Transport by delegating to ProcessManager.Spawn.
func (t *ProcessTransport) Spawn(ctx context.Context, spec *SpawnSpec) (*SpawnHandle, error) {
	return t.pm.Spawn(ctx, spec.Command, spec.Args, spec.StdinInput)
}
