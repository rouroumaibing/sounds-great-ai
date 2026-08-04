package router

import (
	"context"
	"sounds-great-ai/internal/config"
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

type RoutingRule = config.RoutingRule

type RoutingEngine struct {
	rules  []RoutingRule
	roster map[string]*config.BreedConfig
}

func NewEngine(rules []RoutingRule, roster map[string]*config.BreedConfig) *RoutingEngine {
	return &RoutingEngine{rules: rules, roster: roster}
}

func (e *RoutingEngine) Route(ctx context.Context, task string) (*RoutingDecision, error) {
	if decision := e.matchRules(task); decision != nil {
		return decision, nil
	}
	return &RoutingDecision{Plan: []RoutingStep{{Role: "primary", Task: task}}, Reason: "no rule matched — default single-step"}, nil
}
