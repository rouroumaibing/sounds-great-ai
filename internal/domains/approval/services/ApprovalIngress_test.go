package services

import (
	"context"
	"testing"

	approvalPorts "sounds-great-ai/internal/domains/approval/ports"
	approvalStores "sounds-great-ai/internal/domains/approval/stores"
)

func TestApprovalIngress_SubmitAndResolve(t *testing.T) {
	store := approvalStores.NewMemoryApprovalStore()
	ingress := NewApprovalIngress(store, nil)

	item := approvalPorts.ApprovalItem{
		Type:  "breed_config",
		Title: "Update breed A",
		Detail: "Change model to gpt-4",
	}

	id, err := ingress.Submit(context.Background(), item)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	pending, err := ingress.ListPending(context.Background(), "user1")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].Status != "pending" {
		t.Fatalf("expected status pending, got %s", pending[0].Status)
	}

	if err := ingress.Resolve(context.Background(), id, approvalPorts.DecisionApproved); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	pending, _ = ingress.ListPending(context.Background(), "user1")
	if len(pending) != 0 {
		t.Fatal("expected 0 pending after resolve")
	}

	settled, err := ingress.ListSettled(context.Background(), "user1", 10)
	if err != nil {
		t.Fatalf("list settled: %v", err)
	}
	if len(settled) != 1 {
		t.Fatalf("expected 1 settled, got %d", len(settled))
	}
	if settled[0].Status != "approved" {
		t.Fatalf("expected status approved, got %s", settled[0].Status)
	}
}

func TestApprovalIngress_ResolveNonPending(t *testing.T) {
	store := approvalStores.NewMemoryApprovalStore()
	ingress := NewApprovalIngress(store, nil)

	item := approvalPorts.ApprovalItem{
		Type:  "test",
		Title: "Test",
	}
	id, _ := ingress.Submit(context.Background(), item)
	ingress.Resolve(context.Background(), id, approvalPorts.DecisionApproved)

	err := ingress.Resolve(context.Background(), id, approvalPorts.DecisionRejected)
	if err == nil {
		t.Fatal("expected error resolving non-pending item")
	}
}
