package transport

import (
	"testing"

	"sounds-great-ai/pkg/protocol"
)

func TestGetBufferPutBuffer(t *testing.T) {
	b := GetBuffer()
	if b == nil {
		t.Fatal("GetBuffer returned nil")
	}
	if cap(*b) == 0 {
		t.Error("expected non-zero capacity buffer")
	}

	// Write some data, then return to pool.
	*b = append(*b, "hello world"...)
	if len(*b) != 11 {
		t.Fatalf("len = %d, want 11", len(*b))
	}

	PutBuffer(b)

	// After PutBuffer, length should be reset (we can't inspect the same
	// pointer reliably after Put, so just verify the cycle doesn't panic).
	b2 := GetBuffer()
	if b2 == nil {
		t.Fatal("GetBuffer returned nil after recycle")
	}
	if len(*b2) != 0 {
		t.Errorf("len = %d, want 0 after recycle", len(*b2))
	}
	PutBuffer(b2)
}

func TestGetEventPutEvent(t *testing.T) {
	e := GetEvent()
	if e == nil {
		t.Fatal("GetEvent returned nil")
	}

	// Populate fields.
	e.Type = protocol.EventThinking
	e.SessionID = "session-1"
	e.Seq = 42

	PutEvent(e)

	// After PutEvent, the struct should be zeroed.
	e2 := GetEvent()
	if e2 == nil {
		t.Fatal("GetEvent returned nil after recycle")
	}
	if e2.SessionID != "" {
		t.Errorf("SessionID = %q, want empty after recycle", e2.SessionID)
	}
	if e2.Seq != 0 {
		t.Errorf("Seq = %d, want 0 after recycle", e2.Seq)
	}
	PutEvent(e2)
}
