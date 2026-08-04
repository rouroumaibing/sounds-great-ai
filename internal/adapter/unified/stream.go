package unified

// StreamEvent represents one event in a streaming response from a CLI agent.
type StreamEvent struct {
	Type    string         // "text" | "tool_call" | "tool_result" | "thinking" | "error" | "done"
	Content string
	Meta    map[string]any
}

// IsError returns true if this is an error event.
func (e StreamEvent) IsError() bool {
	return e.Type == "error"
}

// IsDone returns true if this is a terminal event.
func (e StreamEvent) IsDone() bool {
	return e.Type == "done"
}
