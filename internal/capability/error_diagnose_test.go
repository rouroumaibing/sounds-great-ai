package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/pack"
)

func TestErrorDiagnose_Name(t *testing.T) {
	c := NewErrorDiagnose()
	if c.Name() != "error_diagnose" || c.Version() != "v1" {
		t.Fatalf("name/version: %q/%q", c.Name(), c.Version())
	}
}

func TestErrorDiagnose_Run_EmptyTrace(t *testing.T) {
	c := NewErrorDiagnose()
	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"trace": {Data: map[string]any{"trace": TraceResult{}}},
		},
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	dr, ok := out.Data["diagnosis"].(DiagnosisResult)
	if !ok {
		t.Fatalf("diagnosis wrong type: %T", out.Data["diagnosis"])
	}
	if dr.RootCause != "no errors to diagnose" {
		t.Fatalf("expected 'no errors to diagnose', got %q", dr.RootCause)
	}
}

func TestErrorDiagnose_Run_LLMFailsToFallback(t *testing.T) {
	c := NewErrorDiagnose()
	tr := TraceResult{
		Matches: []LogEntry{
			{File: "app.log", Line: 42, Level: "ERROR", Content: "panic: nil pointer"},
		},
	}
	input := &pack.TaskInput{
		Breed: &pack.BreedConfig{ModelConfig: pack.ModelConfig{Provider: "test", Model: "m"}},
		Previous: map[string]*pack.TaskOutput{
			"trace": {Data: map[string]any{"trace": tr}},
		},
	}

	withModelFactory(func(_ context.Context, _ *component.ModelConfig) (model.BaseChatModel, error) {
		return nil, errors.New("llm unavailable")
	}, func() {
		out, err := c.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		dr, ok := out.Data["diagnosis"].(DiagnosisResult)
		if !ok {
			t.Fatalf("diagnosis wrong type: %T", out.Data["diagnosis"])
		}
		if dr.RootCause == "" {
			t.Fatal("fallback root_cause should not be empty")
		}
		if !contains(dr.RootCause, "panic: nil pointer") {
			t.Fatalf("expected fallback to include raw trace, got %q", dr.RootCause)
		}
	})
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && stringContains(s, sub)))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
