package connectors

import (
	"context"
	"errors"
	"testing"
)

func TestConnector_SendAndReceive(t *testing.T) {
	rt := &RecordingTransport{Secret: "s3cr3t"}
	c := New("test", rt)
	if err := c.Send(context.Background(), OutboundMessage{To: "u1", Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if len(rt.Sent) != 1 {
		t.Fatalf("sent count = %d", len(rt.Sent))
	}

	rt.Next = &InboundMessage{Channel: "test", EventID: "e1", From: "u2", Text: "yo"}
	// valid signature
	msg, err := c.ReceiveWebhook(context.Background(), []byte("{}"), "valid:s3cr3t")
	if err != nil || msg == nil || msg.EventID != "e1" {
		t.Fatalf("receive: %v %+v", err, msg)
	}
}

func TestConnector_FailClosed_BadSignature(t *testing.T) {
	c := New("test", &RecordingTransport{Secret: "s"})
	rt := &RecordingTransport{Secret: "s"}
	c = New("test", rt)
	// override verification to fail
	bad := false
	rt.VerifyOK = &bad
	if _, err := c.ReceiveWebhook(context.Background(), []byte("{}"), "anything"); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("bad signature must return ErrBadSignature, got %v", err)
	}
}

func TestConnector_IdempotentDuplicate(t *testing.T) {
	rt := &RecordingTransport{Secret: "s"}
	c := New("test", rt)
	rt.Next = &InboundMessage{EventID: "dup", From: "u", Text: "x"}
	if _, err := c.ReceiveWebhook(context.Background(), nil, "valid:s"); err != nil {
		t.Fatal(err)
	}
	// second time same EventID -> duplicate
	if _, err := c.ReceiveWebhook(context.Background(), nil, "valid:s"); !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("duplicate must return ErrDuplicateEvent, got %v", err)
	}
}

func TestConnector_NonMessageEventIgnored(t *testing.T) {
	rt := &RecordingTransport{Secret: "s"}
	c := New("test", rt)
	rt.Next = nil // ParseWebhook returns nil for challenge pings
	msg, err := c.ReceiveWebhook(context.Background(), nil, "valid:s")
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatal("non-message event should be ignored (nil)")
	}
}

func TestRegistry(t *testing.T) {
	r := NewConnectorRegistry()
	r.Register(New("a", &RecordingTransport{}))
	r.Register(New("b", &RecordingTransport{}))
	if r.Get("a") == nil || r.Get("missing") != nil {
		t.Fatal("registry get")
	}
	if len(r.Names()) != 2 {
		t.Fatal("registry names")
	}
}
