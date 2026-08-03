package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/pack"
)

// llmCallSpec describes a single LLM call with structured output + fallback.
// MaxInputChars is MANDATORY — callLLMWithFallback owns rune-safe truncation
// of UserContent to this budget. Callers must NOT pre-truncate.
type llmCallSpec struct {
	Breed         *pack.BreedConfig
	SystemSuffix  string
	UserContent   string
	MaxInputChars int
	Parse         func([]byte) (any, error)
	Fallback      func() any
}

// defaultModelFactory is the package-level model factory used by
// callLLMWithFallback. Tests override it via withModelFactory.
var defaultModelFactory = func(ctx context.Context, cfg *component.ModelConfig) (model.BaseChatModel, error) {
	return component.NewChatModel(ctx, cfg)
}

// withModelFactory temporarily replaces defaultModelFactory for a test body.
func withModelFactory(fn func(ctx context.Context, cfg *component.ModelConfig) (model.BaseChatModel, error), body func()) {
	prev := defaultModelFactory
	defaultModelFactory = fn
	defer func() { defaultModelFactory = prev }()
	body()
}

// callLLMWithFallback resolves provider config from Breed.ModelConfig + env,
// applies rune-safe truncation, calls LLM, sanitizeJSONResponse, Parse, and
// falls back on ANY failure (factory error, LLM error, JSON parse error).
// Never returns an error when Fallback is provided — callers rely on this
// to keep workflows alive.
func callLLMWithFallback(ctx context.Context, spec llmCallSpec) (any, error) {
	if spec.Fallback == nil {
		return nil, fmt.Errorf("llm_helper: Fallback is required")
	}

	userContent := truncateRunes(spec.UserContent, spec.MaxInputChars)

	mdl, err := newModelFromBreed(ctx, spec.Breed)
	if err != nil {
		return spec.Fallback(), nil
	}

	systemPrompt := ""
	if spec.Breed != nil {
		systemPrompt = spec.Breed.SystemPrompt
	}
	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt + spec.SystemSuffix),
		schema.UserMessage(userContent),
	}

	resp, err := mdl.Generate(ctx, messages)
	if err != nil {
		return spec.Fallback(), nil
	}

	cleaned := sanitizeJSONResponse(resp.Content)
	parsed, err := spec.Parse([]byte(cleaned))
	if err != nil {
		return spec.Fallback(), nil
	}
	return parsed, nil
}

// newModelFromBreed resolves provider/apikey/baseurl from breed config + env.
func newModelFromBreed(ctx context.Context, breed *pack.BreedConfig) (model.BaseChatModel, error) {
	if breed == nil {
		return nil, fmt.Errorf("breed config nil")
	}
	provider := breed.ModelConfig.Provider
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
		ModelName: breed.ModelConfig.Model,
	}
	return defaultModelFactory(ctx, cfg)
}

// truncateRunes returns s truncated to at most max runes, UTF-8 safe.
// If s has ≤ max runes, returned unchanged.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// jsonMarshalHelper is exported for tests that need to encode/decode structured Data.
func jsonMarshalHelper(v any) ([]byte, error) {
	return json.Marshal(v)
}
