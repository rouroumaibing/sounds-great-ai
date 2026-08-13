package services

import (
	"context"
	"testing"

	"sounds-great-ai/internal/domains/routing/ports"
	"sounds-great-ai/pkg/pack"
)

func TestRouterSingleMention(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧", "@bianmu"}},
		"xigou":  {ID: "xigou", MentionPatterns: []string{"@细狗", "@xigou"}},
	}
	r := NewMentionRouterService(breeds)
	decision, _ := r.Route(context.Background(), "@边牧 分析这个函数")

	if !decision.HasMentions {
		t.Error("expected HasMentions=true")
	}
	if decision.Strategy != "single" {
		t.Errorf("expected single strategy, got %s", decision.Strategy)
	}
	if len(decision.TargetBreeds) != 1 || decision.TargetBreeds[0] != "bianmu" {
		t.Errorf("expected [bianmu], got %v", decision.TargetBreeds)
	}
}

func TestRouterMultipleMentions(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧"}},
		"xigou":  {ID: "xigou", MentionPatterns: []string{"@细狗"}},
	}
	r := NewMentionRouterService(breeds)
	decision, _ := r.Route(context.Background(), "@边牧 @细狗 分析这个函数")

	if decision.Strategy != "parallel" {
		t.Errorf("expected parallel strategy, got %s", decision.Strategy)
	}
	if len(decision.TargetBreeds) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(decision.TargetBreeds))
	}
	if decision.TargetBreeds[0] != "bianmu" {
		t.Errorf("expected bianmu first, got %s", decision.TargetBreeds[0])
	}
	if decision.TargetBreeds[1] != "xigou" {
		t.Errorf("expected xigou second, got %s", decision.TargetBreeds[1])
	}
}

func TestRouterNoMention(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧"}},
	}
	r := NewMentionRouterService(breeds)
	decision, _ := r.Route(context.Background(), "分析这个函数")

	if decision.HasMentions {
		t.Error("expected HasMentions=false")
	}
	if decision.Strategy != "single" {
		t.Errorf("expected single strategy, got %s", decision.Strategy)
	}
	if decision.TargetBreeds[0] != "bianmu" {
		t.Errorf("expected default bianmu, got %s", decision.TargetBreeds[0])
	}
}

func TestRouterChinesePattern(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧"}},
	}
	r := NewMentionRouterService(breeds)
	decision, _ := r.Route(context.Background(), "@边牧 你好")

	if !decision.HasMentions {
		t.Error("expected Chinese pattern to match")
	}
	if decision.TargetBreeds[0] != "bianmu" {
		t.Errorf("expected bianmu, got %s", decision.TargetBreeds[0])
	}
}

func TestRouterDeduplication(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧", "@bianmu"}},
	}
	r := NewMentionRouterService(breeds)
	decision, _ := r.Route(context.Background(), "@边牧 @bianmu 分析")

	if len(decision.TargetBreeds) != 1 {
		t.Errorf("expected dedup to 1 target, got %d", len(decision.TargetBreeds))
	}
}

func TestRouterCaseInsensitive(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@BianMu"}},
	}
	r := NewMentionRouterService(breeds)
	decision, _ := r.Route(context.Background(), "@bianmu 分析")

	if !decision.HasMentions {
		t.Error("expected case-insensitive match")
	}
}

func TestRouterEmptyPatterns(t *testing.T) {
	t.Parallel()
	r := NewMentionRouterService(map[string]*pack.BreedConfig{})
	decision, _ := r.Route(context.Background(), "@边牧 分析")

	// No breeds are registered (empty first-run catalog). The router must not
	// fall back to a hard-coded, non-existent default breed — surface an empty
	// target plus a warning so the caller can prompt the user to add members.
	if len(decision.TargetBreeds) != 0 {
		t.Errorf("expected empty targets for empty registry, got %v", decision.TargetBreeds)
	}
	if len(decision.Warnings) == 0 {
		t.Error("expected a warning prompting the user to add members")
	}
}

func TestRouterNilReceiver(t *testing.T) {
	t.Parallel()
	var r *MentionRouterService
	decision, _ := r.Route(context.Background(), "@边牧 分析")

	if decision.HasMentions {
		t.Error("expected HasMentions=false for nil router")
	}
	if len(decision.TargetBreeds) != 0 {
		t.Errorf("expected empty targets for nil router, got %v", decision.TargetBreeds)
	}
	if len(decision.Warnings) == 0 {
		t.Error("expected a warning for nil/unavailable router")
	}
}

func TestRouterOverlappingPrefixPatterns(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu":       {ID: "bianmu", MentionPatterns: []string{"@边牧"}},
		"bianmu_short": {ID: "bianmu_short", MentionPatterns: []string{"@边"}},
	}
	r := NewMentionRouterService(breeds)
	decision, _ := r.Route(context.Background(), "@边牧 分析")

	// Both patterns match different breeds — dedup is per-breed, not per-pattern.
	// Longest-first sort ensures bianmu (@边牧) is matched before bianmu_short (@边).
	if len(decision.TargetBreeds) != 2 {
		t.Fatalf("expected 2 targets (different breeds), got %d", len(decision.TargetBreeds))
	}
	if decision.TargetBreeds[0] != "bianmu" {
		t.Errorf("expected bianmu first (longer pattern), got %s", decision.TargetBreeds[0])
	}
}

func TestRouterTableDriven(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧", "@bianmu"}},
		"xigou":  {ID: "xigou", MentionPatterns: []string{"@细狗", "@xigou"}},
		"jinmao": {ID: "jinmao", MentionPatterns: []string{"@金毛", "@jinmao"}},
	}
	r := NewMentionRouterService(breeds)

	tests := []struct {
		name         string
		message      string
		wantTargets  []string
		wantStrategy string
		wantMentions bool
	}{
		{"single Chinese", "@边牧 分析", []string{"bianmu"}, "single", true},
		{"single English", "@xigou review", []string{"xigou"}, "single", true},
		{"multiple mentions", "@边牧 @金毛 分析", []string{"bianmu", "jinmao"}, "parallel", true},
		{"serial arrow intent", "@边牧 → @金毛 分析", []string{"bianmu", "jinmao"}, "serial", true},
		{"serial keyword intent", "@边牧 串联 @金毛 分析", []string{"bianmu", "jinmao"}, "serial", true},
		{"serial english intent", "@xigou serial @bianmu review", []string{"xigou", "bianmu"}, "serial", true},
		{"no mention", "just a regular message", []string{"bianmu"}, "single", false},
		{"dedup same breed", "@边牧 @bianmu 分析", []string{"bianmu"}, "single", true},
		{"case insensitive", "@BIANMU 分析", []string{"bianmu"}, "single", true},
		{"mention in middle", "请 @金毛 来审查", []string{"jinmao"}, "single", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision, _ := r.Route(context.Background(), tc.message)

			if decision.HasMentions != tc.wantMentions {
				t.Errorf("HasMentions = %v, want %v", decision.HasMentions, tc.wantMentions)
			}
			if decision.Strategy != tc.wantStrategy {
				t.Errorf("Strategy = %q, want %q", decision.Strategy, tc.wantStrategy)
			}
			if len(decision.TargetBreeds) != len(tc.wantTargets) {
				t.Fatalf("targets len = %d, want %d", len(decision.TargetBreeds), len(tc.wantTargets))
			}
			for i, want := range tc.wantTargets {
				if decision.TargetBreeds[i] != want {
					t.Errorf("target[%d] = %q, want %q", i, decision.TargetBreeds[i], want)
				}
			}
		})
	}
}

var _ = ports.RoutingDecision{}
