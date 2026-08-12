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
