package unified

import (
	"os/exec"
	"testing"
	"time"
)

func TestLivenessProbe_StartAndStop(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	cmd.Start()
	defer cmd.Process.Kill()

	probe := NewLivenessProbe(cmd.Process.Pid, 1*time.Second, 2000, 5000)
	probe.Start()

	time.Sleep(100 * time.Millisecond)

	if probe.State != ProbeActive {
		t.Errorf("State = %q, want %q", probe.State, ProbeActive)
	}

	probe.Stop()
}

func TestLivenessProbe_DeadProcess(t *testing.T) {
	cmd := exec.Command("echo", "done")
	cmd.Start()
	cmd.Wait()

	probe := NewLivenessProbe(cmd.Process.Pid, 100*time.Millisecond, 200, 500)
	probe.Start()
	defer probe.Stop()

	time.Sleep(200 * time.Millisecond)

	if probe.State != ProbeDead {
		t.Errorf("State = %q, want %q", probe.State, ProbeDead)
	}
}

func TestLivenessProbe_StateTransitions(t *testing.T) {
	probe := &LivenessProbe{
		PID:          99999,
		PollInterval: 100 * time.Millisecond,
		SoftWarnMs:   200,
		StallWarnMs:  500,
		State:        ProbeActive,
		stopCh:       make(chan struct{}),
	}
	probe.pollOnce()
	if probe.State != ProbeDead {
		t.Errorf("State = %q, want %q for non-existent PID", probe.State, ProbeDead)
	}
}
