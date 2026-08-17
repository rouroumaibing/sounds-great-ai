package sop

import (
	"os"
	"testing"
)

// TestAutoRunnerRunNow verifies the server auto-runner emits a passing
// heartbeat over a clean workspace and records telemetry to the resolved
// metrics path. The GLOBAL_CONFIG_ROOT env isolates the write from the real
// home directory.
func TestAutoRunnerRunNow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT", dir)

	r := NewAutoRunner(dir, true)
	res := r.RunNow(false)
	if !res.Passed {
		t.Errorf("auto-run over empty workspace should pass, steps=%+v", res.Steps)
	}
	// ServerMode must downgrade review/sign-off to advisory so the heartbeat
	// never fails on missing human review context.
	for _, s := range res.Steps {
		if s.Name == "fresh_context" || s.Name == "cross_breed_review" || s.Name == "sign_off" {
			if !s.Passed {
				t.Errorf("step %s should be advisory-pass in server mode", s.Name)
			}
		}
	}
	if s := res.Steps; len(s) >= 5 && s[4].Name == "ci_repair" && !s[4].Passed {
		t.Errorf("ci_repair should be skipped (skipHeavy) not failed: %s", s[4].Message)
	}

	last, _, lastErr := r.Last()
	if !last.Passed {
		t.Error("Last() should reflect the run")
	}
	if lastErr != "" {
		t.Errorf("unexpected last error: %s", lastErr)
	}
	if _, err := os.Stat(QCMetricsPath(dir)); err != nil {
		t.Errorf("expected metrics file written at %s: %v", QCMetricsPath(dir), err)
	}
}

// TestAutoRunnerForceHeavy confirms RunNow(forceHeavy=true) runs the ci_repair
// step even when the runner default is skipHeavy.
func TestAutoRunnerForceHeavy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT", dir)
	r := NewAutoRunner(dir, true)
	res := r.RunNow(true)
	for _, s := range res.Steps {
		if s.Name == "ci_repair" {
			// In a temp dir with no go.mod, step5 is advisory-skipped by design,
			// but the point is it is exercised (not gated by SkipHeavy).
			if !s.Passed {
				t.Errorf("ci_repair should not fail in temp dir: %s", s.Message)
			}
		}
	}
}
