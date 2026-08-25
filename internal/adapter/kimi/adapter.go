// Package kimi implements the unified.AgentExecutor interface for the Kimi CLI
// (Moonshot). It is a faithful port of the KimiAgentService stream
// parsing, adapted to Sounds Great AI's stdin/stdout pipe + NDJSON contract.
//
// Invocation: `kimi -p <prompt> --output-format stream-json`.
// Cloud credentials (KIMI_API_KEY / KIMI_BASE_URL / KIMI_MODEL_NAME) are read
// from the environment, consistent with the other four CLI adapters in this
// repo (the platform spawns the CLI inheriting its parent environment — it
// does not inject secrets into the child process).
package kimi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sounds-great-ai/internal/adapter/unified"
)

// Adapter implements AgentExecutor for the Kimi CLI.
type Adapter struct {
	BinaryPath string
	pm         *unified.ProcessManager
	registry   *unified.Registry
	carrierID  string
}

// SetRegistry wires the carrier registry so Execute routes through the R1
// multi-tier fallback chain. When registry is nil (tests / legacy), Execute
// falls back to a direct one-shot pm.Spawn — behavior unchanged.
func (a *Adapter) SetRegistry(r *unified.Registry, id string) {
	a.registry = r
	a.carrierID = id
}

// New creates a Kimi CLI adapter.
func New(pm *unified.ProcessManager) *Adapter {
	return &Adapter{BinaryPath: "kimi", pm: pm}
}

func (a *Adapter) Capabilities() unified.AgentCapabilities {
	return unified.AgentCapabilities{
		SupportsMCP:      true,
		SupportsTools:    true,
		SupportsFileOps:  true,
		OutputFormat:     "stream-json",
		// F274: Kimi exposes a native L0 channel via --agent-file (compression-immune).
		SupportsNativeL0: true,
	}
}

// l0FileWriter writes L0 content to a file and returns the path. Injected for
// testability; production uses os.CreateTemp so no secret is passed on the CLI.
var l0FileWriter = func(content string) (string, error) {
	f, err := os.CreateTemp("", "kimi-l0-*.md")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func (a *Adapter) Health(ctx context.Context) error {
	_, err := exec.LookPath(a.BinaryPath)
	return err
}

func (a *Adapter) Execute(ctx context.Context, req unified.ExecuteRequest) (<-chan unified.StreamEvent, error) {
	if a.pm == nil {
		return nil, fmt.Errorf("process manager not configured")
	}
	args := a.buildArgs(req.Model, req.WorkDir, a.buildPrompt(req), req.SystemPromptL0)
	if a.registry != nil {
		// R1: same-invocation mid-stream fallback across the carrier's
		// transport chain. On a fatal mid-stream error the current tier's
		// output is abandoned and the prompt is re-run on the next transport.
		return unified.RunCarrierFallback(ctx, a.registry, a.carrierID, &unified.SpawnSpec{
			Command:    a.BinaryPath,
			Args:       args,
			WorkDir:    req.WorkDir,
			StdinInput: "",
			SessionID:  a.carrierID,
		}, func(h *unified.SpawnHandle) <-chan unified.StreamEvent {
			return a.streamEvents(h)
		})
	}
	handle, err := a.pm.Spawn(ctx, a.BinaryPath, args, "")
	if err != nil {
		return nil, err
	}
	return a.streamEvents(handle), nil
}

func (a *Adapter) buildArgs(model, workDir, prompt, l0 string) []string {
	args := []string{"-p", prompt}
	if model != "" {
		args = append(args, "--model", model)
	}
	if workDir != "" {
		args = append(args, "--cwd", workDir)
	}
	args = append(args, "--output-format", "stream-json")
	// F274: native L0 channel via --agent-file when an L0 prompt is present.
	if l0 != "" {
		if p, err := l0FileWriter(l0); err == nil {
			args = append(args, "--agent-file", p)
		}
	}
	return args
}

// buildPrompt wraps the system persona + user messages in the
// <system_instructions>/<user_request> envelope so the kimi CLI receives the
// breed identity as ordinary prompt text (modern kimi also supports a native
// --agent-file L0 channel, but the envelope works for both flavors).
func (a *Adapter) buildPrompt(req unified.ExecuteRequest) string {
	var userParts []string
	for _, msg := range req.Messages {
		if msg == nil {
			continue
		}
		userParts = append(userParts, msg.Content)
	}
	user := strings.Join(userParts, "\n")

	system := req.SystemPrompt
	if req.SystemPromptL0 != "" {
		if system != "" {
			system = system + "\n\n" + req.SystemPromptL0
		} else {
			system = req.SystemPromptL0
		}
	}

	if strings.TrimSpace(system) == "" {
		return user
	}
	return "<system_instructions>\n" + system + "\n</system_instructions>\n\n<user_request>\n" + user + "\n</user_request>"
}

func (a *Adapter) streamEvents(h *unified.SpawnHandle) <-chan unified.StreamEvent {
	ch := make(chan unified.StreamEvent, 64)
	h.SetOnStall(func(state unified.ProbeState, hard bool) {
		select {
		case ch <- unified.StreamEvent{
			Type:    "stall_warning",
			Meta:    map[string]any{"state": string(state), "hard": hard},
			Content: unified.LivenessMessage(state, hard),
		}:
		default:
		}
	})
	go func() {
		defer close(ch)
		sawError := false
		var doneMeta map[string]any
		for evt := range unified.ParseNDJSON(h.Stdout) {
			if unified.IsParseError(evt) {
				pe := evt.(unified.ParseError)
				sawError = true
				h.SetStreamError(pe.Line)
				ch <- unified.StreamEvent{Type: "error", Content: pe.Line}
				continue
			}
			obj := evt.(map[string]any)

			role, _ := obj["role"].(string)
			if role == "meta" {
				if sid := readSessionID(obj); sid != "" {
					doneMeta = attachSession(doneMeta, sid)
				}
				continue
			}

		// Kimi stream-json emits assistant messages; skip anything else
		// (e.g. residual system/control frames), consistent with the kimi CLI contract.
			if role != "" && role != "assistant" {
				continue
			}

			if text := extractTextContent(obj["content"]); text != "" {
				ch <- unified.StreamEvent{Type: "text", Content: text}
			}
			if thinking := extractThinkingContent(obj); thinking != "" {
				ch <- unified.StreamEvent{Type: "thinking", Content: thinking}
			}
			for _, tc := range extractToolCalls(obj) {
				ch <- unified.StreamEvent{
					Type:    "tool_call",
					Meta:    map[string]any{"tool": tc.name},
					Content: tc.arguments,
				}
			}
			if usage := parseUsage(obj["usage"]); usage != nil {
				doneMeta = mergeUsage(doneMeta, usage)
			} else if usage := parseUsage(obj["stats"]); usage != nil {
				doneMeta = mergeUsage(doneMeta, usage)
			}
			if sid := readSessionID(obj); sid != "" {
				doneMeta = attachSession(doneMeta, sid)
			}
		}
		if doneMeta != nil {
			ch <- unified.StreamEvent{Type: "done", Meta: doneMeta}
		} else {
			ch <- unified.StreamEvent{Type: "done"}
		}
		// G2: surface a sanitized, classified diagnosis if the CLI failed.
		unified.EmitDiagnosticsIfNeeded(h, ch, sawError)
	}()
	return ch
}

// ---- kimi-event-parser port ----

type toolCall struct {
	name      string
	arguments string
}

func extractTextContent(content any) string {
	if s, ok := content.(string); ok {
		if t := strings.TrimSpace(s); t != "" {
			return t
		}
		return ""
	}
	arr, ok := content.([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			parts = append(parts, s)
			continue
		}
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if s, ok := block["text"].(string); ok && s != "" {
			parts = append(parts, s)
		} else if s, ok := block["content"].(string); ok && s != "" {
			parts = append(parts, s)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func extractThinkingContent(obj map[string]any) string {
	candidates := []any{obj["thinking"], obj["reasoning"], obj["reasoning_content"], obj["thought"]}
	for _, c := range candidates {
		if t := extractTextContent(c); t != "" {
			return t
		}
	}
	if arr, ok := obj["content"].([]any); ok {
		var parts []string
		for _, item := range arr {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range []string{"think", "reasoning"} {
				if s, ok := block[key].(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
			if block["type"] == "thinking" {
				if s, ok := block["text"].(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
		}
		if t := strings.TrimSpace(strings.Join(parts, "\n")); t != "" {
			return t
		}
	}
	return ""
}

func extractToolCalls(obj map[string]any) []toolCall {
	raw, ok := obj["tool_calls"].([]any)
	if !ok {
		return nil
	}
	var calls []toolCall
	for _, item := range raw {
		call, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fn, ok := call["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		if name == "" {
			continue
		}
		calls = append(calls, toolCall{name: name, arguments: stringifyArgs(fn["arguments"])})
	}
	return calls
}

func stringifyArgs(args any) string {
	if s, ok := args.(string); ok {
		return s
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func parseUsage(c any) map[string]any {
	stats, ok := c.(map[string]any)
	if !ok {
		return nil
	}
	usage := map[string]any{}
	if v, ok := stats["total_tokens"].(float64); ok {
		usage["total_tokens"] = v
	}
	if v, ok := stats["input_tokens"].(float64); ok {
		usage["input_tokens"] = v
	}
	if v, ok := stats["output_tokens"].(float64); ok {
		usage["output_tokens"] = v
	}
	if v, ok := stats["cached_input_tokens"].(float64); ok {
		usage["cached_input_tokens"] = v
	}
	if v, ok := stats["context_window"].(float64); ok {
		usage["context_window"] = v
	}
	if v, ok := stats["context_used_tokens"].(float64); ok {
		usage["context_used_tokens"] = v
	}
	if len(usage) == 0 {
		return nil
	}
	return usage
}

func readSessionID(obj map[string]any) string {
	for _, key := range []string{"session_id", "sessionId"} {
		if s, ok := obj[key].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func attachSession(meta map[string]any, sid string) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["session_id"] = sid
	return meta
}

func mergeUsage(meta, usage map[string]any) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	for k, v := range usage {
		meta[k] = v
	}
	return meta
}
