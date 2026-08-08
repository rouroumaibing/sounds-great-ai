package platform

import (
	"testing"

	"sounds-great-ai/internal/config"
)

func TestRouterSingleMention(t *testing.T) {
	t.Parallel()
	breeds := map[string]*config.BreedConfig{
		"bianmu": {
			ID: "bianmu", MentionPatterns: []string{"@边牧", "@bianmu"},
		},
		"xigou": {
			ID: "xigou", MentionPatterns: []string{"@细狗", "@xigou"},
		},
	}
	r := NewRouter(breeds)
	decision := r.Route("@边牧 分析这个函数")

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
	breeds := map[string]*config.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧"}},
		"xigou":  {ID: "xigou", MentionPatterns: []string{"@细狗"}},
	}
	r := NewRouter(breeds)
	decision := r.Route("@边牧 @细狗 分析这个函数")

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
	breeds := map[string]*config.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧"}},
	}
	r := NewRouter(breeds)
	decision := r.Route("分析这个函数")

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
	breeds := map[string]*config.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧"}},
	}
	r := NewRouter(breeds)
	decision := r.Route("@边牧 你好")

	if !decision.HasMentions {
		t.Error("expected Chinese pattern to match")
	}
	if decision.TargetBreeds[0] != "bianmu" {
		t.Errorf("expected bianmu, got %s", decision.TargetBreeds[0])
	}
}

func TestRouterDeduplication(t *testing.T) {
	t.Parallel()
	breeds := map[string]*config.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧", "@bianmu"}},
	}
	r := NewRouter(breeds)
	decision := r.Route("@边牧 @bianmu 分析")

	if len(decision.TargetBreeds) != 1 {
		t.Errorf("expected dedup to 1 target, got %d", len(decision.TargetBreeds))
	}
}

func TestRouterLongestFirstMatching(t *testing.T) {
	t.Parallel()
	breeds := map[string]*config.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@边牧"}},
	}
	// If "@边" were a pattern for another breed, it should NOT match before "@边牧"
	r := NewRouter(breeds)
	decision := r.Route("@边牧 分析")

	if decision.TargetBreeds[0] != "bianmu" {
		t.Errorf("expected bianmu, got %s", decision.TargetBreeds[0])
	}
}

func TestRouterCaseInsensitive(t *testing.T) {
	t.Parallel()
	breeds := map[string]*config.BreedConfig{
		"bianmu": {ID: "bianmu", MentionPatterns: []string{"@BianMu"}},
	}
	r := NewRouter(breeds)
	decision := r.Route("@bianmu 分析")

	if !decision.HasMentions {
		t.Error("expected case-insensitive match")
	}
}

func TestRouterEmptyPatterns(t *testing.T) {
	t.Parallel()
	r := NewRouter(map[string]*config.BreedConfig{})
	decision := r.Route("@边牧 分析")

	if decision.TargetBreeds[0] != "bianmu" {
		t.Errorf("expected default bianmu, got %s", decision.TargetBreeds[0])
	}
}
