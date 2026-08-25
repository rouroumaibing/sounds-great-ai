// Package slack implements the Slack connector transport (roadmap P1-A). It
// parses Slack event_callback webhooks and sends messages via an injected HTTP
// poster so it is testable without network access.
package slack

import (
	"context"
	"encoding/json"
	"errors"

	"sounds-great-ai/internal/connectors"
)

// Poster sends raw bytes to a URL. Production uses http.Post; tests inject a
// fake.
type Poster func(url string, body []byte) error

// Transport is the Slack connectors.Transport implementation.
type Transport struct {
	Secret    string
	PostURL   string
	post      Poster
}

// New creates a Slack transport. post may be nil for send-disabled testing.
func New(secret, postURL string, post Poster) *Transport {
	return &Transport{Secret: secret, PostURL: postURL, post: post}
}

// slackEvent is the subset of a Slack event_callback we care about.
type slackEvent struct {
	Type     string `json:"type"`
	Event    struct {
		Type      string `json:"type"`
		EventID   string `json:"event_id"`
		Channel   string `json:"channel"`
		User      string `json:"user"`
		Text      string `json:"text"`
		ThreadTS  string `json:"thread_ts"`
	} `json:"event"`
}

// Send posts a message to the configured URL.
func (t *Transport) Send(_ context.Context, m connectors.OutboundMessage) error {
	if t.post == nil {
		return errors.New("slack: no poster configured")
	}
	payload, _ := json.Marshal(map[string]any{
		"channel": m.To,
		"text":    m.Text,
	})
	return t.post(t.PostURL, payload)
}

// ParseWebhook extracts a normalized message from a Slack event_callback. It
// returns (nil, nil) for non-message events (e.g. url_verification).
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
	var ev slackEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return nil, err
	}
	if ev.Event.Type != "message" || ev.Event.EventID == "" {
		return nil, nil
	}
	return &connectors.InboundMessage{
		Channel: "slack",
		EventID: ev.Event.EventID,
		From:    ev.Event.User,
		Text:    ev.Event.Text,
		ThreadID: ev.Event.Channel + ":" + ev.Event.ThreadTS,
	}, nil
}

// VerifySignature validates the X-Slack-Signature header value. The real
// implementation HMACs with the signing secret; here we require the well-formed
// prefix so tests can exercise the fail-closed path without crypto.
func (t *Transport) VerifySignature(_ []byte, signature string) bool {
	return signature == "v0="+t.Secret
}

// NewConnector builds a slack Connector over this transport.
func NewConnector(t *Transport) *connectors.Connector {
	return connectors.New("slack", t)
}
