package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/pack"
)

func newMockTaskDecompose(mock *mockChatModel) *TaskDecompose {
	return &TaskDecompose{modelFactory: func(ctx context.Context, cfg *component.ModelConfig) (model.BaseChatModel, error) {
		return mock, nil
	}}
}

func TestTaskDecomposeBasic(t *testing.T) {
	t.Parallel()
	mockModel := &mockChatModel{response: `[{"title":"Search code","description":"Find relevant files","suggest_breed":"xigou","depends_on":[]},{"title":"Analyze","description":"Analyze results","suggest_breed":"xigou","depends_on":["sub-1"]}]`}
	td := newMockTaskDecompose(mockModel)

	out, err := td.Run(context.Background(), &pack.TaskInput{
		Query: "find and analyze code",
		Breed: &pack.BreedConfig{
			SystemPrompt: "you are an orchestrator",
			ModelConfig:  pack.ModelConfig{Provider: "openai", Model: "gpt-4"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	subtasks, ok := out.Data["subtasks"]
	if !ok {
		t.Fatal("subtasks not in output Data")
	}
	var tasks []SubTask
	if err := decodeData(subtasks, &tasks); err != nil {
		t.Fatalf("decode subtasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 subtasks, got %d", len(tasks))
	}
}

func TestTaskDecomposeNilBreed(t *testing.T) {
	t.Parallel()
	mockModel := &mockChatModel{}
	td := newMockTaskDecompose(mockModel)

	_, err := td.Run(context.Background(), &pack.TaskInput{Query: "test"})
	if err == nil {
		t.Fatal("expected error for nil breed")
	}
}

func TestTaskDecomposeLLMError(t *testing.T) {
	t.Parallel()
	td := &TaskDecompose{modelFactory: func(ctx context.Context, cfg *component.ModelConfig) (model.BaseChatModel, error) {
		return nil, errors.New("model creation failed")
	}}

	out, err := td.Run(context.Background(), &pack.TaskInput{
		Query: "test",
		Breed: &pack.BreedConfig{
			SystemPrompt: "test",
			ModelConfig:  pack.ModelConfig{Provider: "openai", Model: "gpt-4"},
		},
	})
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	var tasks []SubTask
	if err := decodeData(out.Data["subtasks"], &tasks); err != nil {
		t.Fatalf("decode fallback subtasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 fallback subtask, got %d", len(tasks))
	}
}

func TestTaskDecomposeInvalidJSON(t *testing.T) {
	t.Parallel()
	mockModel := &mockChatModel{response: "this is not JSON at all"}
	td := newMockTaskDecompose(mockModel)

	out, err := td.Run(context.Background(), &pack.TaskInput{
		Query: "test",
		Breed: &pack.BreedConfig{
			SystemPrompt: "test",
			ModelConfig:  pack.ModelConfig{Provider: "openai", Model: "gpt-4"},
		},
	})
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	var tasks []SubTask
	if err := decodeData(out.Data["subtasks"], &tasks); err != nil {
		t.Fatalf("decode fallback subtasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 fallback subtask, got %d", len(tasks))
	}
}

func TestTaskDecomposeMarkdownWrapped(t *testing.T) {
	t.Parallel()
	mockModel := &mockChatModel{response: "```json\n[{\"title\":\"Task A\",\"description\":\"Do A\",\"suggest_breed\":\"xigou\",\"depends_on\":[]}]\n```"}
	td := newMockTaskDecompose(mockModel)

	out, err := td.Run(context.Background(), &pack.TaskInput{
		Query: "test",
		Breed: &pack.BreedConfig{
			SystemPrompt: "test",
			ModelConfig:  pack.ModelConfig{Provider: "openai", Model: "gpt-4"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var tasks []SubTask
	if err := decodeData(out.Data["subtasks"], &tasks); err != nil {
		t.Fatalf("decode subtasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Task A" {
		t.Errorf("expected 1 subtask 'Task A', got %+v", tasks)
	}
}
