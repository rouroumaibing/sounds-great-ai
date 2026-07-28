package a2a

// AgentCard describes Agent capabilities
type AgentCard struct {
	Name                string   `json:"name"`
	Description         string   `json:"description,omitempty"`
	URL                 string   `json:"url"`
	SupportedInterfaces []string `json:"supported_interfaces"`
	Capabilities        []string `json:"capabilities,omitempty"`
}

// TaskStatus task status enum
type TaskStatus string

const (
	TaskStatusSubmitted     TaskStatus = "submitted"
	TaskStatusWorking       TaskStatus = "working"
	TaskStatusCompleted     TaskStatus = "completed"
	TaskStatusFailed        TaskStatus = "failed"
	TaskStatusInputRequired TaskStatus = "input-required"
	TaskStatusCanceled      TaskStatus = "canceled"
)

// TraceInfo distributed tracing info (OpenTelemetry)
type TraceInfo struct {
	TraceID      string `json:"trace_id,omitempty"`
	SpanID       string `json:"span_id,omitempty"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
}

// ToolCall tool call description
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolResult tool call result
type ToolResult struct {
	CallID string `json:"call_id"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// Part content unit
type Part struct {
	Type       string      `json:"type"`
	Text       string      `json:"text,omitempty"`
	ToolCall   *ToolCall   `json:"tool_call,omitempty"`
	ToolResult *ToolResult `json:"tool_result,omitempty"`
}

// Message message unit
type Message struct {
	ID         string `json:"id"`
	ParentID   string `json:"parent_id,omitempty"`
	Role       string `json:"role"`
	SenderName string `json:"sender_name,omitempty"`
	Parts      []Part `json:"parts"`
}

// ExtractText concatenates all text parts
func (m Message) ExtractText() string {
	var result string
	for _, p := range m.Parts {
		if p.Type == "text" {
			result += p.Text
		}
	}
	return result
}

// Artifact task artifact
type Artifact struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Parts       []Part `json:"parts"`
}

// Task core work unit
type Task struct {
	ID        string                 `json:"id"`
	ContextID string                 `json:"context_id,omitempty"`
	Status    TaskStatus             `json:"status"`
	Artifacts []Artifact             `json:"artifacts,omitempty"`
	History   []Message              `json:"history,omitempty"`
	Trace     *TraceInfo             `json:"trace,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// JSONRPCRequest JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// JSONRPCResponse JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  *Task       `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
