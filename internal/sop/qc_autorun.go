package sop

import (
	"context"
	"log"
	"sync"
	"time"
)

// AutoRunner periodically runs the QC loop inside the server so QC is no longer
// solely a developer-run `make qc` gate. It mirrors clowder's eval:qc scheduler
// (F192): QC runs on an interval and emits telemetry. This closes the last
// runtime gap recorded in the SG↔clowder comparison ("QCLoop not wired into the
// server").
//
// In server mode the loop is a repo-health heartbeat: it runs hygiene (+ optional
// heavy build/test) and persists state, but does NOT auto-verify human
// cross-model review — steps 2/3/7 are advisory. Verification of the actual
// review still happens via `cmd/qc` (explicit panel) and the merge gate.
type AutoRunner struct {
	mu           sync.Mutex
	workspaceDir string
	metricsPath  string
	statePath    string
	skipHeavy    bool
	lastResult   *QCLoopResult
	lastErr      string
	lastRun      time.Time
}

// NewAutoRunner creates a runner for the given workspace. skipHeavy controls the
// default for periodic passes (the on-demand endpoint may force heavy).
func NewAutoRunner(workspaceDir string, skipHeavy bool) *AutoRunner {
	return &AutoRunner{
		workspaceDir: workspaceDir,
		metricsPath:  QCMetricsPath(workspaceDir),
		statePath:    QCStatePath(workspaceDir),
		skipHeavy:    skipHeavy,
	}
}

// Start launches the periodic runner. interval <= 0 disables the ticker (the
// loop then only runs on demand via RunNow). The goroutine exits when ctx is
// cancelled. A single pass runs ~5s after startup so status is populated
// without waiting a full interval.
func (r *AutoRunner) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			r.RunNow(r.skipHeavy)
		}
	}()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.RunNow(r.skipHeavy)
			}
		}
	}()
	log.Printf("QC AutoRunner started (interval=%s, skipHeavy=%v)", interval, r.skipHeavy)
}

// RunNow executes one QC loop pass immediately (used by the periodic ticker and
// the on-demand POST /api/qc/run endpoint). When forceHeavy is true it runs the
// heavy build/test step regardless of the runner's skipHeavy default.
func (r *AutoRunner) RunNow(forceHeavy bool) QCLoopResult {
	loop := NewQCLoop(r.workspaceDir)
	loop.StatePath = r.statePath
	input := QCLoopInput{
		WorkDir:     r.workspaceDir,
		ServerMode:  true,
		SkipHeavy:   r.skipHeavy && !forceHeavy,
		AuthorBreed: "server-auto",
	}
	result := loop.Run(input)

	rec := QCMetricsRecord{
		WorkDir:     r.workspaceDir,
		AuthorBreed: "server-auto",
		Passed:      result.Passed,
		Steps:       result.Steps,
	}
	if err := RecordQCMetrics(r.metricsPath, rec); err != nil {
		log.Printf("QC AutoRunner: metrics write failed: %v", err)
	}

	r.mu.Lock()
	r.lastResult = &result
	r.lastRun = time.Now()
	r.lastErr = ""
	r.mu.Unlock()
	return result
}

// Last returns a snapshot of the most recent run.
func (r *AutoRunner) Last() (QCLoopResult, time.Time, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastResult == nil {
		return QCLoopResult{}, r.lastRun, "no run yet"
	}
	return *r.lastResult, r.lastRun, r.lastErr
}

// MetricsPath exposes the resolved metrics file path (used by the status
// endpoint to fold in aggregate telemetry).
func (r *AutoRunner) MetricsPath() string { return r.metricsPath }

// StatePath exposes the resolved state file path.
func (r *AutoRunner) StatePath() string { return r.statePath }
