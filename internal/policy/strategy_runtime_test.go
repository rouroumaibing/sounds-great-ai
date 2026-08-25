package policy

import (
	"errors"
	"testing"
)

// mockResolver resolves a configured strategy id to Allow; returns err when the
// id is in errIDs. Returns nil (abstain) for any other id.
type mockResolver struct {
	allow map[string]bool
	errIDs map[string]bool
}

func (m *mockResolver) Resolve(ref StrategyRef, ctx StrategyContext) (*StrategyDecision, error) {
	if m.errIDs[ref.ID] {
		return nil, errors.New("boom")
	}
	if allow, ok := m.allow[ref.ID]; ok {
		d := &StrategyDecision{StrategyID: ref.ID, Allow: allow, Reason: "mock"}
		return d, nil
	}
	return nil, nil
}

func TestStrategyChain_ByPriority(t *testing.T) {
	r := &mockResolver{allow: map[string]bool{"low": false, "high": true}}
	// high has lower rank => consulted first, wins.
	chain := NewStrategyChain(
		StrategyRef{ID: "low", PriorityRank: 10, Enabled: true},
		StrategyRef{ID: "high", PriorityRank: 1, Enabled: true},
	)
	d := chain.Resolve(r, StrategyContext{})
	if !d.Allow {
		t.Fatalf("expected high-priority strategy to allow, got %+v", d)
	}
	if d.StrategyID != "high" {
		t.Fatalf("expected winner high, got %s", d.StrategyID)
	}
}

func TestStrategyChain_DisabledSkipped(t *testing.T) {
	r := &mockResolver{allow: map[string]bool{"first": false, "second": true}}
	chain := NewStrategyChain(
		StrategyRef{ID: "first", PriorityRank: 1, Enabled: false}, // skipped
		StrategyRef{ID: "second", PriorityRank: 2, Enabled: true},
	)
	d := chain.Resolve(r, StrategyContext{})
	if !d.Allow || d.StrategyID != "second" {
		t.Fatalf("expected second to win after first disabled, got %+v", d)
	}
}

func TestStrategyChain_FailClosed_Empty(t *testing.T) {
	chain := NewStrategyChain()
	d := chain.Resolve(&mockResolver{}, StrategyContext{})
	if d.Allow {
		t.Fatal("empty chain must deny (fail-closed)")
	}
}

func TestStrategyChain_FailClosed_NilResolver(t *testing.T) {
	chain := NewStrategyChain(StrategyRef{ID: "x", PriorityRank: 1, Enabled: true})
	d := chain.Resolve(nil, StrategyContext{})
	if d.Allow {
		t.Fatal("nil resolver must deny (fail-closed)")
	}
}

func TestStrategyChain_FailClosed_ResolverError(t *testing.T) {
	r := &mockResolver{errIDs: map[string]bool{"bad": true}}
	chain := NewStrategyChain(StrategyRef{ID: "bad", PriorityRank: 1, Enabled: true})
	d := chain.Resolve(r, StrategyContext{})
	if d.Allow {
		t.Fatal("resolver error must deny (fail-closed)")
	}
}

func TestStrategyChain_FailClosed_NoDecision(t *testing.T) {
	// Resolver abstains for everything (no allow map entries).
	chain := NewStrategyChain(StrategyRef{ID: "a", PriorityRank: 1, Enabled: true})
	d := chain.Resolve(&mockResolver{}, StrategyContext{})
	if d.Allow {
		t.Fatal("chain with no resolving strategy must deny (fail-closed)")
	}
}

func TestSealTransitions(t *testing.T) {
	cases := []struct {
		from, to SealState
		ok       bool
	}{
		{SealOpen, SealSealing, true},
		{SealSealing, SealSealed, true},
		{SealSealing, SealOpen, true},
		{SealSealed, SealReleased, true},
		{SealReleased, SealOpen, true},
		{SealOpen, SealSealed, false},   // must go through sealing
		{SealSealed, SealOpen, false},   // must go through released
		{SealOpen, SealReleased, false}, // illegal
	}
	for _, c := range cases {
		got, err := TransitionSeal(c.from, c.to)
		if c.ok && err != nil {
			t.Fatalf("%s->%s: expected ok, got %v", c.from, c.to, err)
		}
		if !c.ok {
			if err == nil {
				t.Fatalf("%s->%s: expected error", c.from, c.to)
			}
			if got != c.from {
				t.Fatalf("%s->%s: state should be unchanged on error", c.from, c.to)
			}
		}
	}
}

func TestSnapshot(t *testing.T) {
	chain := NewStrategyChain(StrategyRef{ID: "a", PriorityRank: 1, Enabled: true})
	d := chain.Resolve(&mockResolver{allow: map[string]bool{"a": true}}, StrategyContext{})
	s := Snapshot("chain1", "th1", "u1", "trace1", d)
	if s.ChainID != "chain1" || s.ThreadID != "th1" || !s.Decision.Allow {
		t.Fatalf("snapshot mismatch: %+v", s)
	}
	if s.TakenAt.IsZero() {
		t.Fatal("snapshot time must be set")
	}
}
