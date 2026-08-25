package object

import "testing"

func TestRegistry_ReadyFailClosedUnknown(t *testing.T) {
	r := NewRegistry()
	if r.Ready("ghost") {
		t.Fatal("unknown object must not be assumed ready (fail-closed)")
	}
}

func TestRegistry_SetAndReady(t *testing.T) {
	r := NewRegistry()
	r.Set(Readiness{ID: "widget", Kind: "ui", Ready: true})
	if !r.Ready("widget") {
		t.Fatal("ready widget must report ready")
	}
	r.Set(Readiness{ID: "widget", Kind: "ui", Ready: false, Reason: "loading"})
	if r.Ready("widget") {
		t.Fatal("disabled widget must report not ready")
	}
	sig, ok := r.Signal("widget")
	if !ok || sig.Reason != "loading" {
		t.Fatalf("signal wrong: %+v ok=%v", sig, ok)
	}
}
