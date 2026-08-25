// Package connectors defines the connector / channel execution layer
// (roadmap P1-A, README#5). It provides a generic Connector that normalizes
// outbound send and inbound webhook receive across IM channels, with two
// safety properties required by the roadmap:
//   - webhook signature verification is fail-closed (bad signature => reject);
//   - webhook processing is idempotent (a repeated EventID is dropped).
//
// Concrete channel transports live in subpackages (slack, telegram, weixin,
// feishu, media) and implement the Transport interface.
package connectors

import (
	"context"
	"errors"
	"sync"
)

// OutboundMessage is a normalized outgoing message.
type OutboundMessage struct {
	Channel   string
	To        string
	Text      string
	ThreadID  string
}

// InboundMessage is a normalized incoming message (after webhook parse).
type InboundMessage struct {
	Channel  string
	EventID  string // stable id used for idempotency
	From     string
	Text     string
	ThreadID  string
}

// Transport is the channel-specific send/receive backend. Implementations wrap
// real SDKs; tests inject fakes.
type Transport interface {
	// Send delivers an outbound message.
	Send(ctx context.Context, m OutboundMessage) error
	// ParseWebhook extracts a normalized InboundMessage from a raw webhook
	// payload. It returns (nil, nil) for payloads that are not message events
	// (e.g. challenge/verification pings).
	ParseWebhook(payload []byte) (*InboundMessage, error)
	// VerifySignature validates the webhook signature. Fail-closed: a bad or
	// missing signature returns false.
	VerifySignature(payload []byte, signature string) bool
}

// ErrBadSignature is returned when a webhook signature fails verification.
var ErrBadSignature = errors.New("connectors: bad webhook signature (fail-closed)")

// ErrDuplicateEvent is returned when an already-processed EventID arrives.
var ErrDuplicateEvent = errors.New("connectors: duplicate webhook event (idempotent drop)")

// Connector is a channel-normalized execution unit.
type Connector struct {
	Name      string
	transport Transport

	mu   sync.Mutex
	seen map[string]bool // EventID -> processed
}

// New creates a connector for a channel over a transport.
func New(name string, t Transport) *Connector {
	return &Connector{Name: name, transport: t, seen: make(map[string]bool)}
}

// Send delivers an outbound message.
func (c *Connector) Send(ctx context.Context, m OutboundMessage) error {
	if c.transport == nil {
		return errors.New("connectors: no transport")
	}
	return c.transport.Send(ctx, m)
}

// ReceiveWebhook verifies the signature, drops duplicates, and returns the
// parsed message. On a bad signature it returns ErrBadSignature (fail-closed);
// on a repeat EventID it returns ErrDuplicateEvent.
func (c *Connector) ReceiveWebhook(ctx context.Context, payload []byte, signature string) (*InboundMessage, error) {
	if c.transport == nil {
		return nil, errors.New("connectors: no transport")
	}
	if !c.transport.VerifySignature(payload, signature) {
		return nil, ErrBadSignature
	}
	msg, err := c.transport.ParseWebhook(payload)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, nil // non-message event, already ignored
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen[msg.EventID] {
		return nil, ErrDuplicateEvent
	}
	c.seen[msg.EventID] = true
	return msg, nil
}

// ConnectorRegistry holds named connectors. Goroutine-safe.
type ConnectorRegistry struct {
	mu        sync.RWMutex
	connectors map[string]*Connector
}

// NewConnectorRegistry creates an empty registry.
func NewConnectorRegistry() *ConnectorRegistry {
	return &ConnectorRegistry{connectors: make(map[string]*Connector)}
}

// Register adds a connector by name (replaces on collision).
func (r *ConnectorRegistry) Register(c *Connector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectors[c.Name] = c
}

// Get returns a connector by name (nil if absent).
func (r *ConnectorRegistry) Get(name string) *Connector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.connectors[name]
}

// Names returns registered connector names.
func (r *ConnectorRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.connectors))
	for n := range r.connectors {
		out = append(out, n)
	}
	return out
}
