package services

import (
	"context"
	"testing"

	approvalPorts "sounds-great-ai/internal/domains/approval/ports"
	approvalStores "sounds-great-ai/internal/domains/approval/stores"
)

func TestApprovalIngress_DualAnchorAndCQRS(t *testing.T) {
	store := approvalStores.NewMemoryApprovalStore()
	a := NewApprovalIngress(store, nil)

	id, err := a.Submit(context.Background(), approvalPorts.ApprovalItem{Type: "distill", Title: "t", Detail: "d"})
	if err != nil {
		t.Fatal(err)
	}
	// CQRS: item appears in the pending projection immediately.
	if got := a.Projection().ByStatus("pending"); len(got) != 1 {
		t.Fatalf("expected 1 pending in projection, got %d", len(got))
	}
	// Dual-anchor: request anchor captured, decision anchor empty pre-resolve.
	it, _ := store.Get(context.Background(), id)
	if it.RequestAnchor == "" {
		t.Fatal("request anchor not captured at submit")
	}
	if it.DecisionAnchor != "" {
		t.Fatal("decision anchor must be empty before resolve")
	}

	if err := a.Resolve(context.Background(), id, approvalPorts.DecisionApproved); err != nil {
		t.Fatal(err)
	}
	// CQRS: moved out of pending into approved.
	if got := a.Projection().ByStatus("pending"); len(got) != 0 {
		t.Fatalf("item should leave pending in projection, got %d", len(got))
	}
	if got := a.Projection().ByStatus(string(approvalPorts.DecisionApproved)); len(got) != 1 {
		t.Fatalf("expected 1 approved in projection, got %d", len(got))
	}
	// Decision anchor captured.
	it, _ = store.Get(context.Background(), id)
	if it.DecisionAnchor == "" {
		t.Fatal("decision anchor not captured at resolve")
	}
}

func TestApprovalIngress_RejectionBackflow(t *testing.T) {
	store := approvalStores.NewMemoryApprovalStore()
	var backflow approvalPorts.ApprovalItem
	done := make(chan struct{}, 1)
	a := NewApprovalIngress(store, nil)
	a.SetResolvedNotifier(func(item approvalPorts.ApprovalItem) {
		backflow = item
		select {
		case done <- struct{}{}:
		default:
		}
	})

	id, _ := a.Submit(context.Background(), approvalPorts.ApprovalItem{Type: "distill", Title: "t"})
	if err := a.ResolveWithReason(context.Background(), id, approvalPorts.DecisionRejected, "insufficient evidence"); err != nil {
		t.Fatal(err)
	}
	<-done
	if backflow.RejectionReason != "insufficient evidence" {
		t.Fatalf("rejection reason not backflowed: %q", backflow.RejectionReason)
	}
	it, _ := store.Get(context.Background(), id)
	if it.RejectionReason != "insufficient evidence" {
		t.Fatalf("store missing rejection reason: %q", it.RejectionReason)
	}
}
