package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"sounds-great-ai/internal/connectors"
)

func TestTelegram_ParseAndSignature(t *testing.T) {
	tr := New("tok", "https://tg", nil)
	payload, _ := json.Marshal(map[string]any{
		"message": map[string]any{
			"message_id": 42,
			"chat":       map[string]any{"id": 99},
			"from":       map[string]any{"username": "bob"},
			"text":       "hi",
		},
	})
	msg, err := tr.ParseWebhook(payload)
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.EventID != "42" || msg.From != "bob" || msg.ThreadID != "chat:99" {
		t.Fatalf("parse mismatch: %+v", msg)
	}
	c := connectors.New("telegram", tr)
	if _, err := c.ReceiveWebhook(context.Background(), payload, "tg:tok"); err != nil {
		t.Fatalf("good sig should pass: %v", err)
	}
	if _, err := c.ReceiveWebhook(context.Background(), payload, "bad"); !errors.Is(err, connectors.ErrBadSignature) {
		t.Fatalf("bad sig must fail: %v", err)
	}
	// idempotent
	if _, err := c.ReceiveWebhook(context.Background(), payload, "tg:tok"); !errors.Is(err, connectors.ErrDuplicateEvent) {
		t.Fatalf("duplicate must error: %v", err)
	}
}
