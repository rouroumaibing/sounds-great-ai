package capability

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"sounds-great-ai/internal/memory"
	"sounds-great-ai/pkg/pack"
)

// stubChatModel is a test double for model.BaseChatModel. It records the last
// messages it received and returns a canned reply — no network.
type stubChatModel struct {
	lastMessages []*schema.Message
	reply        string
}

func (s *stubChatModel) Generate(ctx context.Context, in []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	s.lastMessages = in
	return schema.AssistantMessage(s.reply, nil), nil
}

func (s *stubChatModel) Stream(ctx context.Context, in []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("stream unsupported")
}

var _ model.BaseChatModel = (*stubChatModel)(nil)

func sampleEntries() []*memory.LaneEntry {
	return []*memory.LaneEntry{
		{ID: "e1", Type: memory.LaneDecision, Content: "决定采用事件溯源架构", Source: "session:1"},
		{ID: "e2", Type: memory.LaneLesson, Content: "不要在 internal/ 做 LLM 推理", Source: "session:2"},
	}
}

func TestMemoryReflectReflect(t *testing.T) {
	stub := &stubChatModel{reply: "REFLECTION_RESULT"}
	r := NewMemoryReflect(stub)

	out, err := r.Reflect(context.Background(), sampleEntries(), ReflectOptions{Focus: "协作", MaxChars: 1500})
	if err != nil {
		t.Fatalf("Reflect error: %v", err)
	}
	if out != "REFLECTION_RESULT" {
		t.Fatalf("expected canned reply, got %q", out)
	}
	// The user prompt must contain the entry content so the model can reflect.
	if len(stub.lastMessages) < 2 {
		t.Fatalf("expected system+user messages, got %d", len(stub.lastMessages))
	}
	userMsg := stub.lastMessages[len(stub.lastMessages)-1]
	if !strings.Contains(userMsg.Content, "决定采用事件溯源架构") {
		t.Fatalf("prompt missing entry content: %q", userMsg.Content)
	}
	if !strings.Contains(userMsg.Content, "协作") {
		t.Fatalf("prompt missing focus directive: %q", userMsg.Content)
	}
	if !strings.Contains(userMsg.Content, "1500") {
		t.Fatalf("prompt missing max_chars: %q", userMsg.Content)
	}
}

func TestMemoryReflectHealth(t *testing.T) {
	if err := NewMemoryReflect(nil).Health(); err == nil {
		t.Fatal("expected Health error when no model configured")
	}
	if err := NewMemoryReflect(&stubChatModel{}).Health(); err != nil {
		t.Fatalf("unexpected Health error: %v", err)
	}
}

func TestMemoryReflectRunAdapter(t *testing.T) {
	stub := &stubChatModel{reply: "CAP_REFLECT"}
	r := NewMemoryReflect(stub)
	out, err := r.Run(context.Background(), &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"memory": {Data: map[string]any{"entries": sampleEntries()}},
		},
		CapabilityConfig: map[string]any{"max_chars": 1200, "focus": "架构"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.Data["reflection"] != "CAP_REFLECT" {
		t.Fatalf("expected reflection in output, got %v", out.Data["reflection"])
	}
	// Focus from CapabilityConfig must reach the model prompt.
	if !strings.Contains(stub.lastMessages[len(stub.lastMessages)-1].Content, "架构") {
		t.Fatalf("Run focus not propagated: %q", stub.lastMessages[len(stub.lastMessages)-1].Content)
	}
}

func TestMemoryReflectNoEntries(t *testing.T) {
	r := NewMemoryReflect(&stubChatModel{})
	if _, err := r.Reflect(context.Background(), nil, ReflectOptions{}); err == nil {
		t.Fatal("expected error when reflecting on zero entries")
	}
}
