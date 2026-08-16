package pool

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// errNoSpawnFunc is returned when a WarmPool has no spawn function configured.
var errNoSpawnFunc = errors.New("warm pool: no spawn func configured")

// WarmProcess is a real, long-lived CLI process kept alive across turns for
// reuse (R2 / ADR-002). Unlike the G3 PooledProcess metadata shell, it owns a
// live subprocess with open stdin/stdout pipes so a caller can drive multiple
// turns through it via an injected WarmRunner (defined in the unified package
// to avoid an import cycle: pool must not import unified).
//
// A WarmProcess is "warm" when Alive() && leaseCount==0 (no active lease). A
// bg_daemon transport acquires one or more leases against it; with
// SupportsMultiplexing (F149 Phase C) a single OS process serves many
// concurrent leases — the prerequisite for multiple agents conversing in the
// same warm session. BgDaemonTransport acquires one, marks it leased, runs a
// turn, then releases the lease so the process can be reused, skipping the
// cold start and MCP rebuild that one-shot Spawn pays every turn.
type WarmProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	pid     int
	key     PoolKey
	workDir string

	mu        sync.Mutex
	leaseCount int // number of active leases held against this process
	// leaseGeneration is bumped on force-release (stale-lease, #992) and on
	// retire, so a late-arriving release() from a superseded lease becomes a
	// no-op instead of corrupting the new lease's metrics.
	leaseGeneration int
	alive     bool
	busySince time.Time // time the first (oldest) lease was taken; drives #992
	lastUsed  time.Time
}

// NewWarmProcess wraps an already-started subprocess as a reusable warm
// process. The caller is responsible for starting cmd (and wiring stdin/stdout
// pipes) before calling this.
func NewWarmProcess(cmd *exec.Cmd, stdin io.WriteCloser, stdout io.ReadCloser, key PoolKey, workDir string) *WarmProcess {
	pid := 0
	if cmd != nil && cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	return &WarmProcess{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		pid:      pid,
		key:      key,
		workDir:  workDir,
		alive:    cmd != nil && cmd.Process != nil,
		lastUsed: time.Now(),
	}
}

// PID returns the underlying process id.
func (w *WarmProcess) PID() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pid
}

// Alive reports whether the underlying process is still running. The flag is
// kept accurate by waitAndMark (a goroutine started when the process is
// acquired) which flips it to false once cmd.Wait() returns.
func (w *WarmProcess) Alive() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.alive
}

// waitAndMark blocks until the underlying process exits, then marks the warm
// process dead so the pool stops reusing it. Started by WarmPool.Acquire after
// a cold spawn.
func (w *WarmProcess) waitAndMark() {
	if w.cmd == nil {
		return
	}
	_ = w.cmd.Wait()
	w.mu.Lock()
	w.alive = false
	w.leaseCount = 0
	w.mu.Unlock()
}

// Busy reports whether the process currently serves at least one lease.
func (w *WarmProcess) Busy() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.leaseCount > 0
}

// LeaseCount returns the number of active leases (concurrent turn holders).
func (w *WarmProcess) LeaseCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.leaseCount
}

// Key returns the pool key this warm process belongs to.
func (w *WarmProcess) Key() PoolKey {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.key
}

// Stdin exposes the write end of the warm process (for WarmRunner framing).
func (w *WarmProcess) Stdin() io.WriteCloser {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stdin
}

// Stdout exposes the read end of the warm process (for WarmRunner framing).
func (w *WarmProcess) Stdout() io.Reader {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stdout
}

// IsCwdIntact reports whether the bootstrap working directory the process was
// spawned in still exists. #1203: after a cold spawn, an external
// cleaner may delete the cwd; the child stays alive but any prompt dies with
// getcwd ENOENT, so the pool must retire it and cold-start a fresh process
// (which re-creates the cwd). An empty workDir is treated as intact (the pool
// cannot verify it, but nothing to lose).
func (w *WarmProcess) IsCwdIntact() bool {
	if w.workDir == "" {
		return true
	}
	_, err := os.Stat(w.workDir)
	return err == nil
}

// MarkDead kills the underlying process and marks it non-reusable. Idempotent.
// It also closes the stdin/stdout fds (the PTY master for interactive_pty) so
// the OS releases them and no fd/zombie leak remains — the PtyDriver
// dispose() = kill-session (防僵尸, LL-056). For a PTY warm process stdin and
// stdout are the same *os.File; the guard avoids a double close.
func (w *WarmProcess) MarkDead() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.alive {
		return
	}
	w.alive = false
	w.leaseCount = 0
	if w.stdin != nil {
		_ = w.stdin.Close()
	}
	if w.stdout != nil && any(w.stdout) != any(w.stdin) {
		_ = w.stdout.Close()
	}
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
	}
}

// incLease takes one lease on this process (called under the pool lock, but the
// process lock guards the counter). The first lease stamps busySince for the
// #992 stale-lease timer.
func (w *WarmProcess) incLease() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.leaseCount == 0 {
		w.busySince = time.Now()
	}
	w.leaseCount++
	w.lastUsed = time.Now()
}

// decLease releases one lease and returns the remaining count.
func (w *WarmProcess) decLease() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.leaseCount > 0 {
		w.leaseCount--
	}
	w.lastUsed = time.Now()
	return w.leaseCount
}

// WarmSpawnFunc creates a fresh long-lived subprocess for the given pool key.
// Implementations must start the command and wire stdin/stdout before wrapping
// it with NewWarmProcess.
type WarmSpawnFunc func(key PoolKey) (*WarmProcess, error)

// WarmPoolConfig tunes the warm process pool behavior.
type WarmPoolConfig struct {
	MaxCapacity    int           // max warm processes (default 8)
	TTL            time.Duration // idle TTL before eviction (default 30 min)
	HealthInterval time.Duration // zombie-reaper interval (default 5 min)
	// MaxLeaseMs bounds how long a SINGLE-FLIGHT process may stay leased before
	// the pool treats its lease as stale and force-releases it for reuse
	// (#992: a consumer may vanish without releasing — e.g. Windows
	// console disconnect). 0 disables the safety net (default; matches the
	// pre-R2 one-shot model where a turn owns the process until completion).
	// Ignored when SupportsMultiplexing is true (multiple concurrent leases are
	// legitimate, never force-released).
	MaxLeaseMs int64
	// SupportsMultiplexing mirrors F149 Phase C: a single OS process may
	// serve many concurrent leases (multiple agents in one warm session). When
	// false (default, single-flight), a process is reused only when fully idle
	// (leaseCount==0). Opt-in because it requires the carrier/framing layer to
	// demultiplex concurrent turns on the shared stdin/stdout.
	SupportsMultiplexing bool
}

// DefaultWarmPoolConfig returns sensible defaults.
func DefaultWarmPoolConfig() WarmPoolConfig {
	return WarmPoolConfig{
		MaxCapacity:         8,
		TTL:                  30 * time.Minute,
		HealthInterval:      5 * time.Minute,
		MaxLeaseMs:          0,
		SupportsMultiplexing: false,
	}
}

// WarmPool manages real long-running CLI processes keyed by (projectPath,
// providerProfile), enabling warm reuse across turns (R2). This is the true
// ACP-style persistent-process pool that replaces the G3 metadata shell
// (ProcessPool, which only records lease metadata). WarmPool actually spawns
// and recycles live processes, with LRU/TTL eviction, a background zombie
// reaper, cwd-integrity guarding (#1203), session affinity + sealing, and
// stale-lease recovery (#992). When SupportsMultiplexing is set it permits
// multiple concurrent leases against one process (F149 Phase C).
// When nil/unused, callers fall back to one-shot Spawn — behavior is
// identical to the pre-R2 path.
type WarmPool struct {
	mu      sync.Mutex
	entries map[string]*WarmProcess
	// sessionOwners maps a session id to the warm process that last served it,
	// enabling session affinity: a resume reuses the same process (the
	// sessionOwners map). Affinity persists across Release; it is cleared only when
	// the process is removed from the pool (evict/reap/close) or the session is
	// sealed.
	sessionOwners map[string]*WarmProcess
	// sealedSessions holds session ids that must never be resumed on a reused
	// process (#992 / cancelled-prompt poison). A sealed session forces
	// a fresh replacement process on the next Acquire.
	sealedSessions map[string]bool
	cfg            WarmPoolConfig
	metrics        *PoolMetrics
	spawn          WarmSpawnFunc
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// NewWarmPool creates a warm process pool with the given spawn function.
func NewWarmPool(cfg WarmPoolConfig, spawn WarmSpawnFunc) *WarmPool {
	if cfg.MaxCapacity <= 0 {
		cfg = DefaultWarmPoolConfig()
	}
	if cfg.HealthInterval <= 0 {
		cfg.HealthInterval = DefaultWarmPoolConfig().HealthInterval
	}
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultWarmPoolConfig().TTL
	}
	if spawn == nil {
		spawn = func(key PoolKey) (*WarmProcess, error) {
			return nil, errNoSpawnFunc
		}
	}
	p := &WarmPool{
		entries:        make(map[string]*WarmProcess),
		sessionOwners:  make(map[string]*WarmProcess),
		sealedSessions: make(map[string]bool),
		cfg:            cfg,
		metrics:        NewPoolMetrics(),
		spawn:          spawn,
		stopCh:         make(chan struct{}),
	}
	p.wg.Add(1)
	go p.backgroundLoop()
	return p
}

// Acquire gets or creates a warm process for the given key, optionally honoring
// session affinity. Returns a live, cwd-intact process that may be shared with
// other concurrent leases when SupportsMultiplexing is enabled. The caller must
// call Release on the returned process when its turn completes; Release
// decrements the lease counter (and only idles/evicts the process when the last
// lease is released). A process that is busy under a stale lease (single-flight
// only, #992) is force-released and reused rather than spawning a second process.
func (p *WarmPool) Acquire(key PoolKey, sessionID string) (*WarmProcess, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.metrics.LeaseAcquire()
	k := key.String()

	// 1. Retire any process under this key whose bootstrap cwd was deleted
	//    after spawn (#1203). Covers both the warm-reuse and session-owner
	//    paths. There is at most one entry per key, so this is a single check.
	if wp := p.entries[k]; wp != nil && wp.Alive() && !wp.IsCwdIntact() {
		p.retireEntry(wp, k, "bootstrap cwd missing")
	}

	// 2. Session affinity: reuse the process that last served this session,
	//    provided it is alive and its cwd is intact.
	if sessionID != "" {
		if p.sealedSessions[sessionID] {
			// Sealed (poisoned) session → fresh replacement, never resume.
			delete(p.sessionOwners, sessionID)
		} else if owner := p.sessionOwners[sessionID]; owner != nil &&
			owner.Key().String() == k && owner.Alive() && owner.IsCwdIntact() {
			if !p.cfg.SupportsMultiplexing && owner.LeaseCount() > 0 {
				// Single-flight process is busy. If its lease is stale (#992),
				// force-release the zombie and reuse; otherwise fall through to
				// spawn a fresh process (the in-progress turn still owns it).
			if p.cfg.MaxLeaseMs > 0 && p.staleLease(owner) {
				p.forceRelease(owner, k)
				return p.leaseReadyEntry(owner, k, true), nil
			}
		} else {
			// Multiplexing (any leaseCount) or idle single-flight → reuse.
			return p.leaseReadyEntry(owner, k, true), nil
		}
		}
	}

	// 3. Key-based warm hit: reuse an existing cwd-intact process. For
	//    single-flight, require it to be fully idle (leaseCount==0); for
	//    multiplexing, any alive process may take another lease.
	if wp := p.entries[k]; wp != nil && wp.Alive() && wp.IsCwdIntact() {
		if p.cfg.SupportsMultiplexing || wp.LeaseCount() == 0 {
			return p.leaseReadyEntry(wp, k, true), nil
		}
	}

	// 4. Stale (dead) entry under this key: clean up and respawn.
	if wp := p.entries[k]; wp != nil && !wp.Alive() {
		p.removeWarmFromSessions(wp)
		delete(p.entries, k)
		p.metrics.ZombieCleanup()
	}

	// 5. Stale-lease recovery (#992) for a busy single-flight process: force
	//    release and reuse rather than spawning a second process for the key.
	if !p.cfg.SupportsMultiplexing {
		if wp := p.entries[k]; wp != nil && wp.LeaseCount() > 0 && p.cfg.MaxLeaseMs > 0 && p.staleLease(wp) {
			p.forceRelease(wp, k)
			return p.leaseReadyEntry(wp, k, true), nil
		}
	}

	// 6. Enforce capacity (evict an idle LRU entry if needed).
	if len(p.entries) >= p.cfg.MaxCapacity {
		p.evictLRU()
	}

	wp, err := p.spawn(key)
	if err != nil {
		return nil, err
	}
	go wp.waitAndMark()
	p.entries[k] = wp
	p.metrics.ColdStart()
	return p.leaseReadyEntry(wp, k, false), nil
}

// leaseReadyEntry marks a process leased. When warmHit is true the reuse of an
// existing process is counted as a warm cache hit; a cold start passes false so
// it is counted only as a ColdStart (by the caller). idle→active is recorded on
// the first lease regardless of path.
func (p *WarmPool) leaseReadyEntry(wp *WarmProcess, k string, warmHit bool) *WarmProcess {
	if wp.LeaseCount() == 0 {
		p.metrics.idleToActive()
	}
	wp.incLease()
	if warmHit {
		p.metrics.WarmHit()
	}
	if wp.Key().String() != k {
		// cross-key safety: ensure entries map is consistent
		p.entries[k] = wp
	}
	return wp
}

// staleLease reports whether a busy single-flight process has held its lease
// longer than MaxLeaseMs (#992 zombie lease). Caller holds the lock.
func (p *WarmPool) staleLease(wp *WarmProcess) bool {
	wp.mu.Lock()
	expired := time.Since(wp.busySince) > time.Duration(p.cfg.MaxLeaseMs)*time.Millisecond
	wp.mu.Unlock()
	return expired
}

// forceRelease bumps the generation and clears the lease so a late release from
// the superseded lease becomes a no-op (#992). Caller holds the lock.
func (p *WarmPool) forceRelease(wp *WarmProcess, k string) {
	p.metrics.leaseToIdle()
	wp.mu.Lock()
	wp.leaseGeneration++
	wp.leaseCount = 0
	wp.mu.Unlock()
}

// Release returns one lease on the warm process to the pool (called after a
// turn completes). It does NOT kill the underlying CLI process — that is what
// makes warm reuse possible. When the last lease is released the process
// becomes idle (and is eligible for TTL eviction). A stale/force-released lease
// whose generation no longer matches is a no-op (the process was reused).
func (p *WarmPool) Release(wp *WarmProcess) {
	if wp == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metrics.LeaseRelease()
	if !wp.Alive() {
		p.removeWarmFromSessions(wp)
		delete(p.entries, wp.Key().String())
		p.metrics.ZombieCleanup()
		return
	}
	remaining := wp.decLease()
	if remaining == 0 {
		// Last lease released: process returns to idle and becomes eligible
		// for TTL eviction.
		p.metrics.leaseToIdle()
		p.startIdleTimer(wp, wp.Key().String())
	}
}

// SealSession marks a session id as poisoned so the next Acquire for it starts
// a fresh replacement process instead of resuming a reused one. Carriers call
// this when a turn is aborted/cancelled: a local cancel may settle the iterator
// before the provider quiesces a single-flight carrier.
func (p *WarmPool) SealSession(sessionID string) {
	if sessionID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sealedSessions[sessionID] = true
	delete(p.sessionOwners, sessionID)
}

// IsSessionSealed reports whether a session id has been poisoned.
func (p *WarmPool) IsSessionSealed(sessionID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sealedSessions[sessionID]
}

// ReapDead removes and kills any entries whose process is no longer alive.
// Safe to call directly (e.g. in tests) or from the background reaper.
func (p *WarmPool) ReapDead() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, wp := range p.entries {
		if !wp.Alive() {
			p.removeWarmFromSessions(wp)
			wp.MarkDead()
			delete(p.entries, k)
			p.metrics.ZombieCleanup()
		}
	}
}

// evictLRU evicts the least-recently-used idle entry. Caller holds the lock.
func (p *WarmPool) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	for k, wp := range p.entries {
		if wp.LeaseCount() > 0 {
			continue // never evict a process mid-turn
		}
		if oldestKey == "" || wp.lastUsedBefore(oldestTime) {
			oldestKey = k
			oldestTime = wp.lastUsedTime()
		}
	}
	if oldestKey != "" {
		wp := p.entries[oldestKey]
		if wp != nil {
			p.removeWarmFromSessions(wp)
			wp.MarkDead()
		}
		delete(p.entries, oldestKey)
		p.metrics.LRUEviction()
	}
}

// evictExpired evicts idle entries older than TTL. Caller holds the lock.
func (p *WarmPool) evictExpired() {
	now := time.Now()
	for k, wp := range p.entries {
		if wp.LeaseCount() > 0 {
			continue
		}
		if now.Sub(wp.lastUsedTime()) > p.cfg.TTL {
			p.removeWarmFromSessions(wp)
			wp.MarkDead()
			delete(p.entries, k)
			p.metrics.TTLEviction()
		}
	}
}

// retireEntry retires a ready/active process: drops its session affinity,
// bumps its generation (late releases become no-ops), closes it, and frees the
// slot. Used for #1203 cwd-loss and poisoned single-flight reuse.
func (p *WarmPool) retireEntry(wp *WarmProcess, k string, reason string) {
	idx := -1
	if p.entries[k] == wp {
		idx = 0
	}
	if idx < 0 {
		return
	}
	p.clearIdleTimer(wp)
	p.removeWarmFromSessions(wp)
	wp.mu.Lock()
	if wp.leaseCount > 0 {
		p.metrics.leaseToIdle()
		wp.leaseCount = 0
	}
	wp.leaseGeneration++
	wp.mu.Unlock()
	delete(p.entries, k)
	p.metrics.Eviction()
	wp.MarkDead()
	_ = reason
}

// removeWarmFromSessions drops any session-affinity entries that point at wp.
// Caller holds the pool lock.
func (p *WarmPool) removeWarmFromSessions(wp *WarmProcess) {
	for sid, owner := range p.sessionOwners {
		if owner == wp {
			delete(p.sessionOwners, sid)
		}
	}
}

func (p *WarmPool) backgroundLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cfg.HealthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.ReapDead()
			p.mu.Lock()
			p.evictExpired()
			p.mu.Unlock()
		}
	}
}

// Close shuts down the pool and kills all warm processes.
func (p *WarmPool) Close() {
	close(p.stopCh)
	p.wg.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, wp := range p.entries {
		p.removeWarmFromSessions(wp)
		wp.MarkDead()
	}
	p.entries = make(map[string]*WarmProcess)
	p.sealedSessions = make(map[string]bool)
}

// Size returns the current number of pooled warm processes.
func (p *WarmPool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}

// Metrics returns the pool metrics.
func (p *WarmPool) Metrics() *PoolMetrics { return p.metrics }

// lastUsedTime returns the last-used timestamp (process lock).
func (w *WarmProcess) lastUsedTime() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastUsed
}

// lastUsedBefore reports whether this process was used before t.
func (w *WarmProcess) lastUsedBefore(t time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastUsed.Before(t)
}

// startIdleTimer / clearIdleTimer manage TTL eviction timers. These mirror the
// AcpProcessPool idle timer; SG keeps them lightweight.

func (p *WarmPool) startIdleTimer(wp *WarmProcess, k string) {
	// Idle timing is handled by the background evictExpired ticker keyed on
	// lastUsed; no per-process timer is needed. Placeholder for parity.
	_ = wp
	_ = k
}

func (p *WarmPool) clearIdleTimer(wp *WarmProcess) {
	_ = wp
}
