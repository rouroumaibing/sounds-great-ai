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

// ResultMerge is a capability adapter that uses LLM to synthesize breed results.
type ResultMerge struct {
	modelFactory func(ctx context.Context, cfg *component.ModelConfig) (model.BaseChatModel, error)
}

// NewResultMerge creates a new ResultMerge capability.
func NewResultMerge() *ResultMerge {
	return &ResultMerge{modelFactory: component.NewChatModel}
}

func (r *ResultMerge) Name() string    { return "result_merge" }
func (r *ResultMerge) Version() string { return "v1" }

func (r *ResultMerge) Init(ctx context.Context) error { return nil }
func (r *ResultMerge) Health() error                  { return nil }
func (r *ResultMerge) Close() error                   { return nil }

const resultMergeSystemSuffix = `
You are a result synthesis expert. Integrate multiple breed execution results into a coherent final answer.
Preserve key information from each section, remove redundancy, organize in logical order.
Output JSON: {"summary": "...", "sections": [{"breed_id": "...", "title": "...", "content": "..."}]}`

func (r *ResultMerge) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	if input.Breed == nil {
		return nil, errors.New("result_merge: breed config not available")
	}

	// Collect breed results
	breedResults := collectBreedResults(input)

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

	mdl, err := r.modelFactory(ctx, cfg)
	if err != nil {
		return fallbackMergeResult(breedResults), nil
	}

	// Build user content with balanced truncation
	truncated := balancedTruncate(breedResults, maxPreviousContextLen)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Original task: %s\n\nBreed execution results:\n", input.Query)
	for breed, content := range truncated {
		fmt.Fprintf(&sb, "--- breed: %s ---\n%s\n\n", breed, content)
	}

	messages := []*schema.Message{
		schema.SystemMessage(input.Breed.SystemPrompt + resultMergeSystemSuffix),
		schema.UserMessage(sb.String()),
	}

	resp, err := mdl.Generate(ctx, messages)
	if err != nil {
		return fallbackMergeResult(breedResults), nil
	}

	cleaned := sanitizeJSONResponse(resp.Content)
	var result MergeResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return fallbackMergeResult(breedResults), nil
	}

	return &pack.TaskOutput{
		Data: map[string]any{
			"merge_result": result,
		},
	}, nil
}

// collectBreedResults gathers results from Previous or CapabilityConfig.
// Priority:
//  1. Previous["execute"].Data["dispatched_executions"] — structural, by breed_id
//  2. CapabilityConfig["breed_results"] — test injection
//  3. Previous scan — skip Control=true steps (not stepID string match)
func collectBreedResults(input *pack.TaskInput) map[string]string {
	results := make(map[string]string)

	// 1. Read dispatched_executions structurally
	if input.Previous != nil {
		if execOut, ok := input.Previous["execute"]; ok && execOut != nil {
			if execs, ok := execOut.Data["dispatched_executions"]; ok {
				var breedExecs []BreedExecution
				if decodeData(execs, &breedExecs) == nil {
					for _, be := range breedExecs {
						if be.Output == nil {
							continue
						}
						results[be.BreedID] = formatPreviousOutputs(map[string]*pack.TaskOutput{be.BreedID: be.Output})
					}
				}
			}
		}
	}

	// 2. CapabilityConfig for injected results (test path)
	if len(results) == 0 && input.CapabilityConfig != nil {
		if v, ok := input.CapabilityConfig["breed_results"]; ok {
			if m, ok := v.(map[string]any); ok {
				for breed, val := range m {
					if s, ok := val.(string); ok {
						results[breed] = s
					} else {
						b, _ := json.Marshal(val)
						results[breed] = string(b)
					}
				}
			}
		}
	}

	// 3. Previous scan fallback — skip Control=true steps
	if len(results) == 0 && input.Previous != nil {
		for stepID, out := range input.Previous {
			if out == nil || out.Control {
				continue
			}
			results[stepID] = formatPreviousOutputs(map[string]*pack.TaskOutput{stepID: out})
		}
	}

	return results
}

// fallbackMergeResult returns a raw concatenation when LLM fails.
func fallbackMergeResult(results map[string]string) *pack.TaskOutput {
	if len(results) == 0 {
		return &pack.TaskOutput{
			Data: map[string]any{
				"merge_result": MergeResult{
					Summary: "No results to merge",
				},
			},
		}
	}

	var sections []MergeSection
	var sb strings.Builder
	for breed, content := range results {
		sections = append(sections, MergeSection{
			BreedID: breed,
			Title:   breed,
			Content: content,
		})
		sb.WriteString(content + "\n")
	}
	return &pack.TaskOutput{
		Data: map[string]any{
			"merge_result": MergeResult{
				Summary:  sb.String(),
				Sections: sections,
			},
		},
	}
}
