package unified

import (
	"os"
	"syscall"
	"time"
)

type ProbeState string

const (
	ProbeActive     ProbeState = "active"
	ProbeBusySilent ProbeState = "busy_silent"
	ProbeIdleSilent ProbeState = "idle_silent"
	ProbeDead       ProbeState = "dead"
)

type LivenessProbe struct {
	PID          int
	PollInterval time.Duration
	SoftWarnMs   int
	StallWarnMs  int
	State        ProbeState
	stopCh       chan struct{}
	lastCPUTime  time.Duration
	silentSince  time.Time
	// hardStallNotified guards against spamming the hard-stall warning on every
	// poll once the child has been silent beyond StallWarnMs. Reset on recovery.
	hardStallNotified bool
	// OnStall is invoked when the probe transitions into a stalled
	// (alive-but-silent) state or recovers. The hard flag distinguishes a soft
	// warning (beyond SoftWarnMs) from a hard stall (beyond StallWarnMs).
	OnStall func(state ProbeState, hard bool)
}

// LivenessMessage renders a human-facing, safe message for a probe state change
// (R8). It never includes process internals — only a status hint for the UI.
func LivenessMessage(state ProbeState, hard bool) string {
	switch state {
	case ProbeIdleSilent:
		if hard {
			return "CLI 长时间无响应（疑似卡死），仍在等待输出…"
		}
		return "CLI 响应较慢，正在等待输出…"
	case ProbeActive:
		return "CLI 已恢复响应"
	case ProbeBusySilent:
		return "CLI 正在处理（高 CPU，但无文本输出）"
	default:
		return "CLI 状态：" + string(state)
	}
}

func NewLivenessProbe(pid int, pollInterval time.Duration, softWarnMs, stallWarnMs int) *LivenessProbe {
	return &LivenessProbe{
		PID:          pid,
		PollInterval: pollInterval,
		SoftWarnMs:   softWarnMs,
		StallWarnMs:  stallWarnMs,
		State:        ProbeActive,
		stopCh:       make(chan struct{}),
	}
}

func (p *LivenessProbe) Start() {
	go func() {
		ticker := time.NewTicker(p.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.pollOnce()
			}
		}
	}()
}

func (p *LivenessProbe) Stop() {
	close(p.stopCh)
}

func (p *LivenessProbe) pollOnce() {
	if !processAlive(p.PID) {
		p.State = ProbeDead
		return
	}

	cpuTime := getProcessCPUTime(p.PID)
	now := time.Now()

	if p.silentSince.IsZero() {
		p.silentSince = now
	}

	cpuGrowing := cpuTime > p.lastCPUTime
	p.lastCPUTime = cpuTime

	silentDuration := int(now.Sub(p.silentSince) / time.Millisecond)

	switch p.State {
	case ProbeActive:
		if !cpuGrowing && silentDuration > p.SoftWarnMs {
			p.State = ProbeIdleSilent
			if p.OnStall != nil {
				p.OnStall(p.State, false) // soft stall warning
			}
		} else if cpuGrowing && silentDuration > p.SoftWarnMs {
			p.State = ProbeBusySilent
		}
	case ProbeBusySilent:
		if !cpuGrowing {
			p.State = ProbeIdleSilent
			if p.OnStall != nil {
				p.OnStall(p.State, false) // soft stall warning
			}
		}
	case ProbeIdleSilent:
		if silentDuration > p.StallWarnMs && !p.hardStallNotified {
			p.hardStallNotified = true
			if p.OnStall != nil {
				p.OnStall(p.State, true) // hard stall warning (once)
			}
		}
		if cpuGrowing {
			p.State = ProbeActive
			p.silentSince = time.Time{}
			p.hardStallNotified = false
			if p.OnStall != nil {
				p.OnStall(p.State, false) // recovered
			}
		}
	}
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// getProcessCPUTime returns the total CPU time consumed by pid. It is provided
// per-platform: probe_linux.go (Linux /proc), probe_darwin.go (macOS ps), and
// probe_other.go (fallback) — exactly one is compiled per target.
