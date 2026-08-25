package weixin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"sounds-great-ai/internal/connectors"
)

func TestWeixin_ParseAndSignature(t *testing.T) {
	tr := New("sec", "https://wx", nil)
	payload, _ := json.Marshal(map[string]any{
		"FromUserName": "userA", "ToUserName": "bot", "MsgId": "M1", "Content": "hello",
	})
	msg, err := tr.ParseWebhook(payload)
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.EventID != "M1" || msg.From != "userA" {
		t.Fatalf("parse mismatch: %+v", msg)
	}
	c := connectors.New("weixin", tr)
	if _, err := c.ReceiveWebhook(context.Background(), payload, "wx:sec"); err != nil {
		t.Fatalf("good sig should pass: %v", err)
	}
	if _, err := c.ReceiveWebhook(context.Background(), payload, "bad"); !errors.Is(err, connectors.ErrBadSignature) {
		t.Fatalf("bad sig must fail: %v", err)
	}
	if _, err := c.ReceiveWebhook(context.Background(), payload, "wx:sec"); !errors.Is(err, connectors.ErrDuplicateEvent) {
		t.Fatalf("duplicate must error: %v", err)
	}
}
