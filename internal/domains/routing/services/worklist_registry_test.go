package services

import (
	"testing"

	routingPorts "sounds-great-ai/internal/domains/routing/ports"
)

// G2: per-message depth ceiling terminates the chain once maxDepth is reached.
func TestWorklistDepthCeiling(t *testing.T) {
	r := NewWorklistRegistry()
	inv := "depth-inv"
	r.Register(inv, 3)

	// A->B->C->D accepted (3 pushes, 4 dogs).
	if ok, _, _ := r.Push(inv, "a", "b", routingPorts.SubstantiveActivity{}); !ok {
		t.Fatal("push 1 (a->b) rejected")
	}
	if ok, _, _ := r.Push(inv, "b", "c", routingPorts.SubstantiveActivity{}); !ok {
		t.Fatal("push 2 (b->c) rejected")
	}
	if ok, _, _ := r.Push(inv, "c", "d", routingPorts.SubstantiveActivity{}); !ok {
		t.Fatal("push 3 (c->d) rejected")
	}
	// 4th push hits the depth ceiling.
	ok, reason, _ := r.Push(inv, "d", "e", routingPorts.SubstantiveActivity{})
	if ok {
		t.Fatal("push 4 should hit depth ceiling")
	}
	if reason != "depth" {
		t.Fatalf("reason = %s, want depth", reason)
	}
}

// G2: ping-pong breaker warns at 2 and blocks at 4 consecutive same-pair handoffs
// (maxDepth floored high so depth never trips first).
func TestWorklistPingPongBreaker(t *testing.T) {
	r := NewWorklistRegistry()
	inv := "pingpong-inv"
	r.Register(inv, 8) // depth won't trigger before ping-pong does

	// 1: a->b (nc=1) accept, no warn.
	ok, _, warn := r.Push(inv, "a", "b", routingPorts.SubstantiveActivity{})
	if !ok || warn {
		t.Fatalf("push1 ok=%v warn=%v", ok, warn)
	}
	// 2: b->a (nc=2) accept, WARN.
	ok, _, warn = r.Push(inv, "b", "a", routingPorts.SubstantiveActivity{})
	if !ok || !warn {
		t.Fatalf("push2 ok=%v warn=%v (want warn)", ok, warn)
	}
	// 3: a->b (nc=3) accept, warn.
	ok, _, warn = r.Push(inv, "a", "b", routingPorts.SubstantiveActivity{})
	if !ok || !warn {
		t.Fatalf("push3 ok=%v warn=%v (want warn)", ok, warn)
	}
	// 4: b->a (nc=4) BLOCK.
	ok, reason, _ := r.Push(inv, "b", "a", routingPorts.SubstantiveActivity{})
	if ok {
		t.Fatal("push4 should be blocked by ping-pong breaker")
	}
	if reason != "pingpong" {
		t.Fatalf("reason = %s, want pingpong", reason)
	}
}

// G2: substantive work (long output) resets the streak so a real chain may
// continue past the ping-pong threshold.
func TestWorklistSubstantiveResetsStreak(t *testing.T) {
	r := NewWorklistRegistry()
	inv := "subst-inv"
	r.Register(inv, 8)

	r.Push(inv, "a", "b", routingPorts.SubstantiveActivity{})              // nc=1
	r.Push(inv, "b", "a", routingPorts.SubstantiveActivity{})              // nc=2 (warn)
	// Substantive handoff a->b: streak resets to 1 (no warn).
	ok, _, warn := r.Push(inv, "a", "b", routingPorts.SubstantiveActivity{OutputLen: 300})
	if !ok {
		t.Fatal("substantive push rejected")
	}
	if warn {
		t.Fatal("substantive push should reset streak (no warn)")
	}
	// After reset, the same pair may bounce twice more before blocking again.
	if ok, _, _ := r.Push(inv, "b", "a", routingPorts.SubstantiveActivity{}); !ok {
		t.Fatal("push after reset (nc=2) should be accepted")
	}
	if ok, _, _ := r.Push(inv, "a", "b", routingPorts.SubstantiveActivity{}); !ok {
		t.Fatal("push after reset (nc=3) should be accepted")
	}
	if ok, reason, _ := r.Push(inv, "b", "a", routingPorts.SubstantiveActivity{}); ok || reason != "pingpong" {
		t.Fatalf("push after reset (nc=4) should block, ok=%v reason=%s", ok, reason)
	}
}

// G2: genuinely distinct pairs keep their own streak (depth is the only limiter).
func TestWorklistDistinctPairsNoFalsePingPong(t *testing.T) {
	r := NewWorklistRegistry()
	inv := "chain-inv"
	r.Register(inv, 8)
	// a->b, b->c, c->d are all distinct unordered pairs: each is nc=1.
	for i, pair := range [][2]string{{"a", "b"}, {"b", "c"}, {"c", "d"}} {
		ok, _, _ := r.Push(inv, pair[0], pair[1], routingPorts.SubstantiveActivity{})
		if !ok {
			t.Fatalf("chain push %d (%s->%s) wrongly rejected", i, pair[0], pair[1])
		}
	}
}

// G2: Register is idempotent across recursive handoffs (budget is never reset).
func TestWorklistRegisterIdempotent(t *testing.T) {
	r := NewWorklistRegistry()
	inv := "idem-inv"
	r.Register(inv, 2)
	r.Register(inv, 99) // second call must NOT widen the budget
	if ok, _, _ := r.Push(inv, "a", "b", routingPorts.SubstantiveActivity{}); !ok {
		t.Fatal("push1 rejected")
	}
	if ok, _, _ := r.Push(inv, "b", "c", routingPorts.SubstantiveActivity{}); !ok {
		t.Fatal("push2 rejected")
	}
	if ok, reason, _ := r.Push(inv, "c", "d", routingPorts.SubstantiveActivity{}); ok || reason != "depth" {
		t.Fatalf("push3 should hit depth=2, ok=%v reason=%s", ok, reason)
	}
}

// G11: PushToWorklist dynamically expands the running worklist with new targets,
// records source attribution (a2aFrom), and dedups already-known/executed ones.
func TestWorklistPushToWorklist(t *testing.T) {
	r := NewWorklistRegistry()
	inv := "fanout-inv"
	r.Register(inv, 8)

	// Pre-register a planned target via a normal push.
	if ok, _, _ := r.Push(inv, "a", "b", routingPorts.SubstantiveActivity{}); !ok {
		t.Fatal("planned push a->b rejected")
	}
	// Dynamic expansion: a pulls in c and d (c is new, b already known).
	added := r.PushToWorklist(inv, []string{"c", "b", "d"}, "a")
	if len(added) != 2 {
		t.Fatalf("added = %v, want [c d]", added)
	}
	if r.A2AFrom(inv, "c") != "a" {
		t.Fatalf("a2aFrom[c] = %q, want a", r.A2AFrom(inv, "c"))
	}
	if r.A2AFrom(inv, "d") != "a" {
		t.Fatalf("a2aFrom[d] = %q, want a", r.A2AFrom(inv, "d"))
	}
	// Re-adding known targets yields nothing.
	if again := r.PushToWorklist(inv, []string{"c", "d"}, "x"); len(again) != 0 {
		t.Fatalf("re-add should be empty, got %v", again)
	}
}

// G11: PushToWorklist caps the fan-out so a runaway expansion cannot grow
// unbounded.
func TestWorklistPushToWorklistCap(t *testing.T) {
	r := NewWorklistRegistry()
	inv := "cap-inv"
	r.Register(inv, 8)
	targets := make([]string, 0, maxWorklistFanout+5)
	for i := 0; i < maxWorklistFanout+5; i++ {
		targets = append(targets, "t"+string(rune('a'+i)))
	}
	added := r.PushToWorklist(inv, targets, "a")
	if len(added) != maxWorklistFanout {
		t.Fatalf("added = %d, want cap %d", len(added), maxWorklistFanout)
	}
}
