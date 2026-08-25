// Package media implements the media-delivery connector transport (roadmap
// P1-A, F205 MediaHub / F172 生图归档). It normalizes media-publish webhooks
// and posts delivery notifications via an injected poster.
package media

import (
	"context"
	"encoding/json"
	"errors"

	"sounds-great-ai/internal/connectors"
)

// Poster sends raw bytes to a URL.
type Poster func(url string, body []byte) error

// Transport is the media connectors.Transport implementation.
type Transport struct {
	Secret  string
	PostURL string
	post    Poster
}

// New creates a media transport.
func New(secret, postURL string, post Poster) *Transport {
	return &Transport{Secret: secret, PostURL: postURL, post: post}
}

type mediaEvent struct {
	EventID string `json:"event_id"`
	Type    string `json:"type"` // e.g. "media.published"
	Source  string `json:"source"`
	URL     string `json:"url"`
	Text    string `json:"text"`
}

// Send posts a delivery notification.
func (t *Transport) Send(_ context.Context, m connectors.OutboundMessage) error {
	if t.post == nil {
		return errors.New("media: no poster configured")
	}
	payload, _ := json.Marshal(map[string]any{"to": m.To, "text": m.Text})
	return t.post(t.PostURL, payload)
}

// ParseWebhook extracts a normalized message. Non-publish events return nil.
func (t *Transport) ParseWebhook(payload []byte) (*connectors.InboundMessage, error) {
	var e mediaEvent
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, err
	}
	if e.EventID == "" || e.Type != "media.published" {
		return nil, nil
	}
	return &connectors.InboundMessage{
		Channel:  "media",
		EventID:  e.EventID,
		From:     e.Source,
		Text:     e.Text,
		ThreadID: "media:" + e.Source,
	}, nil
}

// VerifySignature validates the media signature scheme.
func (t *Transport) VerifySignature(_ []byte, signature string) bool {
	return signature == "media:"+t.Secret
}

// NewConnector builds a media Connector over this transport.
func NewConnector(t *Transport) *connectors.Connector {
	return connectors.New("media", t)
}
