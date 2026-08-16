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
	EventWakeHold       EventType = "WAKE_HOLD"
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
func NewEvent(typ EventType, sessionID string, payload any) *Event {
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

// WakeHoldPayload is the payload for WAKE_HOLD events (client → server). It
// releases a parked hold_ball (P2) so the orchestrator can resume the holder.
// Kind is "manual" (human click) or "webhook" (external POST). Token is only
// required for webhook-kind holds.
type WakeHoldPayload struct {
	SessionID string `json:"session_id"`
	Kind      string `json:"kind"`
	Token     string `json:"token,omitempty"`
}

// Bark 事件类型
const (
	EventBarkStart  EventType = "BARK_START"
	EventBarkResult EventType = "BARK_RESULT"
	EventBarkError  EventType = "BARK_ERROR"
	// EventAgentMessage streams assistant text deltas live (G1). The frontend
	// accumulates Content into a running breed_response block; the terminal
	// BARK_RESULT carries the final state.
	EventAgentMessage EventType = "AGENT_MESSAGE"
	// EventAgentLiveness surfaces liveness-probe state changes to the client
	// (R8): soft/hard stalls (alive but silent) and recovery. The frontend
	// renders a transient status bar instead of failing silently.
	EventAgentLiveness EventType = "AGENT_LIVENESS"
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
	// Content carries the final assistant text when available (G9), so the live
	// end-state renders without waiting for REST history hydration. Optional.
	Content string `json:"content,omitempty"`
}

// AgentMessagePayload 是 AGENT_MESSAGE 事件的 payload (G1)
type AgentMessagePayload struct {
	Breed   string `json:"breed"`
	Content string `json:"content"` // incremental text delta
	Done    bool   `json:"done"`    // reserved terminal marker
}

// LivenessPayload 是 AGENT_LIVENESS 事件的 payload (R8)
type LivenessPayload struct {
	Breed   string `json:"breed"`
	State   string `json:"state"`   // "active" | "busy_silent" | "idle_silent" | "dead"
	Hard    bool   `json:"hard"`    // true => hard stall (beyond ProbeStallWarnMs)
	Message string `json:"message"` // safe, human-facing status hint
}

// BarkErrorPayload 是 BARK_ERROR 事件的 payload
type BarkErrorPayload struct {
	Breed string `json:"breed"`
	Error string `json:"error"`
	// Structured diagnostics (cliDiagnostics). All optional; the
	// client falls back to Error when these are empty. Reason/Summary/Hint are
	// classified + public-safe; Excerpt is already sanitized (REDACTED-*) on the
	// server; Source tags where the excerpt came from (gated by a client
	// allowlist before raw display); Meta carries redactable context (paths,
	// cli command) shown in a meta bar.
	Reason  string            `json:"reason,omitempty"`
	Summary string            `json:"summary,omitempty"`
	Hint    string            `json:"hint,omitempty"`
	Excerpt string            `json:"excerpt,omitempty"`
	Source  string            `json:"source,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
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

// Carrier health 事件类型 (T25 / R6)
const (
	EventCarrierHealth EventType = "CARRIER_HEALTH"
)

// CarrierHealthPayload 是 CARRIER_HEALTH 事件的 payload (T25 / R6): the
// backend surfaces a carrier's health (quota/structural/transient degradation)
// so the frontend ConnectionStatusBar can render upstream model health
// directly instead of inferring it from raw stream events.
type CarrierHealthPayload struct {
	Carrier     string `json:"carrier"`
	Transport   string `json:"transport,omitempty"`
	Level       string `json:"level"` // "online" | "degraded" | "offline"
	Reason      string `json:"reason,omitempty"`
	RemainingMs int64  `json:"remaining_ms,omitempty"`
}
