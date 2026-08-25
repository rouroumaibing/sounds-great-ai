package connectors

import (
	"context"
	"sync"
)

// RecordingTransport is a reusable fake connectors.Transport for tests. It
// records sent messages and replays a canned inbound message on ParseWebhook.
// Signature verification passes only for the exact string "valid:"+Secret.
type RecordingTransport struct {
	Secret string

	mu    sync.Mutex
	Sent  []OutboundMessage
	Next  *InboundMessage

	// VerifyOK overrides signature behavior when set (for testing the
	// fail-closed path). If nil, verification is sig == "valid:"+Secret.
	VerifyOK *bool
}

// Send records the message.
func (t *RecordingTransport) Send(_ context.Context, m OutboundMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Sent = append(t.Sent, m)
	return nil
}

// ParseWebhook returns the canned inbound message.
func (t *RecordingTransport) ParseWebhook(_ []byte) (*InboundMessage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Next, nil
}

// VerifySignature implements fail-closed verification.
func (t *RecordingTransport) VerifySignature(_ []byte, signature string) bool {
	if t.VerifyOK != nil {
		return *t.VerifyOK
	}
	return signature == "valid:"+t.Secret
}
