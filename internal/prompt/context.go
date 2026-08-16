package prompt

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

// ContextAssembler formats conversation history for CLI adapter invocations.
// It applies token budget control, message formatting, and truncation strategy.
type ContextAssembler struct {
	maxMessages   int
	maxTokens     int
	maxContentLen int
}

// Default assembler configuration.
const (
	defaultMaxMessages   = 20
	defaultMaxTokens     = 2000
	defaultMaxContentLen = 1500
)

// NewContextAssembler creates an assembler with defaults.
func NewContextAssembler() *ContextAssembler {
	return &ContextAssembler{
		maxMessages:   defaultMaxMessages,
		maxTokens:     defaultMaxTokens,
		maxContentLen: defaultMaxContentLen,
	}
}

// WithMaxMessages sets the max number of messages to include.
func (a *ContextAssembler) WithMaxMessages(n int) *ContextAssembler {
	a.maxMessages = n
	return a
}

// WithMaxTokens sets the token budget for context.
func (a *ContextAssembler) WithMaxTokens(n int) *ContextAssembler {
	a.maxTokens = n
	return a
}

// WithMaxContentLen sets the max characters per message content.
func (a *ContextAssembler) WithMaxContentLen(n int) *ContextAssembler {
	a.maxContentLen = n
	return a
}

// ContextMessage represents a single message in conversation history.
type ContextMessage struct {
	Role      string    // "user" | "assistant" | "system"
	Content   string    // message text
	Sender    string    // breed display name (for assistant messages)
	Timestamp time.Time // when the message was sent
}

// AssembleResult holds the assembled context.
type AssembleResult struct {
	Text         string // formatted context text
	MessageCount int    // number of messages included
	EstTokens    int    // estimated token count
	Truncated    bool   // whether content was truncated
}

// Assemble formats conversation history into a context string.
// Messages are formatted as: [timestamp sender] content
// Token budget is enforced by truncating from the head (oldest messages dropped first).
func (a *ContextAssembler) Assemble(messages []ContextMessage) AssembleResult {
	if len(messages) == 0 {
		return AssembleResult{}
	}

	// Limit number of messages (take the most recent)
	start := 0
	if len(messages) > a.maxMessages {
		start = len(messages) - a.maxMessages
	}
	messages = messages[start:]

	var sb strings.Builder
	truncated := false
	count := 0
	estTokens := 0

	// Build from oldest to newest, but stop if we exceed token budget
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		// Format message
		line := a.formatMessage(msg)
		lineTokens := estimateTokens(line)

		// Check token budget
		if estTokens+lineTokens > a.maxTokens && count > 0 {
			truncated = true
			break
		}

		// Prepend (we're building backwards)
		sb.WriteString(line)
		estTokens += lineTokens
		count++
	}

	return AssembleResult{
		Text:         strings.TrimSpace(sb.String()),
		MessageCount: count,
		EstTokens:    estTokens,
		Truncated:    truncated,
	}
}

// formatMessage formats a single message as [timestamp sender] content.
func (a *ContextAssembler) formatMessage(msg ContextMessage) string {
	content := msg.Content
	if len([]rune(content)) > a.maxContentLen {
		content = string([]rune(content)[:a.maxContentLen]) + "..."
	}

	sender := msg.Sender
	if sender == "" {
		sender = msg.Role
	}

	ts := msg.Timestamp.Format("15:04:05")

	return fmt.Sprintf("[%s %s] %s\n\n", ts, sender, content)
}

// estimateTokens gives a rough token count (1 token ≈ 4 chars for mixed CJK/English).
func estimateTokens(s string) int {
	return len([]rune(s)) / 3
}

// BoundContextByTokens drops the oldest messages until the estimated token
// count of the remaining history fits within maxTokens (Persistent Identity
// P2, homologous auto-compact budget). The most recent messages are
// kept; only the head (oldest) is trimmed, mirroring the platform-side
// compression driven from each cat's contextWindow. A non-positive
// maxTokens is a no-op (no bound applied).
func BoundContextByTokens(msgs []ContextMessage, maxTokens int) []ContextMessage {
	if maxTokens <= 0 || len(msgs) == 0 {
		return msgs
	}
	// Estimate total first; if already under budget, return as-is.
	total := 0
	for _, m := range msgs {
		total += estimateTokens(m.Role + " " + m.Sender + " " + m.Content)
	}
	if total <= maxTokens {
		return msgs
	}
	// Trim from the oldest (head) until within budget.
	kept := make([]ContextMessage, 0, len(msgs))
	used := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		cost := estimateTokens(msgs[i].Role + " " + msgs[i].Sender + " " + msgs[i].Content)
		if used+cost > maxTokens && len(kept) > 0 {
			break
		}
		kept = append(kept, msgs[i])
		used += cost
	}
	// Reverse back to chronological order.
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	return kept
}

// ToSchemaMessages converts ContextMessages to Eino schema.Message format
// for use with CLI adapter ExecuteRequest.
func ToSchemaMessages(messages []ContextMessage) []*schema.Message {
	result := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		var m *schema.Message
		switch msg.Role {
		case "user":
			m = schema.UserMessage(msg.Content)
		case "assistant":
			m = schema.AssistantMessage(msg.Content, nil)
		case "system":
			m = schema.SystemMessage(msg.Content)
		default:
			m = schema.UserMessage(msg.Content)
		}
		result = append(result, m)
	}
	return result
}

// ProtectRecentPairs guarantees the most recent `keepPairs` (user, assistant)
// exchanges survive any downstream truncation (G7 step 3: protect the Q→A
// semantic chain from burst-window cuts). Leading system messages are always
// retained; the trailing `keepPairs*2` (aligned to whole pairs) messages are
// kept, and any older turns are dropped. Non-positive keepPairs is a no-op.
func ProtectRecentPairs(msgs []*schema.Message, keepPairs int) []*schema.Message {
	if keepPairs <= 0 || len(msgs) == 0 {
		return msgs
	}
	// Separate leading system messages (always keep).
	sysEnd := 0
	for sysEnd < len(msgs) && msgs[sysEnd].Role == schema.System {
		sysEnd++
	}
	rest := msgs[sysEnd:]
	keep := keepPairs * 2
	if keep > len(rest) {
		keep = len(rest)
	}
	if keep%2 != 0 {
		keep-- // keep whole pairs only
	}
	if keep < 0 {
		keep = 0
	}
	out := make([]*schema.Message, 0, sysEnd+keep)
	out = append(out, msgs[:sysEnd]...)
	if keep > 0 {
		out = append(out, rest[len(rest)-keep:]...)
	}
	return out
}
