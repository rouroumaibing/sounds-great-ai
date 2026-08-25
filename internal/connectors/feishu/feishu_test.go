package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"sounds-great-ai/internal/connectors"
)

func TestFeishu_ParseAndSignature(t *testing.T) {
	tr := New("sec", "https://fs", nil)
	payload, _ := json.Marshal(map[string]any{
		"type": "event_callback",
		"event": map[string]any{
			"message": map[string]any{
				"message_id": "m1",
				"chat_id":    "oc1",
				"content":    `{"text":"hello"}`,
				"sender":     map[string]any{"sender_id": map[string]any{"union_id": "u1"}},
			},
		},
	})
	msg, err := tr.ParseWebhook(payload)
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.EventID != "m1" || msg.From != "u1" || msg.Text != "hello" {
		t.Fatalf("parse mismatch: %+v", msg)
	}
	c := connectors.New("feishu", tr)
	if _, err := c.ReceiveWebhook(context.Background(), payload, "fs:sec"); err != nil {
		t.Fatalf("good sig should pass: %v", err)
	}
	if _, err := c.ReceiveWebhook(context.Background(), payload, "bad"); !errors.Is(err, connectors.ErrBadSignature) {
		t.Fatalf("bad sig must fail: %v", err)
	}
	if _, err := c.ReceiveWebhook(context.Background(), payload, "fs:sec"); !errors.Is(err, connectors.ErrDuplicateEvent) {
		t.Fatalf("duplicate must error: %v", err)
	}
}
