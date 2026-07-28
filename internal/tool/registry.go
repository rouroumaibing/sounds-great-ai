package tool

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
)

type ToolRegistry struct {
	tools map[string]tool.BaseTool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]tool.BaseTool)}
}

func (r *ToolRegistry) Register(t tool.BaseTool) {
	info, err := t.Info(context.Background())
	if err != nil || info == nil {
		return
	}
	r.tools[info.Name] = t
}

func (r *ToolRegistry) GetTools() []tool.BaseTool {
	result := make([]tool.BaseTool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *ToolRegistry) GetTool(name string) (tool.BaseTool, bool) {
	t, ok := r.tools[name]
	return t, ok
}
