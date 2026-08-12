package router

import (
	"context"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestMatchRulesCodeReview(t *testing.T) {
	rules := []pack.RoutingRule{
		{TaskType: "code_review", AssignRoles: []string{"reviewer"}, RequireCrossBreed: true, Skills: []string{"code-review"}},
	}
	engine := NewEngine(rules, nil)
	decision, err := engine.Route(context.Background(), "please do a code_review of this PR")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if decision == nil {
		t.Fatal("expected non-nil decision")
	}
	if len(decision.Plan) == 0 {
		t.Fatal("expected non-empty plan")
	}
}

func TestMatchRulesNoMatch(t *testing.T) {
	rules := []pack.RoutingRule{
		{TaskType: "code_review", AssignRoles: []string{"reviewer"}},
	}
	engine := NewEngine(rules, nil)
	decision, err := engine.Route(context.Background(), "deploy to production")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if decision.Reason != "no rule matched — default single-step" {
		t.Errorf("expected default fallback, got %s", decision.Reason)
	}
}
