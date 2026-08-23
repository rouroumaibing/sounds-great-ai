package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sounds-great-ai/internal/aspect"
	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"

	"github.com/gorilla/websocket"
)

// waitCtx gives RequestApproval a bounded lifetime tied to the test.
func waitCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// readUntil reads frames from the client socket until one of the wanted type
// arrives (skipping BARK_START/BARK_ERROR routing noise), or times out.
func readUntil(t *testing.T, conn *websocket.Conn, want protocol.EventType) protocol.Event {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read %s failed: %v", want, err)
		}
		var ev protocol.Event
		if err := json.Unmarshal(data, &ev); err != nil {
			t.Fatalf("unmarshal frame %s: %v", string(data), err)
		}
		if ev.Type == want {
			return ev
		}
	}
}

func dialWS(t *testing.T, handler *WSHandler) (*websocket.Conn, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWS))
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := (&websocket.Dialer{HandshakeTimeout: 5 * time.Second}).Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial failed: %v", err)
	}
	return conn, server.Close
}

func writeWS(t *testing.T, conn *websocket.Conn, ev *protocol.Event) {
	t.Helper()
	frame, _ := json.Marshal(ev)
	if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
		t.Fatalf("write %s failed: %v", ev.Type, err)
	}
}

// The HITL loop over the real transport path: USER_INPUT binds the server-side
// streamer → a governed path raises RequestApproval → the client receives
// HITL_APPROVAL → the operator answers via HITL_RESPONSE → the blocked caller
// wakes with the decision.
func TestWSHandlerHitlApprovalLoop(t *testing.T) {
	handler := NewWSHandler(pack.New("test"))
	conn, closeSrv := dialWS(t, handler)
	defer conn.Close()
	defer closeSrv()

	// Bind this connection's server-side streamer under "hitl-session" the
	// way production does (HandleWS does this on USER_INPUT).
	writeWS(t, conn, protocol.NewEvent(protocol.EventUserInput, "hitl-session", &protocol.UserInputPayload{
		Message:   "hello",
		SessionID: "hitl-session",
	}))
	// Drain the routing response (BARK_START or BARK_ERROR) — irrelevant here.
	readUntil(t, conn, protocol.EventBarkStart)

	decision := make(chan bool, 1)
	go func() {
		approved, err := handler.Approvals().RequestApproval(waitCtx(t), &aspect.ApprovalRequest{
			RequestID: "req-hitl-1",
			Action:    "git push --force origin main",
			Impact:    "force-push rewrites remote history",
			SessionID: "hitl-session",
		})
		if err != nil {
			t.Errorf("RequestApproval returned error: %v", err)
			decision <- false
			return
		}
		decision <- approved
	}()

	// Wait until the request is registered.
	deadline := time.Now().Add(2 * time.Second)
	for !handler.Approvals().IsPending("req-hitl-1") {
		if time.Now().After(deadline) {
			t.Fatal("approval never became pending")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The operator's client receives the HITL_APPROVAL card.
	ev := readUntil(t, conn, protocol.EventHITLApproval)
	if ev.SessionID != "hitl-session" {
		t.Fatalf("HITL_APPROVAL session = %s, want hitl-session", ev.SessionID)
	}
	var payload protocol.HITLApprovalPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if payload.RequestID != "req-hitl-1" || payload.Action != "git push --force origin main" {
		t.Fatalf("payload wrong: %+v", payload)
	}

	// Operator approves over the socket → the blocked caller wakes with true.
	writeWS(t, conn, protocol.NewEvent(protocol.EventHitlResponse, "hitl-session", &protocol.HitlResponsePayload{
		RequestID: "req-hitl-1",
		Approved:  true,
		Reason:    "planned rewrite",
	}))

	select {
	case got := <-decision:
		if !got {
			t.Fatal("RequestApproval woke with approved=false, want true")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RequestApproval never resolved")
	}

	// Answering an unknown/expired request surfaces a notice, not an error —
	// the stream stays usable.
	writeWS(t, conn, protocol.NewEvent(protocol.EventHitlResponse, "hitl-session", &protocol.HitlResponsePayload{
		RequestID: "req-gone",
		Approved:  false,
	}))
	if ev := readUntil(t, conn, protocol.EventSystemNotice); ev.SessionID != "" {
		// notices broadcast with empty session id; nothing to assert beyond type
		_ = ev
	}
}

// PendingEscalations backs GET /api/escalations: listed until answered.
func TestWSHandlerPendingEscalationsListing(t *testing.T) {
	handler := NewWSHandler(pack.New("test"))
	handler.emitCvoEscalation("t1", "bianmu", "xigou", "深度超限")

	list := handler.PendingEscalations()
	if len(list) != 1 {
		t.Fatalf("pending escalations = %d, want 1", len(list))
	}
	if list[0].SessionID != "t1" || list[0].Payload == nil || list[0].Payload.EscalationID == "" {
		t.Fatalf("listing wrong: %+v", list[0])
	}
	if len(list[0].Payload.Options) != 2 {
		t.Errorf("expected 2 preset options, got %d", len(list[0].Payload.Options))
	}

	// Answering removes it from the pending list.
	handler.handleEscalationResponse(nil, "t1", protocol.CvoEscalationResponsePayload{
		EscalationID: list[0].Payload.EscalationID,
		Decision:     "intervene",
	})
	if got := handler.PendingEscalations(); len(got) != 0 {
		t.Fatalf("escalation not removed after response: %+v", got)
	}
}
