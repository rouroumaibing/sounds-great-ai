package capability

import (
	"context"

	"sounds-great-ai/internal/aspect"
	"sounds-great-ai/pkg/pack"
)

// PathValidate 路径验证适配器，包装 aspect.CommandGuard
type PathValidate struct {
	guard *aspect.CommandGuard
}

// NewPathValidate 创建一个新的 PathValidate
func NewPathValidate() *PathValidate {
	return &PathValidate{guard: aspect.NewCommandGuard()}
}

func (p *PathValidate) Name() string    { return "path_validate" }
func (p *PathValidate) Version() string { return "v1" }

func (p *PathValidate) Init(ctx context.Context) error { return nil }

func (p *PathValidate) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	result := p.guard.GuardFilePath(input.Path, aspect.FileOpWrite)
	return &pack.TaskOutput{
		Approved: result.Status == aspect.GuardStatusAllowed,
		Reason:   result.Reason,
	}, nil
}

func (p *PathValidate) Health() error { return nil }
func (p *PathValidate) Close() error  { return nil }
