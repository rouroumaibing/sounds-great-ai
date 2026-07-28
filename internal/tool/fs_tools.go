package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/workspace"
)

type ReadFileInput struct {
	Path string `json:"path" jsonschema:"description=文件相对路径"`
}

type EditFileInput struct {
	Path    string `json:"path" jsonschema:"description=文件相对路径"`
	Content string `json:"content" jsonschema:"description=要写入或覆盖的代码内容"`
}

type ListDirInput struct {
	Path string `json:"path" jsonschema:"description=目录相对路径，空字符串表示根目录"`
}

type fileResult struct {
	Message string `json:"message"`
}

const maxFileSize = 32 * 1024

func NewReadFileTool(wm *workspace.WorkspaceManager) *fileToolImpl {
	t, _ := utils.InferTool(
		"read_file",
		"读取工作区中的文件内容，超过32KB会被截断",
		func(ctx context.Context, input ReadFileInput) (fileResult, error) {
			resolved, err := wm.ResolvePath(input.Path)
			if err != nil {
				return fileResult{}, err
			}
			content, err := os.ReadFile(resolved)
			if err != nil {
				return fileResult{}, fmt.Errorf("读取文件失败: %w", err)
			}
			if len(content) > maxFileSize {
				content = content[:maxFileSize]
				return fileResult{Message: string(content) + "\n...[truncated]"}, nil
			}
			return fileResult{Message: string(content)}, nil
		},
	)
	return &fileToolImpl{tool: t}
}

func NewEditFileTool(wm *workspace.WorkspaceManager) *fileToolImpl {
	t, _ := utils.InferTool(
		"edit_file",
		"写入或覆盖工作区中的文件",
		func(ctx context.Context, input EditFileInput) (fileResult, error) {
			resolved, err := wm.ResolvePath(input.Path)
			if err != nil {
				return fileResult{}, err
			}
			dir := filepath.Dir(resolved)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fileResult{}, fmt.Errorf("创建目录失败: %w", err)
			}
			if err := os.WriteFile(resolved, []byte(input.Content), 0644); err != nil {
				return fileResult{}, fmt.Errorf("写入文件失败: %w", err)
			}
			return fileResult{Message: fmt.Sprintf("文件 %s 已写入", input.Path)}, nil
		},
	)
	return &fileToolImpl{tool: t}
}

func NewListDirTool(wm *workspace.WorkspaceManager) *fileToolImpl {
	t, _ := utils.InferTool(
		"list_dir",
		"列出工作区目录内容",
		func(ctx context.Context, input ListDirInput) (fileResult, error) {
			resolved, err := wm.ResolvePath(input.Path)
			if err != nil {
				return fileResult{}, err
			}
			entries, err := os.ReadDir(resolved)
			if err != nil {
				return fileResult{}, fmt.Errorf("读取目录失败: %w", err)
			}
			var sb strings.Builder
			for _, entry := range entries {
				if entry.IsDir() {
					sb.WriteString("dir:  ")
				} else {
					sb.WriteString("file: ")
				}
				sb.WriteString(entry.Name())
				sb.WriteString("\n")
			}
			return fileResult{Message: sb.String()}, nil
		},
	)
	return &fileToolImpl{tool: t}
}

type fileToolImpl struct {
	tool tool.InvokableTool
}

func (f *fileToolImpl) InvokableRun(ctx context.Context, args string) (string, error) {
	return f.tool.InvokableRun(ctx, args)
}

func (f *fileToolImpl) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return f.tool.Info(ctx)
}
