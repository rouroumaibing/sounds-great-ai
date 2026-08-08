package sop

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return rdb, mr
}

func TestWorkflowSOPMemoryMode(t *testing.T) {
	ws := NewWorkflowSOP(nil) // nil → memory mode
	ctx := context.Background()

	if err := ws.TransitionStage(ctx, "feat1", "kickoff"); err != nil {
		t.Fatalf("kickoff failed: %v", err)
	}
	state, err := ws.GetState(ctx, "feat1")
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.Stage != "kickoff" {
		t.Errorf("expected kickoff stage, got %+v", state)
	}
}

func TestWorkflowSOPRedisMode(t *testing.T) {
	rdb, _ := newTestRedis(t)
	ws := NewWorkflowSOP(rdb)
	ctx := context.Background()

	if err := ws.TransitionStage(ctx, "feat1", "kickoff"); err != nil {
		t.Fatalf("kickoff failed: %v", err)
	}
	state, err := ws.GetState(ctx, "feat1")
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.Stage != "kickoff" {
		t.Errorf("expected kickoff stage, got %+v", state)
	}
}

func TestWorkflowSOPTransitions(t *testing.T) {
	ws := NewWorkflowSOP(nil)
	ctx := context.Background()

	ws.TransitionStage(ctx, "feat1", "kickoff")
	if err := ws.TransitionStage(ctx, "feat1", "impl"); err != nil {
		t.Fatalf("kickoff→impl failed: %v", err)
	}
	if err := ws.TransitionStage(ctx, "feat1", "quality_gate"); err != nil {
		t.Fatalf("impl→quality_gate failed: %v", err)
	}
	if err := ws.TransitionStage(ctx, "feat1", "review"); err != nil {
		t.Fatalf("quality_gate→review failed: %v", err)
	}
	if err := ws.TransitionStage(ctx, "feat1", "merge"); err != nil {
		t.Fatalf("review→merge failed: %v", err)
	}
	if err := ws.TransitionStage(ctx, "feat1", "completion"); err != nil {
		t.Fatalf("merge→completion failed: %v", err)
	}
}

func TestWorkflowSOPInvalidTransition(t *testing.T) {
	ws := NewWorkflowSOP(nil)
	ctx := context.Background()

	ws.TransitionStage(ctx, "feat1", "kickoff")
	if err := ws.TransitionStage(ctx, "feat1", "merge"); err == nil {
		t.Error("expected error for kickoff→merge")
	}
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from, to string
		valid    bool
	}{
		{"", "kickoff", true},
		{"kickoff", "impl", true},
		{"impl", "quality_gate", true},
		{"quality_gate", "review", true},
		{"quality_gate", "impl", true}, // can go back
		{"review", "merge", true},
		{"review", "impl", true}, // can go back
		{"merge", "completion", true},
		{"merge", "review", true}, // can go back
		{"kickoff", "merge", false},
		{"completion", "impl", false},
	}
	for _, tt := range tests {
		if got := IsValidTransition(tt.from, tt.to); got != tt.valid {
			t.Errorf("IsValidTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.valid)
		}
	}
}

func TestWorkflowSOPCheckAttest(t *testing.T) {
	rdb, _ := newTestRedis(t)
	ws := NewWorkflowSOP(rdb)
	ctx := context.Background()

	ws.TransitionStage(ctx, "feat1", "kickoff")
	if err := ws.AttestCheck(ctx, "feat1", "spec_exists"); err != nil {
		t.Fatalf("attest failed: %v", err)
	}
	state, _ := ws.GetState(ctx, "feat1")
	if len(state.Checks) != 1 || state.Checks[0].Status != CheckAttested {
		t.Errorf("expected 1 attested check, got %+v", state.Checks)
	}
}

func TestWorkflowSOPCheckVerify(t *testing.T) {
	ws := NewWorkflowSOP(nil)
	ctx := context.Background()

	ws.TransitionStage(ctx, "feat1", "kickoff")
	ws.AttestCheck(ctx, "feat1", "spec_exists")
	ws.VerifyCheck(ctx, "feat1", "spec_exists")
	state, _ := ws.GetState(ctx, "feat1")
	if len(state.Checks) != 1 || state.Checks[0].Status != CheckVerified {
		t.Errorf("expected 1 verified check, got %+v", state.Checks)
	}
}

func TestWorkflowSOPResume(t *testing.T) {
	rdb, _ := newTestRedis(t)
	ws := NewWorkflowSOP(rdb)
	ctx := context.Background()

	ws.TransitionStage(ctx, "feat1", "kickoff")
	ws.TransitionStage(ctx, "feat1", "impl")
	ws.AttestCheck(ctx, "feat1", "worktree_created")

	// Simulate restart: create new WorkflowSOP with same Redis
	ws2 := NewWorkflowSOP(rdb)
	state, err := ws2.Resume(ctx, "feat1")
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if state == nil || state.Stage != "impl" {
		t.Errorf("expected impl stage after resume, got %+v", state)
	}
	if len(state.Checks) != 1 {
		t.Errorf("expected 1 check after resume, got %d", len(state.Checks))
	}
}

func TestWorkflowSOPCASConcurrentModification(t *testing.T) {
	ws := NewWorkflowSOP(nil)
	ctx := context.Background()

	ws.TransitionStage(ctx, "feat1", "kickoff")
	// Try to set with wrong expected stage
	err := ws.SetState(ctx, WorkflowState{
		FeatureID: "feat1",
		Stage:     "impl",
	}, "wrong_stage")
	if err != ErrConcurrentModification {
		t.Errorf("expected ErrConcurrentModification, got %v", err)
	}
}

func TestWorkflowSOPGracefulDegradation(t *testing.T) {
	// Redis with bad address should fall back to memory
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:1"})
	ws := NewWorkflowSOP(rdb)
	if !ws.useMemory {
		t.Error("expected memory mode for unreachable Redis")
	}
	ctx := context.Background()
	if err := ws.TransitionStage(ctx, "feat1", "kickoff"); err != nil {
		t.Fatalf("memory fallback failed: %v", err)
	}
}
