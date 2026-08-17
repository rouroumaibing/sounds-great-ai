// Package a2aadapter implements the unified.AgentExecutor interface for the
// Google A2A Protocol client (§4.7 of the irreversible decisions).
//
// It is the controlled A2A *client* the platform is now allowed to run: it
// invokes an EXTERNAL already-deployed agent (another SG instance, an
// independent A2A agent, …) via `tasks/send` JSON-RPC over HTTPS. It is the
// sibling carrier of the CLI adapters — same Execute contract, different
// transport. It does NOT expose an inbound A2A server and does NOT do platform
// reasoning; the ball-custody ledger (§4.5) remains the orchestration truth
// source, and a handoff to an external A2A agent still passes through the
// ledger's guard.
//
// Protocol types are reused from pkg/a2a (package a2aprotocol below). The
// external endpoint + optional bearer token are resolved per client_id from
// (1) variant.a2a_url in the breed catalog, (2) env SG_A2A_URL_<UPPER(CLIENTID)>,
// then (3) a global SG_A2A_URL fallback.
package a2aadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	a2aprotocol "sounds-great-ai/pkg/a2a"
	"sounds-great-ai/internal/adapter/unified"
)

// Adapter implements AgentExecutor for a remote A2A Protocol agent.
type Adapter struct {
	pm       *unified.ProcessManager
	urls     map[string]string // clientID -> external A2A endpoint
	apiKeys  map[string]string // clientID -> optional bearer token
	timeout  time.Duration
	clientID string // the client_id this adapter answers to (e.g. "a2a")
}

// New creates an A2A protocol client adapter. clientID is the variant
// client_id that routes to this adapter (conventionally "a2a", or a more
// specific "a2a-<name>" to address distinct external agents).
func New(pm *unified.ProcessManager, clientID string) *Adapter {
	if clientID == "" {
		clientID = "a2a"
	}
	return &Adapter{
		pm:        pm,
		urls:      map[string]string{},
		apiKeys:   map[string]string{},
		timeout:   120 * time.Second,
		clientID:  clientID,
	}
}

// SetEndpoint registers an external endpoint for a given client_id. Called by
// platform bootstrap from variant.a2a_url (see platform.go).
func (a *Adapter) SetEndpoint(clientID, url, apiKey string) {
	if url == "" {
		return
	}
	a.urls[clientID] = url
	if apiKey != "" {
		a.apiKeys[clientID] = apiKey
	}
}

// Capabilities reports what the A2A transport supports.
func (a *Adapter) Capabilities() unified.AgentCapabilities {
	return unified.AgentCapabilities{
		SupportsMCP:      false,
		SupportsTools:    false,
		SupportsFileOps:  false,
		OutputFormat:     "a2a-json",
		SupportsNativeL0: false,
	}
}

// Health verifies an endpoint is configured for the given client_id.
func (a *Adapter) Health(ctx context.Context) error {
	if a.lookupURL(a.clientID) == "" {
		return fmt.Errorf("a2a adapter: no endpoint configured for client_id %q (set variant.a2a_url or SG_A2A_URL)", a.clientID)
	}
	return nil
}

// lookupURL resolves the external endpoint for a client_id with the precedence
// documented on the package: variant.a2a_url (registered) > env
// SG_A2A_URL_<UPPER(CLIENTID)> > global SG_A2A_URL.
func (a *Adapter) lookupURL(clientID string) string {
	if u, ok := a.urls[clientID]; ok && u != "" {
		return u
	}
	if u := os.Getenv("SG_A2A_URL_" + strings.ToUpper(clientID)); u != "" {
		return u
	}
	return os.Getenv("SG_A2A_URL")
}

func (a *Adapter) lookupAPIKey(clientID string) string {
	if k, ok := a.apiKeys[clientID]; ok && k != "" {
		return k
	}
	return os.Getenv("SG_A2A_API_KEY_" + strings.ToUpper(clientID))
}

// Execute sends the request to the external A2A agent via tasks/send and
// streams the resulting task artifacts back as unified.StreamEvents.
func (a *Adapter) Execute(ctx context.Context, req unified.ExecuteRequest) (<-chan unified.StreamEvent, error) {
	url := a.lookupURL(req.ClientID)
	if url == "" {
		return nil, fmt.Errorf("a2a adapter: no endpoint configured for client_id %q", req.ClientID)
	}

	prompt := buildPrompt(req)
	rpcReq := a2aprotocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      req.ClientID + "-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Method:  a2aprotocol.MethodTasksSend,
		Params: map[string]any{
			"id": req.ClientID,
			"message": map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": prompt},
				},
			},
		},
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("a2a adapter: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("a2a adapter: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if k := a.lookupAPIKey(req.ClientID); k != "" {
		httpReq.Header.Set("Authorization", "Bearer "+k)
	}

	client := &http.Client{Timeout: a.timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("a2a adapter: call %s: %w", url, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("a2a adapter: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("a2a adapter: upstream %s returned %d: %s", url, resp.StatusCode, string(raw))
	}

	var rpcResp a2aprotocol.JSONRPCResponse
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		return nil, fmt.Errorf("a2a adapter: decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("a2a adapter: upstream error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return a.streamTask(rpcResp.Result), nil
}

// streamTask maps a completed A2A Task into a stream of unified events.
func (a *Adapter) streamTask(task *a2aprotocol.Task) <-chan unified.StreamEvent {
	ch := make(chan unified.StreamEvent, 64)
	go func() {
		defer close(ch)
		if task == nil {
			ch <- unified.StreamEvent{Type: "done"}
			return
		}
		if task.Status == a2aprotocol.TaskStatusFailed || task.Status == a2aprotocol.TaskStatusCanceled {
			ch <- unified.StreamEvent{Type: "error", Content: "a2a agent task " + string(task.Status)}
			ch <- unified.StreamEvent{Type: "done", Meta: map[string]any{"a2a_status": string(task.Status)}}
			return
		}
		// Emit text from artifacts, then from history messages.
		for _, art := range task.Artifacts {
			for _, p := range art.Parts {
				if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
					ch <- unified.StreamEvent{Type: "text", Content: p.Text}
				}
			}
		}
		for _, m := range task.History {
			if t := m.ExtractText(); strings.TrimSpace(t) != "" {
				ch <- unified.StreamEvent{Type: "text", Content: t}
			}
		}
		ch <- unified.StreamEvent{Type: "done", Meta: map[string]any{"a2a_status": string(task.Status), "a2a_task_id": task.ID}}
	}()
	return ch
}

// buildPrompt flattens the Eino conversation + system persona into a single
// prompt string for the external agent. The external A2A agent has its own
// identity; we pass SG's breed persona as ordinary text context.
func buildPrompt(req unified.ExecuteRequest) string {
	var parts []string
	if sys := strings.TrimSpace(req.SystemPrompt); sys != "" {
		parts = append(parts, "<system_instructions>\n"+sys+"\n</system_instructions>")
	}
	if l0 := strings.TrimSpace(req.SystemPromptL0); l0 != "" {
		parts = append(parts, "<identity>\n"+l0+"\n</identity>")
	}
	var userParts []string
	for _, msg := range req.Messages {
		if msg == nil {
			continue
		}
		if t := strings.TrimSpace(msg.Content); t != "" {
			userParts = append(userParts, t)
		}
	}
	if len(userParts) > 0 {
		parts = append(parts, "<user_request>\n"+strings.Join(userParts, "\n")+"\n</user_request>")
	}
	out := strings.Join(parts, "\n\n")
	if strings.TrimSpace(out) == "" {
		return "(empty request)"
	}
	return out
}
