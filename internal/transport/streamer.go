package transport

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"sounds-great-ai/pkg/protocol"
)

type Streamer struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func NewStreamer(conn *websocket.Conn) *Streamer {
	return &Streamer{conn: conn}
}

func (s *Streamer) SendEvent(ctx context.Context, event *protocol.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
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
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.PingMessage, nil)
}
