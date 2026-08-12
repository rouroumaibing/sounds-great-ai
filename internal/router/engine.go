package router

import (
	"context"

	"sounds-great-ai/pkg/pack"
)

type RoutingDecision struct {
	Plan   []RoutingStep
	Reason string
}

type RoutingStep struct {
	BreedID   string
	VariantID string
	Task      string
	Skills    []string
	DependsOn []int
	Role      string
}

type RoutingRule = pack.RoutingRule

type RoutingEngine struct {
	rules  []RoutingRule
	roster map[string]*pack.BreedConfig
}

func NewEngine(rules []RoutingRule, roster map[string]*pack.BreedConfig) *RoutingEngine {
	return &RoutingEngine{rules: rules, roster: roster}
}

func (e *RoutingEngine) Route(ctx context.Context, task string) (*RoutingDecision, error) {
	if decision := e.matchRules(task); decision != nil {
		return decision, nil
	}
	return &RoutingDecision{Plan: []RoutingStep{{Role: "primary", Task: task}}, Reason: "no rule matched — default single-step"}, nil
}
