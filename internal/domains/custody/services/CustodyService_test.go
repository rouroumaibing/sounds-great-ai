package services

import (
	"context"
	"testing"
	"time"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	custodyStores "sounds-great-ai/internal/domains/custody/stores"
)

func TestCustodyService_AcquireAndRelease(t *testing.T) {
	store := custodyStores.NewMemoryCustodyStore()
	svc := NewCustodyService(store, 5*time.Minute)

	lease, err := svc.Acquire(context.Background(), "breed-A")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if lease.Subject != "breed-A" {
		t.Fatalf("expected subject breed-A, got %s", lease.Subject)
	}
	if lease.Generation != 1 {
		t.Fatalf("expected generation 1, got %d", lease.Generation)
	}

	holder, err := svc.CurrentHolder(context.Background(), "breed-A")
	if err != nil {
		t.Fatalf("current holder: %v", err)
	}
	if holder.ID != lease.ID {
		t.Fatal("holder ID mismatch")
	}

	if err := svc.Release(context.Background(), lease); err != nil {
		t.Fatalf("release: %v", err)
	}

	_, err = svc.CurrentHolder(context.Background(), "breed-A")
	if err == nil {
		t.Fatal("expected error after release")
	}
}

func TestCustodyService_DuplicateAcquireFails(t *testing.T) {
	store := custodyStores.NewMemoryCustodyStore()
	svc := NewCustodyService(store, 5*time.Minute)

	if _, err := svc.Acquire(context.Background(), "breed-A"); err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	_, err := svc.Acquire(context.Background(), "breed-A")
	if err == nil {
		t.Fatal("expected error on duplicate acquire")
	}
}

func TestCustodyService_IsStale(t *testing.T) {
	store := custodyStores.NewMemoryCustodyStore()
	svc := NewCustodyService(store, 1*time.Millisecond)

	lease, _ := svc.Acquire(context.Background(), "breed-A")
	time.Sleep(5 * time.Millisecond)

	if !svc.IsStale(context.Background(), lease) {
		t.Fatal("expected lease to be stale")
	}
}

func TestCustodyService_CAS(t *testing.T) {
	store := custodyStores.NewMemoryCustodyStore()
	svc := NewCustodyService(store, 5*time.Minute)

	lease, _ := svc.Acquire(context.Background(), "breed-A")

	err := store.CAS(context.Background(), lease.ID, 1, 2)
	if err != nil {
		t.Fatalf("CAS: %v", err)
	}

	err = store.CAS(context.Background(), lease.ID, 1, 3)
	if err == nil {
		t.Fatal("expected CAS to fail with stale generation")
	}

	_ = custodyPorts.LeaseID("test")
}
