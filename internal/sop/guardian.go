package sop

import "sounds-great-ai/internal/a2a"

type EscalationAction int

const (
	Continue EscalationAction = iota
	EscalateToCVO
	Block
)

type SOPGate struct {
	ID        string
	Trigger   string
	Condition func(*a2a.Thread) bool
	Action    EscalationAction
}

type SOPGuardian struct {
	gates       []SOPGate
	maxA2ADepth int
}

func NewGuardian(gates []SOPGate, maxA2ADepth int) *SOPGuardian {
	if maxA2ADepth <= 0 {
		maxA2ADepth = 3
	}
	return &SOPGuardian{gates: gates, maxA2ADepth: maxA2ADepth}
}

func (g *SOPGuardian) CheckA2ADepth(thread *a2a.Thread) EscalationAction {
	if thread.ReviewRoundCount >= g.maxA2ADepth {
		return EscalateToCVO
	}
	return Continue
}

func (g *SOPGuardian) MaxA2ADepth() int {
	return g.maxA2ADepth
}
