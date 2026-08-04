package sop

import (
	"testing"
	"sounds-great-ai/internal/a2a"
)

func TestCheckA2ADepthContinue(t *testing.T) {
	g := NewGuardian(nil, 3)
	thread := &a2a.Thread{ID: "test", ReviewRoundCount: 1}
	if action := g.CheckA2ADepth(thread); action != Continue {
		t.Errorf("expected Continue, got %v", action)
	}
}

func TestCheckA2ADepthEscalate(t *testing.T) {
	g := NewGuardian(nil, 3)
	thread := &a2a.Thread{ID: "test", ReviewRoundCount: 3}
	if action := g.CheckA2ADepth(thread); action != EscalateToCVO {
		t.Errorf("expected EscalateToCVO, got %v", action)
	}
}

func TestCheckA2ADepthBoundary(t *testing.T) {
	g := NewGuardian(nil, 3)
	thread := &a2a.Thread{ID: "test", ReviewRoundCount: 2}
	if action := g.CheckA2ADepth(thread); action != Continue {
		t.Errorf("expected Continue at round 2 (limit 3), got %v", action)
	}
}
