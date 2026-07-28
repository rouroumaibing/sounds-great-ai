package tool

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

type dummyInput struct {
	Value string `json:"value"`
}

type dummyOutput struct {
	Result string `json:"result"`
}

func TestToolRegistryRegisterAndGet(t *testing.T) {
	registry := NewToolRegistry()
	toolFunc := func(ctx context.Context, input dummyInput) (dummyOutput, error) {
		return dummyOutput{Result: input.Value}, nil
	}
	t1, err := utils.InferTool("test_tool", "A test tool", toolFunc)
	if err != nil {
		t.Fatalf("InferTool failed: %v", err)
	}
	registry.Register(t1)
	got, ok := registry.GetTool("test_tool")
	if !ok {
		t.Error("expected to find registered tool")
	}
	info, _ := got.Info(context.Background())
	if info.Name != "test_tool" {
		t.Errorf("tool name mismatch: got %s, want test_tool", info.Name)
	}
	_, ok = registry.GetTool("nonexistent")
	if ok {
		t.Error("expected not to find unregistered tool")
	}
}

func TestToolRegistryGetTools(t *testing.T) {
	registry := NewToolRegistry()
	toolFunc1 := func(ctx context.Context, input dummyInput) (dummyOutput, error) { return dummyOutput{}, nil }
	toolFunc2 := func(ctx context.Context, input dummyInput) (dummyOutput, error) { return dummyOutput{}, nil }
	t1, _ := utils.InferTool("tool_a", "Tool A", toolFunc1)
	t2, _ := utils.InferTool("tool_b", "Tool B", toolFunc2)
	registry.Register(t1)
	registry.Register(t2)
	tools := registry.GetTools()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestToolRegistryEmpty(t *testing.T) {
	registry := NewToolRegistry()
	tools := registry.GetTools()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
	_ = schema.SystemMessage("")
	_ = tool.BaseTool(nil)
}
