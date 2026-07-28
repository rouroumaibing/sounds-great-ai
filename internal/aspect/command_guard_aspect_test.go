package aspect

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/tool"
	"sounds-great-ai/pkg/protocol"
)

func TestCommandGuardAspectBlocksRmRf(t *testing.T) {
	guard := NewCommandGuard()
	approvalMgr := NewApprovalManager()

	var mu sync.Mutex
	var events []*protocol.Event

	aspect := NewCommandGuardAspect(guard, approvalMgr, "session-1", func(ctx context.Context, ev *protocol.Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	// 模拟 Agent 尝试执行 rm -rf /
	info := &callbacks.RunInfo{
		Name:      "run_command",
		Type:      "RunCommand",
		Component: components.ComponentOfTool,
	}
	input := &tool.CallbackInput{
		ArgumentsInJSON: mustJSON(map[string]string{"command": "rm -rf /"}),
	}

	ctx := aspect.OnStart(context.Background(), info, input)

	mu.Lock()
	defer mu.Unlock()

	// 应该产生一个 TOOL_CALL 事件，状态为 error（被拦截）
	found := false
	for _, ev := range events {
		if ev.Type == protocol.EventToolCall {
			var payload protocol.ToolCallPayload
			json.Unmarshal(ev.Payload, &payload)
			if payload.Status == "error" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected TOOL_CALL error event for blocked command")
	}
	_ = ctx
}

func TestCommandGuardAspectHITLFlow(t *testing.T) {
	guard := NewCommandGuard()
	approvalMgr := NewApprovalManager()

	var mu sync.Mutex
	var events []*protocol.Event

	aspect := NewCommandGuardAspect(guard, approvalMgr, "session-1", func(ctx context.Context, ev *protocol.Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	// 模拟 Agent 尝试写入 .env
	info := &callbacks.RunInfo{
		Name:      "edit_file",
		Type:      "EditFile",
		Component: components.ComponentOfTool,
	}
	input := &tool.CallbackInput{
		ArgumentsInJSON: mustJSON(map[string]string{"path": ".env", "content": "SECRET=xxx"}),
	}

	// 在另一个 goroutine 中批准
	go func() {
		// 等待 HITL_APPROVAL 事件
		time.Sleep(100 * time.Millisecond)
		approvalMgr.ResolveApprovalByAction("write .env", true)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = aspect.OnStart(ctx, info, input)

	// 应该产生 HITL_APPROVAL 事件
	mu.Lock()
	defer mu.Unlock()

	foundHITL := false
	for _, ev := range events {
		if ev.Type == protocol.EventHITLApproval {
			foundHITL = true
		}
	}
	if !foundHITL {
		t.Error("expected HITL_APPROVAL event for .env write")
	}
}

func TestCommandGuardAspectAllowsSafeCommand(t *testing.T) {
	guard := NewCommandGuard()
	approvalMgr := NewApprovalManager()

	var mu sync.Mutex
	var events []*protocol.Event

	aspect := NewCommandGuardAspect(guard, approvalMgr, "session-1", func(ctx context.Context, ev *protocol.Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	info := &callbacks.RunInfo{
		Name:      "run_command",
		Type:      "RunCommand",
		Component: components.ComponentOfTool,
	}
	input := &tool.CallbackInput{
		ArgumentsInJSON: mustJSON(map[string]string{"command": "echo hello"}),
	}

	ctx := aspect.OnStart(context.Background(), info, input)

	// 安全命令不应产生错误事件
	mu.Lock()
	defer mu.Unlock()

	for _, ev := range events {
		if ev.Type == protocol.EventToolCall {
			var payload protocol.ToolCallPayload
			json.Unmarshal(ev.Payload, &payload)
			if payload.Status == "error" {
				t.Error("safe command should not produce error event")
			}
		}
	}
	_ = ctx
}

func mustJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
