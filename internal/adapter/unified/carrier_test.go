package unified

import (
	"context"
	"fmt"
	"testing"
)

type fakeTransport struct {
	kind TransportKind
	fail bool
}

func (f *fakeTransport) Kind() TransportKind { return f.kind }
func (f *fakeTransport) Spawn(_ context.Context, _ *SpawnSpec) (*SpawnHandle, error) {
	if f.fail {
		return nil, fmt.Errorf("boom from %s", f.kind)
	}
	return &SpawnHandle{}, nil
}

func TestRegistry_FallsBackToAvailableTier(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry(nil)
	r.RegisterCarrier(&Carrier{ID: "c", Provider: "claude", Transports: []TransportKind{TransportBgDaemon, TransportPrintSDK}})
	// bg_daemon not registered → should fall back to print_sdk
	r.RegisterTransport(&fakeTransport{kind: TransportPrintSDK})

	h, err := r.Execute(ctx, "c", &SpawnSpec{Command: "claude"})
	if err != nil {
		t.Fatalf("expected fallback success, got %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil handle")
	}
}

func TestRegistry_UsesFirstWorkingTier(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry(nil)
	r.RegisterCarrier(&Carrier{ID: "c", Provider: "claude", Transports: []TransportKind{TransportBgDaemon, TransportPrintSDK}})
	r.RegisterTransport(&fakeTransport{kind: TransportBgDaemon})
	r.RegisterTransport(&fakeTransport{kind: TransportPrintSDK})

	h, err := r.Execute(ctx, "c", &SpawnSpec{Command: "claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("expected handle from bg_daemon tier")
	}
}

func TestRegistry_SkipsDegradedTier(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry(NewMemoryHealth())
	r.RegisterCarrier(&Carrier{ID: "c", Provider: "claude", Transports: []TransportKind{TransportBgDaemon, TransportPrintSDK}})
	r.RegisterTransport(&fakeTransport{kind: TransportBgDaemon})
	r.RegisterTransport(&fakeTransport{kind: TransportPrintSDK})

	// mark bg_daemon tier as degraded (offline via quota reason)
	r.health.RecordFailure(ctx, "c/"+string(TransportBgDaemon), ReasonQuotaExceeded)

	h, err := r.Execute(ctx, "c", &SpawnSpec{Command: "claude"})
	if err != nil {
		t.Fatalf("expected skip+fallback, got %v", err)
	}
	if h == nil {
		t.Fatal("expected handle from print_sdk after skipping degraded bg_daemon")
	}
}

func TestRegistry_AllTiersFail(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry(nil)
	r.RegisterCarrier(&Carrier{ID: "c", Provider: "claude", Transports: []TransportKind{TransportBgDaemon, TransportPrintSDK}})
	r.RegisterTransport(&fakeTransport{kind: TransportBgDaemon, fail: true})
	r.RegisterTransport(&fakeTransport{kind: TransportPrintSDK, fail: true})

	_, err := r.Execute(ctx, "c", &SpawnSpec{Command: "claude"})
	if err == nil {
		t.Fatal("expected error when all tiers fail")
	}
}
