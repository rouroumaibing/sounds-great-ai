package prompt

import (
	"strings"
	"testing"
	"time"
)

func TestContextAssemblerAssembleBasic(t *testing.T) {
	t.Parallel()
	a := NewContextAssembler()
	msgs := []ContextMessage{
		{Role: "user", Content: "Hello", Sender: "user", Timestamp: time.Now()},
		{Role: "assistant", Content: "Hi there", Sender: "边牧", Timestamp: time.Now()},
	}
	result := a.Assemble(msgs)

	if result.MessageCount != 2 {
		t.Errorf("expected 2 messages, got %d", result.MessageCount)
	}
	if !strings.Contains(result.Text, "Hello") {
		t.Error("expected first message content")
	}
	if !strings.Contains(result.Text, "Hi there") {
		t.Error("expected second message content")
	}
	if !strings.Contains(result.Text, "边牧") {
		t.Error("expected sender name in output")
	}
}

func TestContextAssemblerAssembleEmpty(t *testing.T) {
	t.Parallel()
	a := NewContextAssembler()
	result := a.Assemble(nil)
	if result.MessageCount != 0 || result.Text != "" {
		t.Error("expected empty result for nil input")
	}
}

func TestContextAssemblerAssembleMaxMessages(t *testing.T) {
	t.Parallel()
	a := NewContextAssembler().WithMaxMessages(3)
	msgs := make([]ContextMessage, 10)
	for i := range msgs {
		msgs[i] = ContextMessage{
			Role: "user", Content: "msg", Sender: "user",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		}
	}
	result := a.Assemble(msgs)
	if result.MessageCount > 3 {
		t.Errorf("expected at most 3 messages, got %d", result.MessageCount)
	}
}

func TestContextAssemblerAssembleTokenBudget(t *testing.T) {
	t.Parallel()
	a := NewContextAssembler().WithMaxTokens(20)
	msgs := []ContextMessage{
		{Role: "user", Content: strings.Repeat("a", 100), Sender: "user", Timestamp: time.Now()},
		{Role: "assistant", Content: strings.Repeat("b", 100), Sender: "bot", Timestamp: time.Now()},
	}
	result := a.Assemble(msgs)
	if result.EstTokens > 20*2 { // allow some slack for the format overhead
		t.Errorf("token budget exceeded: %d", result.EstTokens)
	}
}

func TestContextAssemblerAssembleContentTruncation(t *testing.T) {
	t.Parallel()
	a := NewContextAssembler().WithMaxContentLen(10)
	msgs := []ContextMessage{
		{Role: "user", Content: strings.Repeat("a", 100), Sender: "user", Timestamp: time.Now()},
	}
	result := a.Assemble(msgs)
	if !strings.Contains(result.Text, "...") {
		t.Error("expected truncation marker")
	}
}

func TestContextAssemblerAssembleCJK(t *testing.T) {
	t.Parallel()
	a := NewContextAssembler()
	msgs := []ContextMessage{
		{Role: "user", Content: "你好世界，这是一个测试消息", Sender: "用户", Timestamp: time.Now()},
	}
	result := a.Assemble(msgs)
	if !strings.Contains(result.Text, "你好世界") {
		t.Error("expected CJK content in output")
	}
}

func TestToSchemaMessages(t *testing.T) {
	t.Parallel()
	msgs := []ContextMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "system", Content: "system prompt"},
		{Role: "unknown", Content: "fallback"},
	}
	result := ToSchemaMessages(msgs)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("expected user role, got %s", result[0].Role)
	}
	if result[1].Role != "assistant" {
		t.Errorf("expected assistant role, got %s", result[1].Role)
	}
	if result[2].Role != "system" {
		t.Errorf("expected system role, got %s", result[2].Role)
	}
	if result[3].Role != "user" {
		t.Errorf("expected fallback to user role, got %s", result[3].Role)
	}
}

func TestEstimateTokens(t *testing.T) {
	t.Parallel()
	if got := estimateTokens("hello world"); got <= 0 {
		t.Error("expected positive token count for ASCII")
	}
	if got := estimateTokens("你好世界"); got <= 0 {
		t.Error("expected positive token count for CJK")
	}
}

// TestBoundContextByTokens verifies Persistent Identity P2: the breed's
// auto-compact budget drops the OLDEST messages first and keeps the most
// recent ones (homologous auto-compact at the orchestration layer).
func TestBoundContextByTokens(t *testing.T) {
	t.Parallel()
	base := time.Now()
	msgs := []ContextMessage{
		{Role: "user", Content: "old message one", Sender: "user", Timestamp: base},
		{Role: "assistant", Content: "old reply one", Sender: "边牧", Timestamp: base},
		{Role: "user", Content: "middle message", Sender: "user", Timestamp: base},
		{Role: "assistant", Content: "recent reply", Sender: "金毛", Timestamp: base},
		{Role: "user", Content: "newest message", Sender: "user", Timestamp: base},
	}

	// Budget of 0 is a no-op.
	if got := BoundContextByTokens(msgs, 0); len(got) != len(msgs) {
		t.Errorf("budget 0 should be a no-op, got %d", len(got))
	}

	// Tight budget: only the most recent message survives, and it must be the
	// newest one (order preserved, oldest dropped).
	got := BoundContextByTokens(msgs, 5)
	if len(got) == 0 {
		t.Fatal("expected at least one message to survive")
	}
	if got[len(got)-1].Content != "newest message" {
		t.Errorf("most recent message should survive, got %q", got[len(got)-1].Content)
	}
	if got[0].Content == "old message one" {
		t.Error("oldest message should have been dropped")
	}

	// Generous budget: everything fits.
	if got := BoundContextByTokens(msgs, 100000); len(got) != len(msgs) {
		t.Errorf("generous budget should keep all, got %d", len(got))
	}
}
