package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
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

// holdMarkerFence is the raw control signal the dog emits at end of turn.
const holdMarkerFence = "```hold_ball\n{\"kind\":\"manual\"}\n```\n请稍候，等待外部条件。"

// fakeHoldExecutor simulates a CLI agent. Behaviour depends on the message:
//   - first turn (no wake notice): declares a hold
//   - resume turn (contains wake notice): hands off to @xigou
//   - xigou turn (contains @xigou): finishes
type fakeHoldExecutor struct {
	calls int32
}

func joinMessages(msgs []*schema.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func (f *fakeHoldExecutor) Execute(_ context.Context, req agentsPorts.ExecuteRequest) (<-chan agentsPorts.StreamEvent, error) {
	atomic.AddInt32(&f.calls, 1)
	content := joinMessages(req.Messages)
	out := make(chan agentsPorts.StreamEvent, 8)
	go func() {
		defer close(out)
		var text string
		switch {
		case strings.Contains(content, "唤醒条件已满足"):
			text = "@xigou 请继续后续步骤"
		case strings.Contains(content, "@xigou"):
			text = "xigou 已完成任务"
		default:
			text = holdMarkerFence
		}
		out <- agentsPorts.StreamEvent{Type: "text", Content: text}
	}()
	return out, nil
}

// The methods below satisfy agentsPorts.IAgentExecutor. The fake IS the port
// (not a wrapped unified.AgentExecutor), so Get is unused by the tests.
func (f *fakeHoldExecutor) Capabilities(clientID string) unified.AgentCapabilities {
	return unified.AgentCapabilities{}
}
func (f *fakeHoldExecutor) Health(_ context.Context, clientID string) error { return nil }
func (f *fakeHoldExecutor) Get(clientID string) (unified.AgentExecutor, error) {
	return nil, errors.New("test fake does not expose underlying adapter")
}
func (f *fakeHoldExecutor) Count() int { return 1 }

func buildHoldTestPlatform() (*platform.Platform, *fakeHoldExecutor) {
	ledger := custodyServices.NewBallLedger(custodyStores.NewMemoryBallLedgerStore())
	holdScheduler := custodyServices.NewHoldScheduler(ledger, nil)
	fake := &fakeHoldExecutor{}

	breeds := map[string]*pack.BreedConfig{
		"h1": {
			ID:             "h1",
			Source:         pack.BreedSourceSystem,
			MentionPatterns: []string{"@h1"},
			Variants: []pack.Variant{{
				ID: "v1", ClientID: "fake", SystemPrompt: "you are h1", DefaultModel: "m",
			}},
		},
		"xigou": {
			ID:             "xigou",
			Source:         pack.BreedSourceSystem,
			MentionPatterns: []string{"@xigou"},
			Variants: []pack.Variant{{
				ID: "v1", ClientID: "fake", SystemPrompt: "you are xigou", DefaultModel: "m",
			}},
		},
	}

	return &platform.Platform{
		AgentExecutor:  fake,
		Breeds:         breeds,
		Leader:         &pack.Leader{},
		BallLedger:     ledger,
		HoldScheduler:  holdScheduler,
		A2AHub:         routingStores.NewA2AHubAdapter(a2a.NewHub(nil)),
		SOP:            sopServices.NewSOPGuardianService(sop.NewGuardian(nil, 3)),
		MentionRouter:  routingServices.NewMentionRouterService(breeds),
	}, fake
}

func TestExtractHoldCondition(t *testing.T) {
	cond, cleaned, held := extractHoldCondition("请稍候 ```hold_ball\n{\"kind\":\"manual\"}\n``` 完成")
	if !held {
		t.Fatal("expected held=true")
	}
	if cond.Kind != ports.WakeManual {
		t.Fatalf("kind = %s, want manual", cond.Kind)
	}
	if !strings.Contains(cleaned, "请稍候") || strings.Contains(cleaned, "hold_ball") {
		t.Fatalf("cleaned = %q", cleaned)
	}

	// No marker.
	if _, _, held := extractHoldCondition("just text"); held {
		t.Fatal("expected held=false for plain text")
	}

	// Webhook kind.
	cond, _, held = extractHoldCondition("```hold_ball\n{\"kind\":\"webhook\",\"token\":\"abc\"}\n```")
	if !held || cond.Kind != ports.WakeWebhook || cond.Token != "abc" {
		t.Fatalf("webhook parse wrong: %+v held=%v", cond, held)
	}

	// Unterminated fence -> no hold.
	if _, _, held := extractHoldCondition("```hold_ball\n{\"kind\":\"manual\"}"); held {
		t.Fatal("expected held=false for unterminated fence")
	}
}

func TestHoldMarkerFilterStripsFence(t *testing.T) {
	var out strings.Builder
	f := newHoldMarkerFilter(func(s string) { out.WriteString(s) })
	// Fence split across two chunks.
	f.push("result ```hold_ball\n{\"kind")
	f.push("\":\"manual\"}\n``` done")
	f.flush()
	got := out.String()
	if strings.Contains(got, "hold_ball") {
		t.Fatalf("fence leaked into output: %q", got)
	}
	if !strings.Contains(got, "result") || !strings.Contains(got, "done") {
		t.Fatalf("content lost: %q", got)
	}
}

func TestExecuteWithPlatformHoldAndResume(t *testing.T) {
	pl, fake := buildHoldTestPlatform()
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

	// Drain events in the background so the streamer never blocks.
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

	session := "hold-session"
	send := func(ev *protocol.Event) {
		data, _ := json.Marshal(ev)
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Turn 1: h1 declares a hold.
	send(protocol.NewEvent(protocol.EventUserInput, session, &protocol.UserInputPayload{
		Message: "@h1 do task", SessionID: session,
	}))

	// Wait for the hold to be registered.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rec, ok := pl.GetHold(context.Background(), session); ok && rec.Holder == "h1" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	rec, ok := pl.GetHold(context.Background(), session)
	if !ok {
		t.Fatal("expected an active hold after turn 1")
	}
	if rec.Holder != "h1" {
		t.Fatalf("holder = %s, want h1", rec.Holder)
	}
	// Ledger projects to parked.
	snap, _ := pl.BallLedger.Snapshot(context.Background(), session)
	if snap.State != ports.BallStateParked {
		t.Fatalf("state = %s, want parked", snap.State)
	}
	if atomic.LoadInt32(&fake.calls) != 1 {
		t.Fatalf("execute calls = %d, want 1", atomic.LoadInt32(&fake.calls))
	}

	// Turn 2: human wakes the hold (manual).
	send(protocol.NewEvent(protocol.EventWakeHold, session, &protocol.WakeHoldPayload{
		SessionID: session, Kind: "manual",
	}))

	// Wait until the resume chain runs: h1 (resume) -> @xigou -> xigou.
	deadline = time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&fake.calls) >= 3 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&fake.calls); got < 3 {
		t.Fatalf("execute calls = %d, want >= 3 (h1, h1-resume, xigou)", got)
	}
	// Hold released.
	if _, ok := pl.GetHold(context.Background(), session); ok {
		t.Fatal("hold should be released after wake")
	}
	snap, _ = pl.BallLedger.Snapshot(context.Background(), session)
	// The chain resolved: h1 held -> woken -> handed to @xigou -> xigou done.
	if snap.State != ports.BallStateResolved {
		t.Fatalf("state = %s, want resolved after resume+done", snap.State)
	}

	conn.Close()
	<-drainDone
}
