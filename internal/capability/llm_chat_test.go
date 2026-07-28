package capability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/pack"
)

// mockChatModel implements model.BaseChatModel for testing.
// It captures the last user message content and returns a fixed response.
type mockChatModel struct {
	lastContent string
	response    string
}

func (m *mockChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	for i := len(input) - 1; i >= 0; i-- {
		if input[i].Role == schema.User {
			m.lastContent = input[i].Content
			break
		}
	}
	resp := m.response
	if resp == "" {
		resp = "mock response"
	}
	return schema.AssistantMessage(resp, nil), nil
}

func (m *mockChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream not supported")
}

func (m *mockChatModel) BindTools(tools []*schema.ToolInfo) error {
	return nil
}

func newMockLLMChat(mock *mockChatModel) *LLMChat {
	return &LLMChat{modelFactory: func(ctx context.Context, cfg *component.ModelConfig) (model.BaseChatModel, error) {
		return mock, nil
	}}
}

func TestLLMChatBasic(t *testing.T) {
	mockModel := &mockChatModel{response: "mock response"}
	l := newMockLLMChat(mockModel)

	out, err := l.Run(context.Background(), &pack.TaskInput{
		Query: "hello",
		Breed: &pack.BreedConfig{
			SystemPrompt: "you are helpful",
			ModelConfig: pack.ModelConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	resp, ok := out.Data["response"].(string)
	if !ok {
		t.Fatalf("response is not string: %T", out.Data["response"])
	}
	if resp != "mock response" {
		t.Errorf("response = %q, want %q", resp, "mock response")
	}

	modelName, ok := out.Data["model"].(string)
	if !ok {
		t.Fatalf("model is not string: %T", out.Data["model"])
	}
	if modelName != "gpt-4" {
		t.Errorf("model = %q, want %q", modelName, "gpt-4")
	}
}

func TestLLMChatNilBreed(t *testing.T) {
	mockModel := &mockChatModel{}
	l := newMockLLMChat(mockModel)

	_, err := l.Run(context.Background(), &pack.TaskInput{
		Query: "hello",
	})
	if err == nil {
		t.Fatal("expected error for nil breed, got nil")
	}
}

func TestLLMChatPreviousContext(t *testing.T) {
	mockModel := &mockChatModel{response: "mock response"}
	l := newMockLLMChat(mockModel)

	prev := map[string]*pack.TaskOutput{
		"step1": {
			Reason: "found results",
			Data: map[string]any{
				"count": 1,
			},
		},
	}

	_, err := l.Run(context.Background(), &pack.TaskInput{
		Query:    "hello",
		Previous: prev,
		Breed: &pack.BreedConfig{
			SystemPrompt: "you are helpful",
			ModelConfig: pack.ModelConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(mockModel.lastContent, "Context from previous steps") {
		t.Errorf("user content does not contain 'Context from previous steps': %q", mockModel.lastContent)
	}
}

func TestLLMChatTruncation(t *testing.T) {
	mockModel := &mockChatModel{response: "mock response"}
	l := newMockLLMChat(mockModel)

	prev := make(map[string]*pack.TaskOutput)
	for i := 0; i < 200; i++ {
		prev[fmt.Sprintf("step%d", i)] = &pack.TaskOutput{
			Reason: strings.Repeat("x", 100),
		}
	}

	_, err := l.Run(context.Background(), &pack.TaskInput{
		Query:    "hello",
		Previous: prev,
		Breed: &pack.BreedConfig{
			SystemPrompt: "you are helpful",
			ModelConfig: pack.ModelConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(mockModel.lastContent, "Truncated") {
		t.Errorf("user content does not contain 'Truncated' marker: length=%d", len(mockModel.lastContent))
	}
}
