package aspect

import (
	"context"
	"testing"
	"time"
)

func TestApprovalManagerApprove(t *testing.T) {
	mgr := NewApprovalManager()

	req := &ApprovalRequest{
		RequestID: "req-approve",
		Action:   "write .env",
		Impact:   "Modifies environment configuration",
		SessionID: "session-1",
	}

	// 在另一个 goroutine 中解析审批
	go func() {
		time.Sleep(100 * time.Millisecond)
		mgr.ResolveApproval("req-approve", true)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	approved, err := mgr.RequestApproval(ctx, req)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}
	if !approved {
		t.Error("expected approval to be granted")
	}
}

func TestApprovalManagerReject(t *testing.T) {
	mgr := NewApprovalManager()

	req := &ApprovalRequest{
		RequestID: "req-reject",
		Action:   "git push --force",
		Impact:   "Force push to remote",
		SessionID: "session-1",
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		mgr.ResolveApproval("req-reject", false)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	approved, err := mgr.RequestApproval(ctx, req)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}
	if approved {
		t.Error("expected approval to be rejected")
	}
}

func TestApprovalManagerTimeout(t *testing.T) {
	mgr := NewApprovalManager()

	req := &ApprovalRequest{
		RequestID: "req-timeout",
		Action:   "dangerous op",
		Impact:   "High risk",
		SessionID: "session-1",
	}

	// 不解析审批，让它超时
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	approved, err := mgr.RequestApproval(ctx, req)
	// 超时应返回错误或 false
	if approved {
		t.Error("expected timeout to result in non-approved")
	}
	_ = err
}
