package capability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/pack"
)

// TaskDecompose is a capability adapter that uses LLM to decompose tasks into subtasks.
type TaskDecompose struct {
	modelFactory func(ctx context.Context, cfg *component.ModelConfig) (model.BaseChatModel, error)
}

// NewTaskDecompose creates a new TaskDecompose capability.
func NewTaskDecompose() *TaskDecompose {
	return &TaskDecompose{modelFactory: component.NewChatModel}
}

func (t *TaskDecompose) Name() string    { return "task_decompose" }
func (t *TaskDecompose) Version() string { return "v1" }

func (t *TaskDecompose) Init(ctx context.Context) error { return nil }
func (t *TaskDecompose) Health() error                  { return nil }
func (t *TaskDecompose) Close() error                   { return nil }

const defaultAvailableBreeds = "bianmu,xigou,jinmao,demu,zangao,zhonghuatianyuanquan"

const taskDecomposeSystemSuffix = `
You are a task decomposition expert. Break the user's task into independent subtasks.
Available breeds and their capabilities:
- bianmu: task_decompose, agent_dispatch, result_merge (orchestrator)
- xigou: code_search, code_analyze, refactor_suggest (code hunter)
- jinmao: rag_search, context_assemble (retriever)
- demu: log_trace, error_diagnose (tracer)
- zangao: format_output, render_markdown (presenter)
- zhonghuatianyuanquan: command_check, path_validate (safety guard)

Output a JSON array. Each element:
{"title": "...", "description": "...", "suggest_breed": "...", "depends_on": ["sub-1"]}`

func (t *TaskDecompose) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	if input.Breed == nil {
		return nil, errors.New("task_decompose: breed config not available")
	}

	provider := input.Breed.ModelConfig.Provider
	apiKey := os.Getenv("MODEL_API_KEY_" + provider)
	if apiKey == "" {
		apiKey = os.Getenv("MODEL_API_KEY")
	}
	baseURL := os.Getenv("MODEL_BASE_URL_" + provider)
	if baseURL == "" {
		baseURL = os.Getenv("MODEL_BASE_URL")
	}

	cfg := &component.ModelConfig{
		Type:      component.ProviderTypeAPIKey,
		APIKey:    apiKey,
		BaseURL:   baseURL,
		ModelName: input.Breed.ModelConfig.Model,
	}

	mdl, err := t.modelFactory(ctx, cfg)
	if err != nil {
		return fallbackSubtasks(input), nil
	}

	userContent := input.Query
	if len(input.Previous) > 0 {
		userContent += "\n\nContext from previous steps:\n" + formatPreviousOutputs(input.Previous)
	}

	messages := []*schema.Message{
		schema.SystemMessage(input.Breed.SystemPrompt + taskDecomposeSystemSuffix),
		schema.UserMessage(userContent),
	}

	resp, err := mdl.Generate(ctx, messages)
	if err != nil {
		return fallbackSubtasks(input), nil
	}

	cleaned := sanitizeJSONResponse(resp.Content)
	var subtasks []SubTask
	if err := json.Unmarshal([]byte(cleaned), &subtasks); err != nil {
		return fallbackSubtasks(input), nil
	}

	// Assign IDs if missing
	for i := range subtasks {
		if subtasks[i].ID == "" {
			subtasks[i].ID = fmt.Sprintf("sub-%d", i+1)
		}
	}

	return &pack.TaskOutput{
		Control: true,
		Data: map[string]any{
			"subtasks": subtasks,
			"count":    len(subtasks),
		},
	}, nil
}

// fallbackSubtasks returns a single subtask with the original query.
func fallbackSubtasks(input *pack.TaskInput) *pack.TaskOutput {
	breedID := ""
	if input.Breed != nil {
		breedID = input.Breed.ID
	}
	st := []SubTask{{
		ID:           "sub-1",
		Title:        input.Query,
		Description:  input.Query,
		SuggestBreed: breedID,
	}}
	return &pack.TaskOutput{
		Control: true,
		Data: map[string]any{
			"subtasks": st,
			"count":    1,
		},
	}
}

// getAvailableBreeds resolves the list of available breed IDs from config or env.
func getAvailableBreeds(input *pack.TaskInput) map[string]bool {
	// 1. Try CapabilityConfig
	if input.CapabilityConfig != nil {
		if v, ok := input.CapabilityConfig["available_breeds"]; ok {
			if breeds, ok := v.([]any); ok {
				m := make(map[string]bool, len(breeds))
				for _, b := range breeds {
					if s, ok := b.(string); ok {
						m[s] = true
					}
				}
				if len(m) > 0 {
					return m
				}
			}
		}
	}
	// 2. Try env var
	if envBreeds := os.Getenv("AVAILABLE_BREEDS"); envBreeds != "" {
		m := make(map[string]bool)
		for _, b := range strings.Split(envBreeds, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				m[b] = true
			}
		}
		if len(m) > 0 {
			return m
		}
	}
	// 3. Default
	m := make(map[string]bool)
	for _, b := range strings.Split(defaultAvailableBreeds, ",") {
		m[b] = true
	}
	return m
}
