package pool

import (
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// spawnCat starts a long-lived `cat` process (stdin/stdout open, stays alive
// until killed) to simulate a warm CLI process.
func spawnCat(key PoolKey) (*WarmProcess, error) {
	return spawnCatIn(key, "")
}

func spawnCatIn(key PoolKey, workDir string) (*WarmProcess, error) {
	cmd := exec.Command("cat")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return NewWarmProcess(cmd, stdin, stdout, key, workDir), nil
}

func newTestWarmPool() *WarmPool {
	return NewWarmPool(DefaultWarmPoolConfig(), spawnCat)
}

func TestWarmPoolColdStart(t *testing.T) {
	p := newTestWarmPool()
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}
	wp, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !wp.Alive() {
		t.Fatal("warm process should be alive")
	}
	if p.Size() != 1 {
		t.Errorf("size = %d, want 1", p.Size())
	}
	snap := p.Metrics().Snapshot(1, 1)
	if snap.ColdStartCount != 1 {
		t.Errorf("cold start = %d, want 1", snap.ColdStartCount)
	}
	p.Release(wp)
}

func TestWarmPoolWarmHitReusesSamePID(t *testing.T) {
	p := newTestWarmPool()
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}
	wp1, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	pid1 := wp1.PID()
	p.Release(wp1)

	wp2, err := p.Acquire(key, "s2")
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	defer p.Release(wp2)
	if wp2.PID() != pid1 {
		t.Errorf("warm hit should reuse same process (pid %d vs %d)", wp2.PID(), pid1)
	}
	snap := p.Metrics().Snapshot(1, 1)
	if snap.WarmHitCount != 1 {
		t.Errorf("warm hit = %d, want 1", snap.WarmHitCount)
	}
	if snap.ColdStartCount != 1 {
		t.Errorf("cold start = %d, want 1", snap.ColdStartCount)
	}
}

func TestWarmPoolDeadProcessReapedOnRelease(t *testing.T) {
	p := newTestWarmPool()
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}
	wp, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	// Kill the underlying process; waitAndMark flips alive=false asynchronously.
	wp.MarkDead()
	// Give waitAndMark a moment to observe exit.
	time.Sleep(50 * time.Millisecond)
	if wp.Alive() {
		t.Fatal("process should be dead after MarkDead")
	}
	p.Release(wp)
	if p.Size() != 0 {
		t.Errorf("dead process should be reaped on release, size = %d", p.Size())
	}
}

func TestWarmPoolReapDead(t *testing.T) {
	p := newTestWarmPool()
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}
	wp, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	wp.MarkDead()
	p.ReapDead()
	if p.Size() != 0 {
		t.Errorf("ReapDead should remove dead entry, size = %d", p.Size())
	}
	snap := p.Metrics().Snapshot(0, 0)
	if snap.ZombieCleanupCount < 1 {
		t.Errorf("expected at least 1 zombie cleanup, got %d", snap.ZombieCleanupCount)
	}
}

func TestWarmPoolConcurrentAcquireRelease(t *testing.T) {
	p := newTestWarmPool()
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wp, err := p.Acquire(key, "s")
			if err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			time.Sleep(time.Millisecond)
			p.Release(wp)
		}()
	}
	wg.Wait()
	// At most one live entry for the same key (reused across goroutines).
	if p.Size() > 1 {
		t.Errorf("expected <=1 pooled entry, got %d", p.Size())
	}
}

func TestWarmPoolNoSpawnFunc(t *testing.T) {
	p := NewWarmPool(DefaultWarmPoolConfig(), nil)
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}
	if _, err := p.Acquire(key, "s"); err == nil {
		t.Fatal("expected error when no spawn func configured")
	}
}

// TestWarmPoolCwdGoneRetiresAndColdStarts (#1203): when the bootstrap cwd of a
// warm process is removed, Acquire must retire it and cold-start a fresh one
// rather than handing back a process whose prompts would die with getcwd ENOENT.
func TestWarmPoolCwdGoneRetiresAndColdStarts(t *testing.T) {
	dir, err := os.MkdirTemp("", "warmcwd")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	p := NewWarmPool(DefaultWarmPoolConfig(), func(k PoolKey) (*WarmProcess, error) {
		return spawnCatIn(k, dir)
	})
	defer p.Close()

	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}
	wp1, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !wp1.IsCwdIntact() {
		t.Fatal("cwd should be intact right after spawn")
	}
	p.Release(wp1)

	// Delete the bootstrap cwd; the warm process is still alive but cwd-less.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if wp1.IsCwdIntact() {
		t.Fatal("IsCwdIntact should report false after cwd removed")
	}

	wp2, err := p.Acquire(key, "s2")
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	defer p.Release(wp2)
	// A different process must have been cold-started (cwd check forces retire).
	if wp2.PID() == wp1.PID() {
		t.Errorf("cwd-gone warm process should not be reused (pid %d)", wp2.PID())
	}
	snap := p.Metrics().Snapshot(1, 1)
	if snap.ColdStartCount != 2 {
		t.Errorf("cold start = %d, want 2 (original + cwd-retire replacement)", snap.ColdStartCount)
	}
}

// TestWarmPoolSessionAffinityReusesSameProcess: re-acquiring with the same
// session id reuses the same warm process (resume semantics), even across
// different callers, provided it is idle and intact.
func TestWarmPoolSessionAffinityReusesSameProcess(t *testing.T) {
	p := newTestWarmPool()
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}

	wp1, err := p.Acquire(key, "session-A")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	pid1 := wp1.PID()
	p.Release(wp1)

	// Different session id → key-based warm hit (still same process).
	wp2, err := p.Acquire(key, "session-B")
	if err != nil {
		t.Fatalf("acquire B: %v", err)
	}
	p.Release(wp2)

	// Same session id as wp1 → affinity reuse of the same process.
	wp3, err := p.Acquire(key, "session-A")
	if err != nil {
		t.Fatalf("acquire A again: %v", err)
	}
	defer p.Release(wp3)
	if wp3.PID() != pid1 {
		t.Errorf("session affinity should reuse the same process (pid %d vs %d)", wp3.PID(), pid1)
	}
	snap := p.Metrics().Snapshot(1, 1)
	if snap.WarmHitCount < 2 {
		t.Errorf("expected at least 2 warm hits (B + affinity A), got %d", snap.WarmHitCount)
	}
	if snap.ColdStartCount != 1 {
		t.Errorf("cold start = %d, want 1", snap.ColdStartCount)
	}
}

// TestWarmPoolStaleLeaseRecovery (#992): with MaxLeaseMs set, a process that
// stays busy past the lease bound is force-released and reused on the next
// Acquire instead of spawning a second process for the same key (which would
// leak the first).
func TestWarmPoolStaleLeaseRecovery(t *testing.T) {
	cfg := DefaultWarmPoolConfig()
	cfg.MaxLeaseMs = 30 // force-release leases older than 30ms
	p := NewWarmPool(cfg, spawnCat)
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}

	// Acquire and hold (never Release) to simulate a vanished consumer.
	wp1, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	pid1 := wp1.PID()
	if !wp1.Busy() {
		t.Fatal("process should be busy after acquire")
	}

	// Wait past MaxLeaseMs so the lease is considered stale.
	time.Sleep(60 * time.Millisecond)

	wp2, err := p.Acquire(key, "s2")
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	defer p.Release(wp2)
	if wp2.PID() != pid1 {
		t.Errorf("stale lease should be force-released and reused (pid %d vs %d)", wp2.PID(), pid1)
	}
	if p.Size() != 1 {
		t.Errorf("expected exactly 1 pooled process after stale-lease recovery, got %d", p.Size())
	}
}

// TestWarmPoolMultiplexingServesConcurrentLeases (F149 Phase C): with
// SupportsMultiplexing, a single OS process serves many concurrent leases —
// the prerequisite for multiple agents conversing in the same warm session.
// A second Acquire while the first lease is still held MUST reuse the same
// process (same PID) instead of cold-starting a second one, and the lease
// counter must reflect both holders.
func TestWarmPoolMultiplexingServesConcurrentLeases(t *testing.T) {
	cfg := DefaultWarmPoolConfig()
	cfg.SupportsMultiplexing = true
	p := NewWarmPool(cfg, spawnCat)
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}

	// First lease (held, never released yet).
	wp1, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	pid1 := wp1.PID()
	if wp1.LeaseCount() != 1 {
		t.Fatalf("leaseCount = %d, want 1", wp1.LeaseCount())
	}

	// Second lease, same key, first still held → must reuse same process.
	wp2, err := p.Acquire(key, "s2")
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	if wp2.PID() != pid1 {
		t.Errorf("multiplexing must reuse the same process (pid %d vs %d)", wp2.PID(), pid1)
	}
	if wp2.LeaseCount() != 2 {
		t.Errorf("leaseCount = %d, want 2 (two concurrent holders)", wp2.LeaseCount())
	}
	if p.Size() != 1 {
		t.Errorf("multiplexing keeps exactly 1 pooled process, got %d", p.Size())
	}

	// Release the second lease: process stays alive with the first still held.
	p.Release(wp2)
	if wp1.LeaseCount() != 1 {
		t.Errorf("after one release leaseCount = %d, want 1", wp1.LeaseCount())
	}
	if !wp1.Alive() {
		t.Fatal("process must stay alive while first lease is held")
	}

	// Release the first lease: process returns to idle.
	p.Release(wp1)
	if wp1.LeaseCount() != 0 {
		t.Errorf("after all releases leaseCount = %d, want 0", wp1.LeaseCount())
	}
}

// TestWarmPoolSingleFlightNoReuseWhileBusy documents the single-flight default:
// a process that is still busy under a lease is NOT reused for a different
// concurrent acquire (turns would interleave on shared stdin). A fresh process
// is spawned for the key instead. (Multiplexing is the opt-in path that enables
// shared-session concurrency; see TestWarmPoolMultiplexingServesConcurrentLeases.)
func TestWarmPoolSingleFlightNoReuseWhileBusy(t *testing.T) {
	p := newTestWarmPool()
	defer p.Close()
	key := PoolKey{ProjectPath: "/tmp/proj", ProviderProfile: "claude"}

	wp1, err := p.Acquire(key, "s1")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer wp1.MarkDead() // single-flight overwrite orphans wp1; clean it up
	pid1 := wp1.PID()

	// Second acquire while wp1 is still held (single-flight default).
	wp2, err := p.Acquire(key, "s2")
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	defer p.Release(wp2)
	if wp2.PID() == pid1 {
		t.Errorf("single-flight busy process must not be reused (pid %d)", pid1)
	}
	p.Release(wp1)
}
