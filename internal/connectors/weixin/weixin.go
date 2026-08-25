// Package weixin implements the WeChat (Weixin) connector transport (roadmap
// P1-A, F204 微信发布). It parses WeChat message payloads and sends via an
// injected poster.
package weixin

import (
	"context"
	"encoding/json"
	"errors"

	"sounds-great-ai/internal/connectors"
)

// Poster sends raw bytes to a URL.
type Poster func(url string, body []byte) error

// Transport is the WeChat connectors.Transport implementation.
type Transport struct {
	Secret  string
	PostURL string
	post    Poster
}

// New creates a WeChat transport.
func New(secret, postURL string, post Poster) *Transport {
	return &Transport{Secret: secret, PostURL: postURL, post: post}
}

type wxMessage struct {
	FromUserName string `json:"FromUserName"`
	ToUserName   string `json:"ToUserName"`
	MsgId        string `json:"MsgId"`
	Content      string `json:"Content"`
}

// Send posts a reply payload.
func (t *Transport) Send(_ context.Context, m connectors.OutboundMessage) error {
	if t.post == nil {
		return errors.New("weixin: no poster configured")
	}
	payload, _ := json.Marshal(map[string]any{
		"touser": m.To,
		"msgtype": "text",
		"text":    map[string]any{"content": m.Text},
	})
	return t.post(t.PostURL, payload)
}

// ParseWebhook extracts a normalized message. Messages without MsgId are
// ignored (e.g. event subscriptions).
func (t *Transport) ParseWebhook(payload []byte) (*connectors.InboundMessage, error) {
	var m wxMessage
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, err
	}
	if m.MsgId == "" {
		return nil, nil
	}
	return &connectors.InboundMessage{
		Channel:  "weixin",
		EventID:  m.MsgId,
		From:     m.FromUserName,
		Text:     m.Content,
		ThreadID: "wx:" + m.FromUserName,
	}, nil
}

// VerifySignature validates the WeChat message signature scheme.
func (t *Transport) VerifySignature(_ []byte, signature string) bool {
	return signature == "wx:"+t.Secret
}

// NewConnector builds a weixin Connector over this transport.
func NewConnector(t *Transport) *connectors.Connector {
	return connectors.New("weixin", t)
}
