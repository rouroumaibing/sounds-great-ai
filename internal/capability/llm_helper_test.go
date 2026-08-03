package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/pack"
)

func TestTruncateRunes_UnderLimit(t *testing.T) {
	in := "hello"
	got := truncateRunes(in, 10)
	if got != in {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestTruncateRunes_OverLimit(t *testing.T) {
	in := "abcdefghij" // 10 runes
	got := truncateRunes(in, 5)
	if got != "abcde" {
		t.Fatalf("expected abcde, got %q", got)
	}
}

func TestTruncateRunes_UTF8Safe(t *testing.T) {
	// 6 chinese chars = 6 runes, 18 bytes
	in := "你好世界你好"
	got := truncateRunes(in, 3)
	if got != "你好世" {
		t.Fatalf("expected 你好世, got %q", got)
	}
}

func TestTruncateRunes_Empty(t *testing.T) {
	if got := truncateRunes("", 10); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// --- callLLMWithFallback tests ---

func TestCallLLMWithFallback_FallbackOnFactoryError(t *testing.T) {
	called := false
	spec := llmCallSpec{
		Breed:         &pack.BreedConfig{ModelConfig: pack.ModelConfig{Provider: "test", Model: "m"}},
		SystemSuffix:  "",
		UserContent:   "x",
		MaxInputChars: 100,
		Parse: func(b []byte) (any, error) {
			t.Fatal("Parse should not be called on factory error")
			return nil, nil
		},
		Fallback: func() any {
			called = true
			return "fallback-value"
		},
	}
	// Inject a failing factory via package-level override (see llm_helper.go)
	withModelFactory(func(_ context.Context, _ *component.ModelConfig) (model.BaseChatModel, error) {
		return nil, errors.New("factory boom")
	}, func() {
		got, err := callLLMWithFallback(context.Background(), spec)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got != "fallback-value" {
			t.Fatalf("expected fallback-value, got %v", got)
		}
		if !called {
			t.Fatal("fallback not called")
		}
	})
}

func TestCallLLMWithFallback_TruncatesInput(t *testing.T) {
	long := make([]rune, 5000)
	for i := range long {
		long[i] = 'x'
	}
	captured := ""
	withModelFactory(func(_ context.Context, _ *component.ModelConfig) (model.BaseChatModel, error) {
		return &fakeModel{generateFn: func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error) {
			// Capture the user message length
			if len(msgs) > 1 {
				captured = msgs[1].Content
			}
			return &schema.Message{Content: "{}"}, nil
		}}, nil
	}, func() {
		spec := llmCallSpec{
			Breed:         &pack.BreedConfig{ModelConfig: pack.ModelConfig{Provider: "test", Model: "m"}},
			UserContent:   string(long),
			MaxInputChars: 100,
			Parse:         func(b []byte) (any, error) { return "parsed", nil },
			Fallback:      func() any { return "fallback" },
		}
		_, _ = callLLMWithFallback(context.Background(), spec)
	})
	if len([]rune(captured)) > 100 {
		t.Fatalf("expected ≤100 runes, got %d", len([]rune(captured)))
	}
}

// fakeModel implements model.BaseChatModel for testing.
// The BaseChatModel interface (eino v0.9.13) requires only Generate and Stream;
// BindTools is part of the deprecated ChatModel interface and is NOT needed here.
type fakeModel struct {
	generateFn func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error)
}

func (f *fakeModel) Generate(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if f.generateFn != nil {
		return f.generateFn(ctx, msgs)
	}
	return &schema.Message{Content: "{}"}, nil
}

func (f *fakeModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not implemented")
}
