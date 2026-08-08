package transport

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"sounds-great-ai/pkg/protocol"
)

// batchableEventTypes are high-frequency small events that benefit from micro-batching.
var batchableEventTypes = map[protocol.EventType]bool{
	protocol.EventThinking:       true,
	protocol.EventToolCall:       true,
	protocol.EventTerminalOutput: true,
}

const (
	batchFlushInterval = 16 * time.Millisecond
	batchMaxSize       = 32 // max events per batch before forced flush
)

type Streamer struct {
	conn    *websocket.Conn
	writeMu sync.Mutex

	// Per-session sequence number for gap detection
	seq uint64

	// Batch buffer for high-frequency small events
	batchMu     sync.Mutex
	batch       []*protocol.Event
	batchTimer  *time.Timer
	batchClosed bool
}

func NewStreamer(conn *websocket.Conn) *Streamer {
	return &Streamer{conn: conn}
}

func (s *Streamer) SendEvent(ctx context.Context, event *protocol.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Assign sequence number for gap detection
	s.writeMu.Lock()
	s.seq++
	event.Seq = s.seq
	s.writeMu.Unlock()

	// Non-batchable events: flush pending batch first, then send directly
	if !batchableEventTypes[event.Type] {
		s.flushBatch()
		return s.writeEvent(event)
	}

	// Batchable events: add to buffer
	s.batchMu.Lock()
	if s.batchClosed {
		s.batchMu.Unlock()
		return s.writeEvent(event)
	}

	s.batch = append(s.batch, event)

	// Start timer on first event, or flush if buffer is full
	if len(s.batch) == 1 {
		s.batchTimer = time.AfterFunc(batchFlushInterval, s.flushBatch)
	} else if len(s.batch) >= batchMaxSize {
		if s.batchTimer != nil {
			s.batchTimer.Stop()
		}
		s.batchMu.Unlock()
		s.flushBatch()
		return nil
	}
	s.batchMu.Unlock()
	return nil
}

// flushBatch sends all buffered events as a single write.
// If only one event is buffered, sends it as-is (no protocol change).
// If multiple events, sends as JSON array.
func (s *Streamer) flushBatch() {
	s.batchMu.Lock()
	if len(s.batch) == 0 {
		s.batchMu.Unlock()
		return
	}
	if s.batchTimer != nil {
		s.batchTimer.Stop()
		s.batchTimer = nil
	}
	pending := s.batch
	s.batch = nil
	s.batchMu.Unlock()

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if len(pending) == 1 {
		data, err := json.Marshal(pending[0])
		if err != nil {
			return
		}
		s.conn.WriteMessage(websocket.TextMessage, data)
		return
	}

	// Send as JSON array — client handles both single and array forms
	data, err := json.Marshal(pending)
	if err != nil {
		return
	}
	s.conn.WriteMessage(websocket.TextMessage, data)
}

// writeEvent sends a single event directly.
func (s *Streamer) writeEvent(event *protocol.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

// Send satisfies pack.EventSink. Delegates to SendEvent.
func (s *Streamer) Send(ctx context.Context, ev *protocol.Event) error {
	return s.SendEvent(ctx, ev)
}

func (s *Streamer) SendPing() error {
	s.flushBatch()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.PingMessage, nil)
}

// Close flushes any pending batch and marks the streamer as closed.
func (s *Streamer) Close() {
	s.batchMu.Lock()
	s.batchClosed = true
	s.batchMu.Unlock()
	s.flushBatch()
}
