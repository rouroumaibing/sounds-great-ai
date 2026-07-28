package capability

import (
	"context"

	"sounds-great-ai/internal/aspect"
	"sounds-great-ai/pkg/pack"
)

// CommandCheck 命令安全检查适配器，包装 aspect.CommandGuard
type CommandCheck struct {
	guard *aspect.CommandGuard
}

// NewCommandCheck 创建一个新的 CommandCheck
func NewCommandCheck() *CommandCheck {
	return &CommandCheck{guard: aspect.NewCommandGuard()}
}

func (c *CommandCheck) Name() string    { return "command_check" }
func (c *CommandCheck) Version() string { return "v1" }

func (c *CommandCheck) Init(ctx context.Context) error { return nil }

func (c *CommandCheck) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	result := c.guard.GuardCommand(input.Command)
	return &pack.TaskOutput{
		Approved: result.Status == aspect.GuardStatusAllowed,
		Reason:   result.Reason,
	}, nil
}

func (c *CommandCheck) Health() error { return nil }
func (c *CommandCheck) Close() error  { return nil }
