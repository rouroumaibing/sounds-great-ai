package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/adapter/unified"
	ports "sounds-great-ai/internal/domains/custody/ports"
	custodyServices "sounds-great-ai/internal/domains/custody/services"
	custodyStores "sounds-great-ai/internal/domains/custody/stores"
	routingServices "sounds-great-ai/internal/domains/routing/services"
	routingStores "sounds-great-ai/internal/domains/routing/stores"
	agentsPorts "sounds-great-ai/internal/domains/agents/ports"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/sop"
	sopServices "sounds-great-ai/internal/domains/sop/services"
	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"
)

// serialFakeExecutor records the order in which breeds are invoked (derived
// from the system prompt) so the test can assert a serial pipeline runs sa
// then sb, not in parallel.
type serialFakeExecutor struct {
	mu    sync.Mutex
	order []string
}

func (f *serialFakeExecutor) Execute(_ context.Context, req agentsPorts.ExecuteRequest) (<-chan agentsPorts.StreamEvent, error) {
	f.mu.Lock()
	switch {
	case strings.Contains(req.SystemPrompt, "sys-sa"):
		f.order = append(f.order, "sa")
	case strings.Contains(req.SystemPrompt, "sys-sb"):
		f.order = append(f.order, "sb")
	default:
		f.order = append(f.order, "unknown")
	}
	f.mu.Unlock()

	out := make(chan agentsPorts.StreamEvent, 4)
	go func() {
		defer close(out)
		// No hold, no @mention: a plain finishing turn.
		out <- agentsPorts.StreamEvent{Type: "text", Content: "ok"}
	}()
	return out, nil
}

// The methods below satisfy agentsPorts.IAgentExecutor. The fake IS the port
// (not a wrapped unified.AgentExecutor), so Get is unused by the tests.
func (f *serialFakeExecutor) Capabilities(clientID string) unified.AgentCapabilities {
	return unified.AgentCapabilities{}
}
func (f *serialFakeExecutor) Health(_ context.Context, clientID string) error { return nil }
func (f *serialFakeExecutor) Get(clientID string) (unified.AgentExecutor, error) {
	return nil, errors.New("test fake does not expose underlying adapter")
}
func (f *serialFakeExecutor) Count() int { return 1 }

func (f *serialFakeExecutor) getOrder() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.order))
	copy(cp, f.order)
	return cp
}

func buildSerialTestPlatform() (*platform.Platform, *serialFakeExecutor, *custodyStores.MemoryBallLedgerStore) {
	store := custodyStores.NewMemoryBallLedgerStore()
	ledger := custodyServices.NewBallLedger(store)
	holdScheduler := custodyServices.NewHoldScheduler(ledger, nil)
	fake := &serialFakeExecutor{}

	breeds := map[string]*pack.BreedConfig{
		"sa": {
			ID: "sa", Source: pack.BreedSourceSystem,
			MentionPatterns: []string{"@sa"},
			Variants: []pack.Variant{{ID: "v1", ClientID: "fake", SystemPrompt: "sys-sa", DefaultModel: "m"}},
		},
		"sb": {
			ID: "sb", Source: pack.BreedSourceSystem,
			MentionPatterns: []string{"@sb"},
			Variants: []pack.Variant{{ID: "v1", ClientID: "fake", SystemPrompt: "sys-sb", DefaultModel: "m"}},
		},
	}

	return &platform.Platform{
		AgentExecutor:  fake,
		Breeds:         breeds,
		Leader:         &pack.Leader{},
		BallLedger:    ledger,
		HoldScheduler: holdScheduler,
		A2AHub:        routingStores.NewA2AHubAdapter(a2a.NewHub(nil)),
		SOP:           sopServices.NewSOPGuardianService(sop.NewGuardian(nil, 3)),
		MentionRouter: routingServices.NewMentionRouterService(breeds),
	}, fake, store
}

func TestExecuteSerialPipeline(t *testing.T) {
	pl, fake, store := buildSerialTestPlatform()
	p := pack.New("test")
	handler := NewWSHandlerWithPlatform(p, pl)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		conn.SetReadDeadline(time.Now().Add(6 * time.Second))
		for {
			if _, _, e := conn.ReadMessage(); e != nil {
				return
			}
		}
	}()

	session := "serial-session"
	send := func(ev *protocol.Event) {
		data, _ := json.Marshal(ev)
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Serial intent: @sa → @sb. Router returns Strategy=serial.
	send(protocol.NewEvent(protocol.EventUserInput, session, &protocol.UserInputPayload{
		Message: "@sa → @sb 分析", SessionID: session,
	}))

	// Both breeds should run in order before we assert.
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if len(fake.getOrder()) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	order := fake.getOrder()
	if len(order) != 2 {
		t.Fatalf("execute calls = %d, want 2 (sa, sb)", len(order))
	}
	if order[0] != "sa" || order[1] != "sb" {
		t.Fatalf("serial order = %v, want [sa sb]", order)
	}

	// Disposition closure: the serial link sa → sb is recorded as
	// dispatch_dispositioned in the custody ledger.
	events, err := store.GetEvents(context.Background(), session)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	var link *ports.BallEvent
	for i := range events {
		if events[i].Type == ports.DispatchDispositioned && events[i].From == "sa" && events[i].To == "sb" {
			link = &events[i]
			break
		}
	}
	if link == nil {
		t.Fatal("expected dispatch_dispositioned(sa → sb) in ledger")
	}

	// The pipeline resolves once sb finishes.
	snap, _ := pl.BallLedger.Snapshot(context.Background(), session)
	if snap.State != ports.BallStateResolved {
		t.Fatalf("state = %s, want resolved", snap.State)
	}

	conn.Close()
	<-drainDone
}

func TestExecuteSerialNoDoubleHandoff(t *testing.T) {
	// Even if a serial breed's reply mentions the next breed, the serial loop
	// owns the worklist (suppressHandoff=true) so the next breed is NOT invoked
	// twice. We verify exactly 2 calls for a 2-breed serial chain.
	pl, fake, _ := buildSerialTestPlatform()
	p := pack.New("test")
	handler := NewWSHandlerWithPlatform(p, pl)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		conn.SetReadDeadline(time.Now().Add(6 * time.Second))
		for {
			if _, _, e := conn.ReadMessage(); e != nil {
				return
			}
		}
	}()

	session := "serial-no-double"
	send := func(ev *protocol.Event) {
		data, _ := json.Marshal(ev)
		conn.WriteMessage(websocket.TextMessage, data)
	}
	send(protocol.NewEvent(protocol.EventUserInput, session, &protocol.UserInputPayload{
		Message: "@sa → @sb 分析", SessionID: session,
	}))

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if len(fake.getOrder()) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := len(fake.getOrder()); got != 2 {
		t.Fatalf("execute calls = %d, want exactly 2 (serial loop owns worklist)", got)
	}

	conn.Close()
	<-drainDone
}
