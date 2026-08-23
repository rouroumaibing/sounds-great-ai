package aspect

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sounds-great-ai/pkg/protocol"
)

// ApprovalRequest 表示一个 HITL 审批请求
type ApprovalRequest struct {
	RequestID string
	Action    string // e.g. "write .env", "git push --force"
	Impact    string // 人类可读的影响描述
	SessionID string
}

// ApprovalManager 管理 HITL 审批流程
type ApprovalManager struct {
	pending     map[string]chan bool // request_id -> approval channel
	mu          sync.Mutex
	eventSender func(ctx context.Context, ev *protocol.Event) // 可选：发送 HITL_APPROVAL 事件
}

// NewApprovalManager 创建一个新的审批管理器
func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{
		pending: make(map[string]chan bool),
	}
}

// SetEventSender 设置事件发送回调（用于发送 HITL_APPROVAL 事件到前端）
func (am *ApprovalManager) SetEventSender(sender func(ctx context.Context, ev *protocol.Event)) {
	am.eventSender = sender
}

// RequestApproval 阻塞直到用户响应或超时
func (am *ApprovalManager) RequestApproval(ctx context.Context, req *ApprovalRequest) (bool, error) {
	ch := make(chan bool, 1)

	am.mu.Lock()
	am.pending[req.RequestID] = ch
	am.mu.Unlock()

	// 发送 HITL_APPROVAL 事件到前端
	if am.eventSender != nil {
		ev := protocol.NewEvent(protocol.EventHITLApproval, req.SessionID, &protocol.HITLApprovalPayload{
			Action:    req.Action,
			Approved:  false,
			RequestID: req.RequestID,
			Impact:    req.Impact,
		})
		am.eventSender(ctx, ev)
	}

	// 清理 pending
	defer func() {
		am.mu.Lock()
		delete(am.pending, req.RequestID)
		am.mu.Unlock()
	}()

	// 等待结果或超时
	select {
	case approved := <-ch:
		return approved, nil
	case <-ctx.Done():
		return false, fmt.Errorf("审批超时，操作已取消: %w", ctx.Err())
	}
}

// IsPending reports whether a request is currently awaiting a decision.
func (am *ApprovalManager) IsPending(requestID string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()
	_, ok := am.pending[requestID]
	return ok
}

// ResolveApproval 用户响应审批时调用。返回是否命中待审请求——false 表示
// 该 request_id 不存在（已超时、已处理或服务重启后丢失）。
func (am *ApprovalManager) ResolveApproval(requestID string, approved bool) bool {
	am.mu.Lock()
	ch, ok := am.pending[requestID]
	if ok {
		delete(am.pending, requestID)
	}
	am.mu.Unlock()

	if ok {
		ch <- approved
	}
	return ok
}

// RequestApprovalWithTimeout 带默认60秒超时的审批请求
func (am *ApprovalManager) RequestApprovalWithTimeout(req *ApprovalRequest) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return am.RequestApproval(ctx, req)
}
