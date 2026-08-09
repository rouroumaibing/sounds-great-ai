package ports

import "context"

// RegisteredHook represents a hook registered in the system.
type RegisteredHook struct {
	ID       string
	Stage    string
	Order    int
	Enabled  bool
	Template string
	Dir      string
}

// IHookRegistry is the port for hook registry.
type IHookRegistry interface {
	Scan() error
	Get(id string) *RegisteredHook
	All() []*RegisteredHook
	GetStageHooks(stage string) []*RegisteredHook
}

// IHookPipeline is the port for hook pipeline execution.
type IHookPipeline interface {
	Execute(ctx context.Context, stage string, input map[string]any) (map[string]any, error)
}

// IHookTraceStore is the port for hook trace storage.
type IHookTraceStore interface {
	Record(ctx context.Context, trace HookTrace) error
	Query(ctx context.Context, filter HookTraceFilter) ([]HookTrace, error)
}

// HookTrace represents a hook execution trace.
type HookTrace struct {
	ID        string                 `json:"id"`
	HookID    string                 `json:"hook_id"`
	Stage     string                 `json:"stage"`
	Input     map[string]any         `json:"input"`
	Output    map[string]any         `json:"output"`
	Error     string                 `json:"error,omitempty"`
	Duration  int64                  `json:"duration_ns"`
	Timestamp int64                  `json:"timestamp"`
}

// HookTraceFilter filters hook trace queries.
type HookTraceFilter struct {
	HookID string
	Stage  string
	Limit  int
}
