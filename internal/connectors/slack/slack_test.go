package slack

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"sounds-great-ai/internal/connectors"
)

func TestSlack_ParseWebhookMessage(t *testing.T) {
	tr := New("sec", "https://slack", nil)
	payload, _ := json.Marshal(map[string]any{
		"type": "event_callback",
		"event": map[string]any{
			"type":      "message",
			"event_id":  "Ev123",
			"channel":   "C1",
			"user":      "U1",
			"text":      "hello",
			"thread_ts": "161",
		},
	})
	msg, err := tr.ParseWebhook(payload)
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.EventID != "Ev123" || msg.From != "U1" || msg.ThreadID != "C1:161" {
		t.Fatalf("parse mismatch: %+v", msg)
	}
}

func TestSlack_IgnoreChallenge(t *testing.T) {
	tr := New("sec", "https://slack", nil)
	payload, _ := json.Marshal(map[string]any{"type": "url_verification", "challenge": "abc"})
	msg, err := tr.ParseWebhook(payload)
	if err != nil || msg != nil {
		t.Fatalf("challenge must be ignored: %v %+v", err, msg)
	}
}

func TestSlack_FailClosedSignature(t *testing.T) {
	tr := New("sec", "https://slack", nil)
	c := connectors.New("slack", tr)
	if tr.VerifySignature(nil, "wrong") {
		t.Fatal("bad signature must fail")
	}
	payload, _ := json.Marshal(map[string]any{
		"type":  "event_callback",
		"event": map[string]any{"type": "message", "event_id": "E1", "user": "U1", "text": "x", "channel": "C1"},
	})
	if _, err := c.ReceiveWebhook(context.Background(), payload, "wrong"); !errors.Is(err, connectors.ErrBadSignature) {
		t.Fatalf("bad sig must error: %v", err)
	}
	// good signature passes
	if _, err := c.ReceiveWebhook(context.Background(), payload, "v0=sec"); err != nil {
		t.Fatalf("good sig should pass: %v", err)
	}
}

func TestSlack_SendUsesPoster(t *testing.T) {
	var gotURL string
	var gotBody []byte
	tr := New("sec", "https://slack/x", func(u string, b []byte) error {
		gotURL = u
		gotBody = b
		return nil
	})
	c := connectors.New("slack", tr)
	if err := c.Send(context.Background(), connectors.OutboundMessage{To: "C1", Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if gotURL != "https://slack/x" || len(gotBody) == 0 {
		t.Fatalf("poster not invoked: url=%q", gotURL)
	}
}

func TestSlack_ConnectorIdempotent(t *testing.T) {
	tr := New("sec", "https://slack", nil)
	c := connectors.New("slack", tr)
	payload, _ := json.Marshal(map[string]any{
		"type":  "event_callback",
		"event": map[string]any{"type": "message", "event_id": "E1", "user": "U1", "text": "x", "channel": "C1"},
	})
	if _, err := c.ReceiveWebhook(context.Background(), payload, "v0=sec"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ReceiveWebhook(context.Background(), payload, "v0=sec"); err == nil {
		t.Fatal("duplicate event must error")
	}
}
