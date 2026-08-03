package pack

import (
	"context"

	"sounds-great-ai/pkg/protocol"
)

// EventSink is implemented by transport layers (e.g. *transport.Streamer) to
// push events to clients. Capabilities like stream_response use it to emit
// results incrementally. A nil Sink means "no transport bound"; capabilities
// must degrade gracefully (skip sending, return streamed=false).
type EventSink interface {
	Send(ctx context.Context, ev *protocol.Event) error
}
