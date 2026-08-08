package cue

import "testing"

func TestCatalogDetectSubjectSeen(t *testing.T) {
	c := NewCatalog()
	input := CatalogInput{
		SubjectMentions: []string{"UserService", "AuthService"},
		KnownSubjects:   []string{"UserService", "ConfigService"},
	}
	ops := c.Detect(input)
	if len(ops) == 0 {
		t.Fatal("expected at least 1 opportunity")
	}
	found := false
	for _, op := range ops {
		if op.Type == OpSubjectSeen && op.Subject == "UserService" {
			found = true
		}
	}
	if !found {
		t.Error("expected subject_seen for UserService")
	}
}

func TestCatalogDetectDeliveryDecision(t *testing.T) {
	c := NewCatalog()
	input := CatalogInput{
		DeliveryDecisionPending: true,
	}
	ops := c.Detect(input)
	if len(ops) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(ops))
	}
	if ops[0].Type != OpDeliveryDecision {
		t.Errorf("expected delivery_decision, got %s", ops[0].Type)
	}
}

func TestCatalogDetectJudgmentSurface(t *testing.T) {
	c := NewCatalog()
	input := CatalogInput{
		JudgmentSurfaceEntered: true,
	}
	ops := c.Detect(input)
	if len(ops) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(ops))
	}
	if ops[0].Type != OpJudgmentSurfaceEntered {
		t.Errorf("expected judgment_surface_entered, got %s", ops[0].Type)
	}
}

func TestCatalogFailClosedOnEmpty(t *testing.T) {
	c := NewCatalog()
	input := CatalogInput{}
	ops := c.Detect(input)
	if len(ops) != 0 {
		t.Fatalf("expected 0 opportunities for empty input, got %d", len(ops))
	}
}

func TestIsKnownType(t *testing.T) {
	if !IsKnownType(OpSubjectSeen) {
		t.Error("OpSubjectSeen should be known")
	}
	if IsKnownType("unknown_type") {
		t.Error("unknown type should not be known")
	}
}

func TestAllOpportunityTypes(t *testing.T) {
	types := AllOpportunityTypes()
	if len(types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(types))
	}
}
