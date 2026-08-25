package media

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"sounds-great-ai/internal/connectors"
)

func TestMedia_ParseAndSignature(t *testing.T) {
	tr := New("sec", "https://media", nil)
	payload, _ := json.Marshal(map[string]any{
		"event_id": "evt1", "type": "media.published", "source": "yt", "url": "x", "text": "done",
	})
	msg, err := tr.ParseWebhook(payload)
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.EventID != "evt1" || msg.From != "yt" {
		t.Fatalf("parse mismatch: %+v", msg)
	}
	c := connectors.New("media", tr)
	if _, err := c.ReceiveWebhook(context.Background(), payload, "media:sec"); err != nil {
		t.Fatalf("good sig should pass: %v", err)
	}
	if _, err := c.ReceiveWebhook(context.Background(), payload, "bad"); !errors.Is(err, connectors.ErrBadSignature) {
		t.Fatalf("bad sig must fail: %v", err)
	}
	if _, err := c.ReceiveWebhook(context.Background(), payload, "media:sec"); !errors.Is(err, connectors.ErrDuplicateEvent) {
		t.Fatalf("duplicate must error: %v", err)
	}
}
