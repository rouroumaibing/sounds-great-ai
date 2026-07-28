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

// LLMChat is a capability adapter that performs LLM conversation via Eino ChatModel.
type LLMChat struct {
	modelFactory func(ctx context.Context, cfg *component.ModelConfig) (model.BaseChatModel, error)
}

// NewLLMChat creates a new LLMChat capability.
func NewLLMChat() *LLMChat {
	return &LLMChat{modelFactory: component.NewChatModel}
}

func (l *LLMChat) Name() string    { return "llm_chat" }
func (l *LLMChat) Version() string { return "v1" }

func (l *LLMChat) Init(ctx context.Context) error { return nil }
func (l *LLMChat) Health() error                  { return nil }
func (l *LLMChat) Close() error                   { return nil }

// Run executes the LLM chat capability.
func (l *LLMChat) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	if input.Breed == nil {
		return nil, errors.New("llm_chat: breed config not available")
	}

	provider := input.Breed.ModelConfig.Provider

	// Resolve API key: try provider-specific env var, fallback to generic.
	apiKey := os.Getenv("MODEL_API_KEY_" + provider)
	if apiKey == "" {
		apiKey = os.Getenv("MODEL_API_KEY")
	}

	// Resolve base URL: try provider-specific env var, fallback to generic.
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

	mdl, err := l.modelFactory(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("llm_chat: create model failed: %w", err)
	}

	// Build user content, appending previous step context if available.
	userContent := input.Query
	if len(input.Previous) > 0 {
		userContent += "\n\nContext from previous steps:\n" + formatPreviousOutputs(input.Previous)
	}

	messages := []*schema.Message{
		schema.SystemMessage(input.Breed.SystemPrompt),
		schema.UserMessage(userContent),
	}

	resp, err := mdl.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm_chat: generate failed: %w", err)
	}

	return &pack.TaskOutput{
		Data: map[string]any{
			"response": resp.Content,
			"model":    input.Breed.ModelConfig.Model,
		},
	}, nil
}

const maxPreviousContextLen = 12000

// formatPreviousOutputs serializes previous step outputs into a readable context string.
func formatPreviousOutputs(prev map[string]*pack.TaskOutput) string {
	var sb strings.Builder
	for id, out := range prev {
		fmt.Fprintf(&sb, "## Step: %s\n", id)
		if out != nil {
			fmt.Fprintf(&sb, "Reason: %s\n", out.Reason)
			if len(out.Data) > 0 {
				b, _ := json.MarshalIndent(out.Data, "", "  ")
				fmt.Fprintf(&sb, "```json\n%s\n```\n", string(b))
			} else if len(out.Results) > 0 {
				b, _ := json.MarshalIndent(out.Results, "", "  ")
				fmt.Fprintf(&sb, "```json\n%s\n```\n", string(b))
			}
		}
	}
	if sb.Len() > maxPreviousContextLen {
		return sb.String()[:maxPreviousContextLen] + "... [Truncated due to context limit]"
	}
	return sb.String()
}
