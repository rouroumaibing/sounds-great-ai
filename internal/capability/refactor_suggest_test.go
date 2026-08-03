package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/pack"
)

func TestRefactorSuggest_Name(t *testing.T) {
	c := NewRefactorSuggest()
	if c.Name() != "refactor_suggest" || c.Version() != "v1" {
		t.Fatalf("name/version: %q/%q", c.Name(), c.Version())
	}
}

func TestRefactorSuggest_Run_EmptyAnalysis(t *testing.T) {
	c := NewRefactorSuggest()
	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"analyze": {Data: map[string]any{"analysis": AnalysisResult{}}},
		},
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rr, ok := out.Data["refactor"].(RefactorResult)
	if !ok {
		t.Fatalf("refactor wrong type: %T", out.Data["refactor"])
	}
	if rr.Summary != "no refactor needed" {
		t.Fatalf("expected 'no refactor needed', got %q", rr.Summary)
	}
}

func TestRefactorSuggest_Run_LLMFailsToFallback(t *testing.T) {
	c := NewRefactorSuggest()
	ar := AnalysisResult{
		Summary: "module has 3 callers",
		Findings: []Finding{
			{Type: "call", File: "a.go", Line: 1, Severity: "medium", Description: "tight coupling"},
		},
	}
	input := &pack.TaskInput{
		Query: "refactor suggestions",
		Breed: &pack.BreedConfig{ModelConfig: pack.ModelConfig{Provider: "test", Model: "m"}},
		Previous: map[string]*pack.TaskOutput{
			"analyze": {Data: map[string]any{"analysis": ar}},
		},
	}

	withModelFactory(func(_ context.Context, _ *component.ModelConfig) (model.BaseChatModel, error) {
		return nil, errors.New("llm unavailable")
	}, func() {
		out, err := c.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		rr, ok := out.Data["refactor"].(RefactorResult)
		if !ok {
			t.Fatalf("refactor wrong type: %T", out.Data["refactor"])
		}
		if len(rr.Suggestions) != 1 {
			t.Fatalf("expected 1 fallback suggestion, got %d", len(rr.Suggestions))
		}
		if rr.Suggestions[0].File != "a.go" {
			t.Fatalf("suggestion file: %q", rr.Suggestions[0].File)
		}
	})
}
