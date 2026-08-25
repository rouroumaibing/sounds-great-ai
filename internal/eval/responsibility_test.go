package eval

import "testing"

func TestResponsibilityInbox_StateMachine(t *testing.T) {
	r := NewResponsibilityInbox()
	r.Open("item-1")
	if s, _ := r.State("item-1"); s != RespOpen {
		t.Fatalf("new item should be open, got %s", s)
	}
	if err := r.Transition("item-1", RespClaimed); err != nil {
		t.Fatalf("open->claimed should be legal: %v", err)
	}
	if err := r.Transition("item-1", RespResolved); err != nil {
		t.Fatalf("claimed->resolved should be legal: %v", err)
	}
	// Resolved is terminal: any transition is illegal (fail-closed).
	if err := r.Transition("item-1", RespClaimed); err != ErrInvalidRespTransition {
		t.Fatalf("resolved->claimed must be illegal, got %v", err)
	}
	// Unknown id fails closed.
	if err := r.Transition("ghost", RespClaimed); err != ErrInvalidRespTransition {
		t.Fatalf("unknown id must fail closed, got %v", err)
	}
}

func TestResponsibilityInbox_Escalation(t *testing.T) {
	r := NewResponsibilityInbox()
	r.Open("x")
	if err := r.Transition("x", RespEscalated); err != nil {
		t.Fatalf("open->escalated legal: %v", err)
	}
	if err := r.Transition("x", RespOpen); err != nil {
		t.Fatalf("escalated->open legal: %v", err)
	}
}
