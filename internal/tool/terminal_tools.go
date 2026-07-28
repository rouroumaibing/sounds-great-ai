package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool/utils"
	"sounds-great-ai/internal/workspace"
)

// RunCommandInput 是 run_command 工具的输入
type RunCommandInput struct {
	Command string `json:"command" jsonschema:"description=要执行的 shell 命令"`
	Timeout int    `json:"timeout" jsonschema:"description=超时时间（秒），默认30"`
}

// commandResult 是命令执行的返回
type commandResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}

// NewRunCommandTool 创建运行命令工具
func NewRunCommandTool(executor *workspace.PTYExecutor) *fileToolImpl {
	t, _ := utils.InferTool(
		"run_command",
		"在工作区中执行 shell 命令，返回输出结果",
		func(ctx context.Context, input RunCommandInput) (commandResult, error) {
			timeout := input.Timeout
			if timeout <= 0 {
				timeout = 30
			}

			cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()

			outputCh, err := executor.Execute(cmdCtx, input.Command)
			if err != nil {
				return commandResult{}, fmt.Errorf("命令执行失败: %w", err)
			}

			var sb strings.Builder
			for out := range outputCh {
				sb.WriteString(out.Data)
			}

			exitCode := 0
			if cmdCtx.Err() == context.DeadlineExceeded {
				exitCode = -1
			}

			return commandResult{
				Output:   sb.String(),
				ExitCode: exitCode,
			}, nil
		},
	)
	return &fileToolImpl{tool: t}
}
