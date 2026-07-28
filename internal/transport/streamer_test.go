package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"sounds-great-ai/pkg/protocol"
)

func TestStreamerSendEvent(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	received := make(chan *protocol.Event, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
		}
		defer conn.Close()

		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}

		var ev protocol.Event
		if err := json.Unmarshal(data, &ev); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		received <- &ev
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	streamer := NewStreamer(conn)
	ev := protocol.NewEvent(protocol.EventThinking, "session-1", &protocol.ThinkingPayload{
		Step:    1,
		Content: "test thinking",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := streamer.SendEvent(ctx, ev); err != nil {
		t.Fatalf("SendEvent failed: %v", err)
	}

	select {
	case got := <-received:
		if got.Type != protocol.EventThinking {
			t.Errorf("type mismatch: got %s, want %s", got.Type, protocol.EventThinking)
		}
		if got.SessionID != "session-1" {
			t.Errorf("session_id mismatch: got %s, want session-1", got.SessionID)
		}
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for event")
	}
}
