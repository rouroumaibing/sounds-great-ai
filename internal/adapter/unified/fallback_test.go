package unified

import (
	"context"
	"fmt"
	"testing"
)

// fakeTransport (defined in carrier_test.go) returns a bare SpawnHandle; the
// fallback wrapper drives the stream via the test's streamFn, which ignores the
// handle, so no live process is needed.

// errTransport simulates a transport whose Spawn fails with a fixed error — the
// the "thrown" spawn-failure path.
type errTransport struct {
	kind TransportKind
	err  string
}

func (e *errTransport) Kind() TransportKind { return e.kind }

func (e *errTransport) Spawn(_ context.Context, _ *SpawnSpec) (*SpawnHandle, error) {
	return nil, fmt.Errorf("%s", e.err)
}

func collectEvents(ch <-chan StreamEvent) []StreamEvent {
	var out []StreamEvent
	for e := range ch {
		out = append(out, e)
	}
	return out
}

// TestRunCarrierFallbackSwapsOnThrownFatal exercises the R1 THROWN
// path: the first tier's Spawn fails with a quota (429) error; the wrapper
// records the failure for the next call, emits a carrier_fallback notice, and
// re-runs the prompt on the next transport, which succeeds. Exactly one hop.
func TestRunCarrierFallbackSwapsOnThrownFatal(t *testing.T) {
	reg := NewRegistry(nil)
	reg.RegisterTransport(&errTransport{kind: TransportBgDaemon, err: "rate limit 429 exceeded"})
	reg.RegisterTransport(&fakeTransport{kind: TransportPrintSDK})
	reg.RegisterCarrier(&Carrier{ID: "claude", Transports: []TransportKind{TransportBgDaemon, TransportPrintSDK}})

	callCount := 0
	streamFn := func(h *SpawnHandle) <-chan StreamEvent {
		callCount++
		c := make(chan StreamEvent, 8)
		go func() {
			c <- StreamEvent{Type: "text", Content: "tier1-success"}
			c <- StreamEvent{Type: "done"}
			close(c)
		}()
		return c
	}

	ch, err := RunCarrierFallback(context.Background(), reg, "claude", &SpawnSpec{}, streamFn)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	events := collectEvents(ch)

	// tier0 errored at Spawn, so streamFn is only called for tier1.
	if callCount != 1 {
		t.Fatalf("expected streamFn called once (tier0 errored before streaming), got %d", callCount)
	}
	var hasFallback, hasTier1 bool
	for _, e := range events {
		if e.Type == "system_info" && e.Meta["kind"] == "carrier_fallback" {
			hasFallback = true
		}
		if e.Type == "text" && e.Content == "tier1-success" {
			hasTier1 = true
		}
	}
	if !hasFallback {
		t.Errorf("expected a carrier_fallback system_info event")
	}
	if !hasTier1 {
		t.Errorf("expected tier1 success text in output")
	}
}

// TestRunCarrierFallbackNoSwapOnYieldedFatal exercises the R1 YIELDED
// path: a fatal error surfaces mid-stream as an error event. Per the design, this
// does NOT switch carriers mid-stream (partial output may already be yielded) —
// the event is recorded for the NEXT call and passed through unchanged. Only
// one tier is ever attempted.
func TestRunCarrierFallbackNoSwapOnYieldedFatal(t *testing.T) {
	reg := NewRegistry(nil)
	reg.RegisterTransport(&fakeTransport{kind: TransportBgDaemon})
	reg.RegisterTransport(&fakeTransport{kind: TransportPrintSDK})
	reg.RegisterCarrier(&Carrier{ID: "claude", Transports: []TransportKind{TransportBgDaemon, TransportPrintSDK}})

	callCount := 0
	streamFn := func(h *SpawnHandle) <-chan StreamEvent {
		callCount++
		c := make(chan StreamEvent, 8)
		go func() {
			c <- StreamEvent{Type: "text", Content: "partial-output"}
			c <- StreamEvent{Type: "error", Content: "rate limit 429 exceeded"}
			c <- StreamEvent{Type: "done"}
			close(c)
		}()
		return c
	}
	ch, _ := RunCarrierFallback(context.Background(), reg, "claude", &SpawnSpec{}, streamFn)
	events := collectEvents(ch)

	// R1 semantics: yielded fatal is NOT a mid-stream switch.
	if callCount != 1 {
		t.Fatalf("expected streamFn called once (no mid-stream swap), got %d", callCount)
	}
	var hasFallback, hasYieldedError, hasPartial bool
	for _, e := range events {
		if e.Meta["kind"] == "carrier_fallback" {
			hasFallback = true
		}
		if e.Type == "error" {
			hasYieldedError = true
		}
		if e.Type == "text" && e.Content == "partial-output" {
			hasPartial = true
		}
	}
	if hasFallback {
		t.Errorf("R1: yielded fatal must NOT trigger a mid-stream carrier_fallback")
	}
	if !hasYieldedError {
		t.Errorf("expected the yielded error event to pass through")
	}
	if !hasPartial {
		t.Errorf("expected partial output to be preserved (no buffer-drop)")
	}
}

// TestRunCarrierFallbackExhaustsChainEmitsError confirms that when every tier
// fails at Spawn, exactly one fallback notice is emitted and a terminal error
// event closes the stream (no silent hang). No cascading beyond the single hop.
func TestRunCarrierFallbackExhaustsChainEmitsError(t *testing.T) {
	reg := NewRegistry(nil)
	reg.RegisterTransport(&errTransport{kind: TransportBgDaemon, err: "quota 429"})
	reg.RegisterTransport(&errTransport{kind: TransportPrintSDK, err: "invalid config"})
	reg.RegisterCarrier(&Carrier{ID: "claude", Transports: []TransportKind{TransportBgDaemon, TransportPrintSDK}})

	callCount := 0
	streamFn := func(h *SpawnHandle) <-chan StreamEvent {
		callCount++
		c := make(chan StreamEvent, 8)
		close(c)
		return c
	}
	ch, _ := RunCarrierFallback(context.Background(), reg, "claude", &SpawnSpec{}, streamFn)
	events := collectEvents(ch)

	if callCount != 0 {
		t.Fatalf("expected streamFn never called (both tiers errored at Spawn), got %d", callCount)
	}
	last := events[len(events)-1]
	if last.Type != "error" {
		t.Errorf("expected terminal error event, got %+v", last)
	}
	fallbacks := 0
	for _, e := range events {
		if e.Meta["kind"] == "carrier_fallback" {
			fallbacks++
		}
	}
	if fallbacks != 1 {
		t.Errorf("expected exactly 1 fallback notice (single hop, no cascade), got %d", fallbacks)
	}
}

// TestRunCarrierFallbackRecordsFailureForNextCall verifies (b): after a thrown
// fatal, the degraded tier's health is recorded so the NEXT call skips it.
func TestRunCarrierFallbackRecordsFailureForNextCall(t *testing.T) {
	health := NewMemoryHealth()
	reg := NewRegistry(health)
	reg.RegisterTransport(&errTransport{kind: TransportBgDaemon, err: "quota 429 exceeded"})
	reg.RegisterTransport(&fakeTransport{kind: TransportPrintSDK})
	reg.RegisterCarrier(&Carrier{ID: "claude", Transports: []TransportKind{TransportBgDaemon, TransportPrintSDK}})

	streamFn := func(h *SpawnHandle) <-chan StreamEvent {
		c := make(chan StreamEvent, 8)
		go func() {
			c <- StreamEvent{Type: "text", Content: "tier1-ok"}
			c <- StreamEvent{Type: "done"}
			close(c)
		}()
		return c
	}
	ch, _ := RunCarrierFallback(context.Background(), reg, "claude", &SpawnSpec{}, streamFn)
	collectEvents(ch) // drain

	if lvl := health.Level(context.Background(), "claude/"+string(TransportBgDaemon)); lvl != "offline" {
		t.Errorf("expected bg_daemon to be offline after quota failure, got %q", lvl)
	}
	if lvl := health.Level(context.Background(), "claude/"+string(TransportPrintSDK)); lvl != "online" {
		t.Errorf("expected print_sdk to stay online, got %q", lvl)
	}
}

func TestIsFatalMidStream(t *testing.T) {
	cases := map[string]bool{
		"rate limit 429 exceeded":    true,
		"Quota exceeded":             true,
		"authentication failed":      true,
		"context window exceeded":    true,
		"model not found":            true,
		"service unavailable 503":    true,
		"upstream policy rejected":   true,
		"network error ECONNREFUSED": false, // transient -> not fatal
		"some generic failure":       false,
		"":                           false,
	}
	for text, want := range cases {
		if got := IsFatalMidStream(text); got != want {
			t.Errorf("IsFatalMidStream(%q) = %v, want %v", text, got, want)
		}
	}
}
