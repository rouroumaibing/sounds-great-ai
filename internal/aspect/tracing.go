package aspect

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/tool"
	"sounds-great-ai/pkg/protocol"
)

// EventSender 是发送事件的回调函数类型
type EventSender func(ctx context.Context, ev *protocol.Event)

// TracingCallback 实现 Eino callback 接口，推送 THINKING 和 TOOL_CALL 事件
type TracingCallback struct {
	sessionID string
	sendEvent EventSender
	handler   callbacks.Handler
}

// NewTracingCallback 创建一个新的追踪回调
func NewTracingCallback(sessionID string, sendEvent EventSender) *TracingCallback {
	tc := &TracingCallback{
		sessionID: sessionID,
		sendEvent: sendEvent,
	}
	tc.handler = tc.buildHandler()
	return tc
}

// GetHandler 返回 Eino callback handler
func (tc *TracingCallback) GetHandler() callbacks.Handler {
	return tc.handler
}

// OnStart 委托给底层 handler
func (tc *TracingCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	return tc.handler.OnStart(ctx, info, input)
}

// OnEnd 委托给底层 handler
func (tc *TracingCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	return tc.handler.OnEnd(ctx, info, output)
}

// OnError 委托给底层 handler
func (tc *TracingCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	return tc.handler.OnError(ctx, info, err)
}

func (tc *TracingCallback) buildHandler() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			if info == nil {
				return ctx
			}

			switch info.Component {
			case components.ComponentOfChatModel:
				// 模型开始：推送 THINKING 事件
				ev := protocol.NewEvent(protocol.EventThinking, tc.sessionID, &protocol.ThinkingPayload{
					Step:    1,
					Content: "Model generating...",
				})
				tc.sendEvent(ctx, ev)

			case components.ComponentOfTool:
				// 工具开始：推送 TOOL_CALL 事件 (status: running)
				toolInput := tool.ConvCallbackInput(input)
				toolName := ""
				params := ""
				if toolInput != nil {
					params = toolInput.ArgumentsInJSON
				}
				if info != nil {
					toolName = info.Name
				}
				ev := protocol.NewEvent(protocol.EventToolCall, tc.sessionID, &protocol.ToolCallPayload{
					Tool:   toolName,
					Params: params,
					Status: "running",
				})
				tc.sendEvent(ctx, ev)
			}

			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			if info == nil {
				return ctx
			}

			switch info.Component {
			case components.ComponentOfTool:
				// 工具结束：推送 TOOL_CALL 事件 (status: success/error)
				toolOutput := tool.ConvCallbackOutput(output)
				result := ""
				status := "success"
				if toolOutput != nil {
					result = toolOutput.Response
				}
				ev := protocol.NewEvent(protocol.EventToolCall, tc.sessionID, &protocol.ToolCallPayload{
					Tool:   info.Name,
					Result: result,
					Status: status,
				})
				tc.sendEvent(ctx, ev)
			}

			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			if info == nil {
				return ctx
			}

			if info.Component == components.ComponentOfTool {
				ev := protocol.NewEvent(protocol.EventToolCall, tc.sessionID, &protocol.ToolCallPayload{
					Tool:   info.Name,
					Result: err.Error(),
					Status: "error",
				})
				tc.sendEvent(ctx, ev)
			}

			return ctx
		}).
		Build()
}
