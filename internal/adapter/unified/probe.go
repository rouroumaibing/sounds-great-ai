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
		} else if cpuGrowing && silentDuration > p.SoftWarnMs {
			p.State = ProbeBusySilent
		}
	case ProbeBusySilent:
		if !cpuGrowing {
			p.State = ProbeIdleSilent
		}
	case ProbeIdleSilent:
		if cpuGrowing {
			p.State = ProbeActive
			p.silentSince = time.Time{}
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

func getProcessCPUTime(pid int) time.Duration {
	return 0
}
