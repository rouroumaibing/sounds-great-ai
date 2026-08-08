package protocol

import (
	"encoding/json"
	"time"
)

// EventType 是协议事件的类型
type EventType string

const (
	EventThinking       EventType = "THINKING"
	EventToolCall       EventType = "TOOL_CALL"
	EventCodeDiff       EventType = "CODE_DIFF"
	EventTerminalOutput EventType = "TERMINAL_OUTPUT"
	EventUserInput      EventType = "USER_INPUT"
	EventHITLApproval   EventType = "HITL_APPROVAL"
	EventHitlResponse   EventType = "HITL_RESPONSE"
)

// Event 是 WebSocket 协议的统一事件结构
type Event struct {
	Type      EventType       `json:"type"`
	SessionID string          `json:"session_id"`
	Timestamp int64           `json:"timestamp"`
	Seq       uint64          `json:"seq,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// NewEvent 创建一个新的事件，自动设置时间戳并序列化 payload
func NewEvent(typ EventType, sessionID string, payload interface{}) *Event {
	data, _ := json.Marshal(payload)
	return &Event{
		Type:      typ,
		SessionID: sessionID,
		Timestamp: time.Now().Unix(),
		Payload:   data,
	}
}

// ThinkingPayload 是 THINKING 事件的 payload
type ThinkingPayload struct {
	Step      int    `json:"step"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// ToolCallPayload 是 TOOL_CALL 事件的 payload
type ToolCallPayload struct {
	Tool   string `json:"tool"`
	Params string `json:"params"`
	Result string `json:"result"`
	Status string `json:"status"`
}

// CodeDiffPayload 是 CODE_DIFF 事件的 payload
type CodeDiffPayload struct {
	File   string `json:"file"`
	Diff   string `json:"diff"`
	Action string `json:"action"`
}

// TerminalOutputPayload 是 TERMINAL_OUTPUT 事件的 payload
type TerminalOutputPayload struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

// UserInputPayload 是 USER_INPUT 事件的 payload
type UserInputPayload struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

// HITLApprovalPayload 是 HITL_APPROVAL 事件的 payload
type HITLApprovalPayload struct {
	Action    string `json:"action"`
	Approved  bool   `json:"approved"`
	RequestID string `json:"request_id"`
	Impact    string `json:"impact"`
}

// HitlResponsePayload is the payload for HITL_RESPONSE events (client → server)
type HitlResponsePayload struct {
	RequestID string `json:"request_id"`
	Approved  bool   `json:"approved"`
	Reason    string `json:"reason"`
}

// Bark 事件类型
const (
	EventBarkStart  EventType = "BARK_START"
	EventBarkResult EventType = "BARK_RESULT"
	EventBarkError  EventType = "BARK_ERROR"
)

// BarkStartPayload 是 BARK_START 事件的 payload
type BarkStartPayload struct {
	Breed     string `json:"breed"`
	SessionID string `json:"session_id"`
	Query     string `json:"query"`
}

// StepResult 单步执行结果
type StepResult struct {
	Capability string `json:"capability"`
	Approved   bool   `json:"approved"`
	Reason     string `json:"reason"`
}

// BarkResultPayload 是 BARK_RESULT 事件的 payload
type BarkResultPayload struct {
	Breed   string                `json:"breed"`
	Success bool                  `json:"success"`
	Steps   map[string]StepResult `json:"steps"`
}

// BarkErrorPayload 是 BARK_ERROR 事件的 payload
type BarkErrorPayload struct {
	Breed string `json:"breed"`
	Error string `json:"error"`
}

// System notice 事件类型
const (
	EventSystemNotice EventType = "SYSTEM_NOTICE"
)

// SystemNoticePayload 是 SYSTEM_NOTICE 事件的 payload
type SystemNoticePayload struct {
	Severity  string `json:"severity"`  // "critical", "warning", "info", "recovery"
	Title     string `json:"title"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"` // ISO 8601
}
