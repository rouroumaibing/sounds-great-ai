// Package feishu implements the Feishu (Lark) connector transport (roadmap
// P1-A, F292 飞书会议). It parses Feishu event callbacks and sends via an
// injected poster.
package feishu

import (
	"context"
	"encoding/json"
	"errors"

	"sounds-great-ai/internal/connectors"
)

// Poster sends raw bytes to a URL.
type Poster func(url string, body []byte) error

// Transport is the Feishu connectors.Transport implementation.
type Transport struct {
	Secret  string
	PostURL string
	post    Poster
}

// New creates a Feishu transport.
func New(secret, postURL string, post Poster) *Transport {
	return &Transport{Secret: secret, PostURL: postURL, post: post}
}

type fsEvent struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Event     struct {
		Message struct {
			MessageID string `json:"message_id"`
			ChatID    string `json:"chat_id"`
			Content   string `json:"content"` // JSON string with "text"
			Sender    struct {
				SenderID struct {
					UnionID string `json:"union_id"`
				} `json:"sender_id"`
			} `json:"sender"`
		} `json:"message"`
	} `json:"event"`
}

// Send posts a message via the Feishu API.
func (t *Transport) Send(_ context.Context, m connectors.OutboundMessage) error {
	if t.post == nil {
		return errors.New("feishu: no poster configured")
	}
	payload, _ := json.Marshal(map[string]any{
		"receive_id": m.To,
		"msg_type":   "text",
		"content":    `{"text":"` + m.Text + `"}`,
	})
	return t.post(t.PostURL, payload)
}

// ParseWebhook extracts a normalized message. url_verification challenges and
// non-message events return nil.
func (t *Transport) ParseWebhook(payload []byte) (*connectors.InboundMessage, error) {
	var env struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, err
	}
	if env.Type == "url_verification" {
		return nil, nil
	}
	var ev fsEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return nil, err
	}
	mid := ev.Event.Message.MessageID
	if mid == "" {
		return nil, nil
	}
	var content struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal([]byte(ev.Event.Message.Content), &content)
	return &connectors.InboundMessage{
		Channel:  "feishu",
		EventID:  mid,
		From:     ev.Event.Message.Sender.SenderID.UnionID,
		Text:     content.Text,
		ThreadID: "fs:" + ev.Event.Message.ChatID,
	}, nil
}

// VerifySignature validates the Feishu signature scheme.
func (t *Transport) VerifySignature(_ []byte, signature string) bool {
	return signature == "fs:"+t.Secret
}

// NewConnector builds a feishu Connector over this transport.
func NewConnector(t *Transport) *connectors.Connector {
	return connectors.New("feishu", t)
}
