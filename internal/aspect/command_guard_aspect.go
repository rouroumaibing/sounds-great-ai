package aspect

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/tool"
	"sounds-great-ai/pkg/protocol"
)

// CommandGuardAspect 组合命令防护、审批管理和事件发送
type CommandGuardAspect struct {
	guard       *CommandGuard
	approvalMgr *ApprovalManager
	sessionID   string
	sendEvent   EventSender
	handler     callbacks.Handler
}

// NewCommandGuardAspect 创建一个新的命令防护切面
func NewCommandGuardAspect(
	guard *CommandGuard,
	approvalMgr *ApprovalManager,
	sessionID string,
	sendEvent EventSender,
) *CommandGuardAspect {
	aspect := &CommandGuardAspect{
		guard:       guard,
		approvalMgr: approvalMgr,
		sessionID:   sessionID,
		sendEvent:   sendEvent,
	}
	aspect.approvalMgr.SetEventSender(sendEvent)
	aspect.handler = aspect.buildHandler()
	return aspect
}

// GetHandler 返回 Eino callback handler
func (a *CommandGuardAspect) GetHandler() callbacks.Handler {
	return a.handler
}

// OnStart 委托给底层 handler
func (a *CommandGuardAspect) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	return a.handler.OnStart(ctx, info, input)
}

func (a *CommandGuardAspect) buildHandler() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			if info == nil {
				return ctx
			}

			if info.Component == components.ComponentOfTool {
				toolInput := tool.ConvCallbackInput(input)
				if toolInput == nil {
					return ctx
				}

				switch info.Name {
				case "run_command":
					return a.guardRunCommand(ctx, info, toolInput)
				case "edit_file":
					return a.guardEditFile(ctx, info, toolInput)
				}
			}

			return ctx
		}).
		Build()
}

// guardRunCommand 拦截 run_command 工具调用
func (a *CommandGuardAspect) guardRunCommand(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
	var params struct {
		Command string `json:"command"`
	}
	json.Unmarshal([]byte(input.ArgumentsInJSON), &params)

	result := a.guard.GuardCommand(params.Command)

	switch result.Status {
	case GuardStatusBlocked:
		// 发送拦截事件
		ev := protocol.NewEvent(protocol.EventToolCall, a.sessionID, &protocol.ToolCallPayload{
			Tool:   info.Name,
			Params: input.ArgumentsInJSON,
			Result: fmt.Sprintf("[HARD RAILS BLOCKED]: %s", result.Reason),
			Status: "error",
		})
		a.sendEvent(ctx, ev)
		// 返回已取消的 context
		return context.WithValue(ctx, guardBlockedKey{}, true)

	case GuardStatusNeedsApproval:
		req := &ApprovalRequest{
			RequestID: fmt.Sprintf("cmd-%d", len(input.ArgumentsInJSON)),
			Action:    params.Command,
			Impact:    result.Reason,
			SessionID: a.sessionID,
		}
		approved, err := a.approvalMgr.RequestApproval(ctx, req)
		if err != nil || !approved {
			ev := protocol.NewEvent(protocol.EventToolCall, a.sessionID, &protocol.ToolCallPayload{
				Tool:   info.Name,
				Params: input.ArgumentsInJSON,
				Result: "用户拒绝操作或审批超时",
				Status: "error",
			})
			a.sendEvent(ctx, ev)
			return context.WithValue(ctx, guardBlockedKey{}, true)
		}
	}

	return ctx
}

// guardEditFile 拦截 edit_file 工具调用
func (a *CommandGuardAspect) guardEditFile(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	json.Unmarshal([]byte(input.ArgumentsInJSON), &params)

	result := a.guard.GuardFilePath(params.Path, FileOpWrite)

	switch result.Status {
	case GuardStatusBlocked:
		ev := protocol.NewEvent(protocol.EventToolCall, a.sessionID, &protocol.ToolCallPayload{
			Tool:   info.Name,
			Params: input.ArgumentsInJSON,
			Result: fmt.Sprintf("[HARD RAILS BLOCKED]: %s", result.Reason),
			Status: "error",
		})
		a.sendEvent(ctx, ev)
		return context.WithValue(ctx, guardBlockedKey{}, true)

	case GuardStatusNeedsApproval:
		req := &ApprovalRequest{
			RequestID: fmt.Sprintf("edit-%s", params.Path),
			Action:    fmt.Sprintf("write %s", params.Path),
			Impact:    result.Reason,
			SessionID: a.sessionID,
		}
		approved, err := a.approvalMgr.RequestApproval(ctx, req)
		if err != nil || !approved {
			ev := protocol.NewEvent(protocol.EventToolCall, a.sessionID, &protocol.ToolCallPayload{
				Tool:   info.Name,
				Params: input.ArgumentsInJSON,
				Result: "用户拒绝操作或审批超时",
				Status: "error",
			})
			a.sendEvent(ctx, ev)
			return context.WithValue(ctx, guardBlockedKey{}, true)
		}
	}

	return ctx
}

// guardBlockedKey 用于在 context 中标记被拦截的请求
type guardBlockedKey struct{}

// ResolveApprovalByAction 通过动作描述解析审批（测试辅助方法）
func (am *ApprovalManager) ResolveApprovalByAction(action string, approved bool) {
	am.mu.Lock()
	defer am.mu.Unlock()
	for _, ch := range am.pending {
		ch <- approved
		return
	}
}
