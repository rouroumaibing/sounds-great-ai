// Package telegram implements the Telegram connector transport (roadmap P1-A).
package telegram

import (
	"context"
	"encoding/json"
	"errors"

	"sounds-great-ai/internal/connectors"
)

// Poster sends raw bytes to a URL.
type Poster func(url string, body []byte) error

// Transport is the Telegram connectors.Transport implementation.
type Transport struct {
	Secret  string // bot token (used in signature scheme here)
	PostURL string
	post    Poster
}

// New creates a Telegram transport.
func New(token, postURL string, post Poster) *Transport {
	return &Transport{Secret: token, PostURL: postURL, post: post}
}

type tgUpdate struct {
	Message struct {
		MessageID int `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			Username string `json:"username"`
		} `json:"from"`
		Text string `json:"text"`
	} `json:"message"`
}

// Send posts a message via the Bot API.
func (t *Transport) Send(_ context.Context, m connectors.OutboundMessage) error {
	if t.post == nil {
		return errors.New("telegram: no poster configured")
	}
	payload, _ := json.Marshal(map[string]any{"chat_id": m.To, "text": m.Text})
	return t.post(t.PostURL, payload)
}

// ParseWebhook extracts a normalized message. Non-message updates return nil.
func (t *Transport) ParseWebhook(payload []byte) (*connectors.InboundMessage, error) {
	var u tgUpdate
	if err := json.Unmarshal(payload, &u); err != nil {
		return nil, err
	}
	if u.Message.MessageID == 0 {
		return nil, nil
	}
	return &connectors.InboundMessage{
		Channel:  "telegram",
		EventID:  itoa(u.Message.MessageID),
		From:     u.Message.From.Username,
		Text:     u.Message.Text,
		ThreadID: "chat:" + itoa64(u.Message.Chat.ID),
	}, nil
}

// VerifySignature checks the injected token scheme.
func (t *Transport) VerifySignature(_ []byte, signature string) bool {
	return signature == "tg:"+t.Secret
}

// NewConnector builds a telegram Connector over this transport.
func NewConnector(t *Transport) *connectors.Connector {
	return connectors.New("telegram", t)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
