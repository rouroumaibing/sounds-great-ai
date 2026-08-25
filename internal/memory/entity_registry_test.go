package memory

import (
	"testing"
)

// F260: approved person/entity lane entries become resolvable; exact match works.
func TestEntityRegistry_ResolveExact(t *testing.T) {
	reg := newEventRegistry(t)
	// Seed an approved person entry (the registry truth source in SG).
	p := reg.Lane(LanePerson).Submit("Alden", "session:1")
	p.OperatorID = "op-1"
	reg.Lane(LanePerson).Approve(p.ID)

	er := NewEntityRegistry(reg)
	got := er.Resolve("op-1", "Alden")
	if got == nil || got.Canonical != "Alden" {
		t.Fatalf("expected Alden resolved, got %+v", got)
	}
	if got.Kind != EntityKindPerson {
		t.Fatalf("expected person kind, got %s", got.Kind)
	}
}

// F260: containment resolve — a known alias inside a longer message resolves.
func TestEntityRegistry_ResolveContainment(t *testing.T) {
	reg := newEventRegistry(t)
	p := reg.Lane(LanePerson).Submit("未婚喵", "session:1")
	p.OperatorID = "op-1"
	reg.Lane(LanePerson).Approve(p.ID)
	er := NewEntityRegistry(reg)

	// The operator wrote a longer sentence containing the alias.
	got := er.Resolve("op-1", "昨天和未婚喵一起去散步了")
	if got == nil || got.Canonical != "未婚喵" {
		t.Fatalf("expected 未婚喵 resolved by containment, got %+v", got)
	}
}

// F260 KD-7: an entry owned by op-1 is NOT resolvable by op-2 (fail-closed).
func TestEntityRegistry_OwnerScopingFailClosed(t *testing.T) {
	reg := newEventRegistry(t)
	p := reg.Lane(LanePerson).Submit("Alden", "session:1")
	p.OperatorID = "op-1"
	reg.Lane(LanePerson).Approve(p.ID)
	er := NewEntityRegistry(reg)

	if got := er.Resolve("op-2", "Alden"); got != nil {
		t.Fatalf("op-2 must not resolve op-1's entity: %+v", got)
	}
}

// F260 Phase A: propose a concept (pending) is not yet resolvable; after
// approval it becomes resolvable, and the proposal defaults to non-canon stance
// (anti stance-collapse guard).
func TestEntityRegistry_ProposeThenApprove(t *testing.T) {
	reg := newEventRegistry(t)
	er := NewEntityRegistry(reg)

	e, err := er.ProposeEntity("op-1", "家属喵", "家属喵", "proposed@thread-x/2026-08-23")
	if err != nil {
		t.Fatalf("ProposeEntity: %v", err)
	}
	// Pending → not resolvable yet.
	if got := er.Resolve("op-1", "家属喵"); got != nil {
		t.Fatalf("pending entity must not resolve: %+v", got)
	}
	// Approve → resolvable.
	if !er.ApproveEntity(e.ID) {
		t.Fatal("ApproveEntity failed")
	}
	got := er.Resolve("op-1", "家属喵")
	if got == nil || got.Status != "approved" {
		t.Fatalf("approved entity should resolve: %+v", got)
	}
	if got.Stance != "endorsed" {
		t.Fatalf("approved person/entity should be endorsed, got %s", got.Stance)
	}
}

// F260 fail-closed: proposing without an owner is rejected.
func TestEntityRegistry_ProposeRequiresOwner(t *testing.T) {
	reg := newEventRegistry(t)
	er := NewEntityRegistry(reg)
	if _, err := er.ProposeEntity("", "x", "x", "src"); err != ErrEntityOwnerRequired {
		t.Fatalf("expected ErrEntityOwnerRequired, got %v", err)
	}
}
