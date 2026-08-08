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
	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/sop"
	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"
)

type barkTestCap struct {
	name    string
	version string
}

func (c *barkTestCap) Name() string                                  { return c.name }
func (c *barkTestCap) Version() string                               { return c.version }
func (c *barkTestCap) Init(ctx context.Context) error                { return nil }
func (c *barkTestCap) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	return &pack.TaskOutput{Approved: true, Reason: "safe"}, nil
}
func (c *barkTestCap) Health() error { return nil }
func (c *barkTestCap) Close() error  { return nil }

func TestWSHandlerConnectionAndRoundTrip(t *testing.T) {
	handler := NewWSHandler(pack.New("test"))
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	userEvent := protocol.NewEvent(protocol.EventUserInput, "test-session", &protocol.UserInputPayload{
		Message:   "hello agent",
		SessionID: "test-session",
	})
	data, _ := json.Marshal(userEvent)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}

	var resp protocol.Event
	if err := json.Unmarshal(respData, &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.Type != protocol.EventBarkStart {
		t.Errorf("expected BARK_START response, got %s", resp.Type)
	}
}

func TestWSHandlerInvalidMessage(t *testing.T) {
	handler := NewWSHandler(pack.New("test"))
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("not json")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected error event, got disconnect: %v", err)
	}

	var resp protocol.Event
	if err := json.Unmarshal(respData, &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
}

func TestWSHandlerBarkStartAndResult(t *testing.T) {
	// Create a pack with a registered breed that has a real capability
	p := pack.New("test")

	// Register a test capability
	err := p.RegisterCapability(&barkTestCap{name: "command_check", version: "v1"})
	if err != nil {
		t.Fatalf("RegisterCapability: %v", err)
	}

	// Register breed
	p.Register(&pack.BreedConfig{
		ID:     "zhonghuatianyuanquan",
		Source: pack.BreedSourceSystem,
	})

	handler := NewWSHandler(p)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send USER_INPUT with @mention
	userEvent := protocol.NewEvent(protocol.EventUserInput, "bark-session", &protocol.UserInputPayload{
		Message:   "@zhonghuatianyuanquan check this",
		SessionID: "bark-session",
	})
	data, _ := json.Marshal(userEvent)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Read BARK_START event
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read BARK_START failed: %v", err)
	}
	var startEv protocol.Event
	json.Unmarshal(respData, &startEv)
	if startEv.Type != protocol.EventBarkStart {
		t.Errorf("expected BARK_START, got %s", startEv.Type)
	}
	var startPayload protocol.BarkStartPayload
	json.Unmarshal(startEv.Payload, &startPayload)
	if startPayload.Breed != "zhonghuatianyuanquan" {
		t.Errorf("breed = %q, want %q", startPayload.Breed, "zhonghuatianyuanquan")
	}

	// Read BARK_RESULT event
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, respData2, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read BARK_RESULT failed: %v", err)
	}
	var resultEv protocol.Event
	json.Unmarshal(respData2, &resultEv)
	if resultEv.Type != protocol.EventBarkResult {
		t.Errorf("expected BARK_RESULT, got %s", resultEv.Type)
	}
}

func TestWSHandlerBarkErrorOnMissingBreed(t *testing.T) {
	p := pack.New("test")
	// No breeds registered — default "bianmu" will fail
	handler := NewWSHandler(p)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	userEvent := protocol.NewEvent(protocol.EventUserInput, "err-session", &protocol.UserInputPayload{
		Message:   "hello with no breed",
		SessionID: "err-session",
	})
	data, _ := json.Marshal(userEvent)
	conn.WriteMessage(websocket.TextMessage, data)

	// Read BARK_START
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.ReadMessage()

	// Read BARK_ERROR
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read BARK_ERROR failed: %v", err)
	}
	var errEv protocol.Event
	json.Unmarshal(respData, &errEv)
	if errEv.Type != protocol.EventBarkError {
		t.Errorf("expected BARK_ERROR, got %s", errEv.Type)
	}
}

// newConcurrentTestPack builds a Pack with a registered breed backed by barkTestCap,
// mirroring the setup in TestWSHandlerBarkStartAndResult.
func newConcurrentTestPack(t *testing.T) *pack.Pack {
	t.Helper()
	p := pack.New("test")
	if err := p.RegisterCapability(&barkTestCap{name: "command_check", version: "v1"}); err != nil {
		t.Fatalf("RegisterCapability: %v", err)
	}
	p.Register(&pack.BreedConfig{
		ID:     "zhonghuatianyuanquan",
		Source: pack.BreedSourceSystem,
	})
	return p
}

// TestWSHandlerConcurrencyNoPanic sends 20 USER_INPUT messages rapidly against a
// breed with a registered capability. The handler's semaphore (maxConcurrentBark=8)
// limits concurrent Bark goroutines; the test verifies all 40 expected events
// (BARK_START + BARK_RESULT per message) are received without panic. Must pass
// under -race.
func TestWSHandlerConcurrencyNoPanic(t *testing.T) {
	handler := NewWSHandler(newConcurrentTestPack(t))
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	const msgCount = 20
	// Send all messages rapidly before reading any responses to maximize
	// concurrent in-flight Bark goroutines and stress the semaphore.
	for i := 0; i < msgCount; i++ {
		userEvent := protocol.NewEvent(protocol.EventUserInput, "concurrent-session", &protocol.UserInputPayload{
			Message:   "@zhonghuatianyuanquan check this",
			SessionID: "concurrent-session",
		})
		data, _ := json.Marshal(userEvent)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
	}

	// Each USER_INPUT yields BARK_START + BARK_RESULT = 2 events.
	const expectedEvents = msgCount * 2
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	for i := 0; i < expectedEvents; i++ {
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Fatalf("read event %d/%d failed: %v", i+1, expectedEvents, err)
		}
	}
}

// TestWSHandlerWriteSafetyNoPanic focuses on write safety: multiple goroutines
// (Bark workers + ping path) write events through the Streamer. Sending 10
// messages in rapid succession produces concurrent writes that must not race.
// Must pass under -race.
func TestWSHandlerWriteSafetyNoPanic(t *testing.T) {
	handler := NewWSHandler(newConcurrentTestPack(t))
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	const msgCount = 10
	for i := 0; i < msgCount; i++ {
		userEvent := protocol.NewEvent(protocol.EventUserInput, "writesafety-session", &protocol.UserInputPayload{
			Message:   "@zhonghuatianyuanquan check this",
			SessionID: "writesafety-session",
		})
		data, _ := json.Marshal(userEvent)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
	}

	const expectedEvents = msgCount * 2
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for i := 0; i < expectedEvents; i++ {
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Fatalf("read event %d/%d failed: %v", i+1, expectedEvents, err)
		}
	}
}

func TestDetectMentionInResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "no mention",
			input:    "just a regular response",
			expected: nil,
		},
		{
			name:     "single mention",
			input:    "need @xigou review code",
			expected: []string{"xigou"},
		},
		{
			name:     "multiple mentions",
			input:    "@bianmu decompose task, then @demu trace logs",
			expected: []string{"bianmu", "demu"},
		},
		{
			name:     "mention with surrounding text",
			input:    "I think @jinmao should retrieve the knowledge base for this",
			expected: []string{"jinmao"},
		},
		{
			name:     "duplicate mentions deduplicated",
			input:    "@xigou check this and @xigou also that",
			expected: []string{"xigou"},
		},
		{
			name:     "email address not matched",
			input:    "contact me at user@example.com",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMentionInResponse(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestHandleA2AHandoffDepthExceeded(t *testing.T) {
	guardian := sop.NewGuardian(nil, 1)
	thread := &a2a.Thread{
		ID:               "test-thread",
		ReviewRoundCount: 1,
		Participants:     []string{"bianmu"},
	}

	action := guardian.CheckA2ADepth(thread)
	if action != sop.EscalateToCVO {
		t.Fatalf("expected EscalateToCVO, got %d", action)
	}
}

func TestHandleA2AHandoffDepthOK(t *testing.T) {
	guardian := sop.NewGuardian(nil, 3)
	thread := &a2a.Thread{
		ID:               "test-thread",
		ReviewRoundCount: 0,
		Participants:     []string{"bianmu"},
	}

	action := guardian.CheckA2ADepth(thread)
	if action != sop.Continue {
		t.Fatalf("expected Continue, got %d", action)
	}
}

func TestHandleA2AHandoffSelectReviewer(t *testing.T) {
	reviewer := sop.SelectReviewer("bianmu", []string{"xigou", "demu"}, sop.ReviewPolicy{
		RequireDifferentBreed: true,
	})
	if reviewer != "xigou" {
		t.Fatalf("expected xigou, got %q", reviewer)
	}

	reviewer = sop.SelectReviewer("bianmu", []string{"bianmu"}, sop.ReviewPolicy{
		RequireDifferentBreed: true,
	})
	if reviewer != "" {
		t.Fatalf("expected empty, got %q", reviewer)
	}
}

func TestExecuteWithPlatformTriggersHandoffOnMention(t *testing.T) {
	response := "analysis done, need @xigou review code"
	mentions := detectMentionInResponse(response)

	if len(mentions) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(mentions))
	}
	if mentions[0] != "xigou" {
		t.Errorf("expected xigou, got %q", mentions[0])
	}
}

func TestExecuteWithPlatformNoHandoffWithoutMention(t *testing.T) {
	response := "task done, code fixed"
	mentions := detectMentionInResponse(response)

	if mentions != nil {
		t.Fatalf("expected nil, got %v", mentions)
	}
}
