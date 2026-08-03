package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/pack"
)

func TestCodeAnalyze_Name(t *testing.T) {
	c := NewCodeAnalyze()
	if c.Name() != "code_analyze" || c.Version() != "v1" {
		t.Fatalf("name/version: %q/%q", c.Name(), c.Version())
	}
}

func TestCodeAnalyze_Run_EmptyMatches(t *testing.T) {
	c := NewCodeAnalyze()
	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"search": {Data: map[string]any{"matches": []SearchMatch{}}},
		},
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	ar, ok := out.Data["analysis"].(AnalysisResult)
	if !ok {
		t.Fatalf("analysis wrong type: %T", out.Data["analysis"])
	}
	if len(ar.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(ar.Findings))
	}
}

func TestCodeAnalyze_Run_MissingSearch(t *testing.T) {
	c := NewCodeAnalyze()
	input := &pack.TaskInput{Previous: map[string]*pack.TaskOutput{}}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	ar, ok := out.Data["analysis"].(AnalysisResult)
	if !ok {
		t.Fatalf("analysis wrong type: %T", out.Data["analysis"])
	}
	if ar.Summary != "" {
		t.Fatalf("expected empty summary, got %q", ar.Summary)
	}
}

func TestCodeAnalyze_Run_LLMFailsToFallback(t *testing.T) {
	c := NewCodeAnalyze()
	matches := []SearchMatch{
		{File: "foo.go", Line: 10, Content: "func foo() {}"},
		{File: "bar.go", Line: 20, Content: "func bar() {}"},
	}
	input := &pack.TaskInput{
		Query: "find callers",
		Breed: &pack.BreedConfig{ModelConfig: pack.ModelConfig{Provider: "test", Model: "m"}},
		Previous: map[string]*pack.TaskOutput{
			"search": {Data: map[string]any{"matches": matches}},
		},
	}

	// Inject failing model factory
	withModelFactory(func(_ context.Context, _ *component.ModelConfig) (model.BaseChatModel, error) {
		return nil, errors.New("llm unavailable")
	}, func() {
		out, err := c.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		ar, ok := out.Data["analysis"].(AnalysisResult)
		if !ok {
			t.Fatalf("analysis wrong type: %T", out.Data["analysis"])
		}
		if len(ar.Findings) != 2 {
			t.Fatalf("fallback should produce 2 findings (one per match), got %d", len(ar.Findings))
		}
		if ar.Findings[0].File != "foo.go" {
			t.Fatalf("first finding file: %q", ar.Findings[0].File)
		}
	})
}
