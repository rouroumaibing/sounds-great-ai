package component

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestCLIModelGenerate(t *testing.T) {
	cfg := &ModelConfig{
		Type:      ProviderTypeCLI,
		ModelName: "test-model",
		CLIPath:   "echo",
		CLIArgs:   []string{"hello from CLI"},
	}

	ctx := context.Background()
	m, err := NewChatModel(ctx, cfg)
	if err != nil {
		t.Fatalf("NewChatModel failed: %v", err)
	}

	msgs := []*schema.Message{
		schema.UserMessage("test prompt"),
	}

	resp, err := m.Generate(ctx, msgs)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty response content")
	}
}

func TestCLIModelStream(t *testing.T) {
	cfg := &ModelConfig{
		Type:      ProviderTypeCLI,
		ModelName: "test-model",
		CLIPath:   "echo",
		CLIArgs:   []string{"line1\nline2\nline3"},
	}

	ctx := context.Background()
	m, err := NewChatModel(ctx, cfg)
	if err != nil {
		t.Fatalf("NewChatModel failed: %v", err)
	}

	msgs := []*schema.Message{
		schema.UserMessage("test prompt"),
	}

	reader, err := m.Stream(ctx, msgs)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	defer reader.Close()

	var chunks []*schema.Message
	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one chunk from stream")
	}
}

func TestNewChatModelUnknownProvider(t *testing.T) {
	cfg := &ModelConfig{
		Type:      "unknown",
		ModelName: "test",
	}

	_, err := NewChatModel(context.Background(), cfg)
	if err == nil {
		t.Error("expected error for unknown provider type")
	}
}
