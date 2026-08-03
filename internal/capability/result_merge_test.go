package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/pack"
)

func newMockResultMerge(mock *mockChatModel) *ResultMerge {
	return &ResultMerge{modelFactory: func(ctx context.Context, cfg *component.ModelConfig) (model.BaseChatModel, error) {
		return mock, nil
	}}
}

func TestResultMergeBasic(t *testing.T) {
	t.Parallel()
	mockModel := &mockChatModel{response: `{"summary":"Combined result","sections":[{"breed_id":"xigou","title":"Code Search","content":"Found 3 files"}]}`}
	rm := newMockResultMerge(mockModel)

	out, err := rm.Run(context.Background(), &pack.TaskInput{
		Query: "find and analyze code",
		Previous: map[string]*pack.TaskOutput{
			"dispatch":    {Data: map[string]any{"dispatch_plan": DispatchPlan{}}},
			"breed_xigou": {Data: map[string]any{"response": "Found 3 files"}},
		},
		Breed: &pack.BreedConfig{
			SystemPrompt: "you are an orchestrator",
			ModelConfig:  pack.ModelConfig{Provider: "openai", Model: "gpt-4"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	resultAny, ok := out.Data["merge_result"]
	if !ok {
		t.Fatal("merge_result not in output")
	}
	var result MergeResult
	if err := decodeData(resultAny, &result); err != nil {
		t.Fatalf("decode merge_result: %v", err)
	}
	if result.Summary != "Combined result" {
		t.Errorf("summary = %q", result.Summary)
	}
}

func TestResultMergeNilBreed(t *testing.T) {
	t.Parallel()
	mockModel := &mockChatModel{}
	rm := newMockResultMerge(mockModel)

	_, err := rm.Run(context.Background(), &pack.TaskInput{Query: "test"})
	if err == nil {
		t.Fatal("expected error for nil breed")
	}
}

func TestResultMergeEmptyResults(t *testing.T) {
	t.Parallel()
	mockModel := &mockChatModel{response: "mock"}
	rm := newMockResultMerge(mockModel)

	out, err := rm.Run(context.Background(), &pack.TaskInput{
		Query: "test",
		Previous: map[string]*pack.TaskOutput{
			"dispatch": {Data: map[string]any{"dispatch_plan": DispatchPlan{}}},
		},
		Breed: &pack.BreedConfig{
			SystemPrompt: "test",
			ModelConfig:  pack.ModelConfig{Provider: "openai", Model: "gpt-4"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var result MergeResult
	if err := decodeData(out.Data["merge_result"], &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Summary == "" {
		t.Error("expected non-empty fallback summary")
	}
}

func TestResultMergeBalancedTruncation(t *testing.T) {
	t.Parallel()
	// Test that balancedTruncate gives each breed equal budget
	results := map[string]string{
		"a": string(make([]byte, 5000)),
		"b": string(make([]byte, 5000)),
		"c": string(make([]byte, 5000)),
	}
	truncated := balancedTruncate(results, 3000) // perBreed = 1000
	for breed, content := range truncated {
		runes := []rune(content)
		if len(runes) > 1100 { // 1000 + truncation marker
			t.Errorf("breed %s not properly truncated: %d runes", breed, len(runes))
		}
	}
}

// Ensure errors import is used
var _ = errors.New

func TestCollectBreedResults_ReadsDispatchedExecutions(t *testing.T) {
	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"execute": {
				Control: true,
				Data: map[string]any{
					"dispatched_executions": []BreedExecution{
						{
							SubTaskID: "sub-1",
							BreedID:   "xigou",
							Output:    &pack.TaskOutput{Data: map[string]any{"findings": "found user module"}},
						},
						{
							SubTaskID: "sub-2",
							BreedID:   "demu",
							Output:    &pack.TaskOutput{Data: map[string]any{"trace": "panic trace"}},
						},
					},
				},
			},
		},
	}

	results := collectBreedResults(input)
	if len(results) != 2 {
		t.Fatalf("expected 2 breed results, got %d: %v", len(results), results)
	}
	if _, ok := results["xigou"]; !ok {
		t.Fatal("missing xigou")
	}
	if _, ok := results["demu"]; !ok {
		t.Fatal("missing demu")
	}
}

func TestCollectBreedResults_SkipsControlFlaggedSteps(t *testing.T) {
	// Even with a custom stepID (user renamed), Control flag must skip it.
	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"my_custom_decompose": {
				Control: true,
				Data:    map[string]any{"subtasks": "stuff"},
			},
			"breed_step": {
				Data: map[string]any{"result": "real"},
			},
		},
	}

	results := collectBreedResults(input)
	if _, ok := results["my_custom_decompose"]; ok {
		t.Fatal("Control=true step should be skipped")
	}
	if _, ok := results["breed_step"]; !ok {
		t.Fatal("non-control step should be included")
	}
}
