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
	// Sink, when non-nil, lets capabilities push events to a transport
	// (e.g. WebSocket). Capabilities must handle nil Sink gracefully.
	Sink EventSink
}

// TaskOutput capability 输出
type TaskOutput struct {
	Approved bool
	Reason   string
	Results  []any
	Data     map[string]any
	// Control marks this output as belonging to a control-flow step
	// (decompose/dispatch/execute) rather than a breed result. result_merge
	// skips Control=true outputs instead of matching by stepID string.
	//
	// Read-only contract: reference types inside Data (nested maps, slices,
	// pointers) are strictly read-only. Capabilities MUST NOT mutate upstream
	// Data values; downstream consumers rely on this for safe shallow-copy.
	Control bool
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
