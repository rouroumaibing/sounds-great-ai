package pack

import "context"

// ExecutionContext 执行上下文
type ExecutionContext struct {
	UserID      string            `json:"user_id"`
	SessionID   string            `json:"session_id"`
	Workspace   string            `json:"workspace"`
	TraceID     string            `json:"trace_id"`
	Permissions []string          `json:"permissions"`
	Metadata    map[string]string `json:"metadata"`
}

// TaskInput capability 输入
type TaskInput struct {
	Query            string
	Command          string
	Path             string
	Context          *ExecutionContext
	Previous         map[string]*TaskOutput
	CapabilityConfig map[string]any
	// READ-ONLY: current breed config (model, system_prompt, etc.)
	Breed *BreedConfig
}

// TaskOutput capability 输出
type TaskOutput struct {
	Approved bool
	Reason   string
	Results  []any
	Data     map[string]any
}

// Capability 接口，含生命周期方法
type Capability interface {
	Name() string
	Version() string
	Init(ctx context.Context) error
	Run(ctx context.Context, input *TaskInput) (*TaskOutput, error)
	Health() error
	Close() error
}
