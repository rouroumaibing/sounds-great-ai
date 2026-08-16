package gemini

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"sounds-great-ai/internal/adapter/unified"
)

// Adapter implements AgentExecutor for Gemini CLI.
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

func New(pm *unified.ProcessManager) *Adapter {
	return &Adapter{BinaryPath: "gemini", pm: pm}
}

func (a *Adapter) Capabilities() unified.AgentCapabilities {
	return unified.AgentCapabilities{
		SupportsMCP:      false,
		SupportsTools:    true,
		SupportsFileOps:  true,
		OutputFormat:     "stream-json",
		SupportsNativeL0: false, // no native L0 flag wired in this adapter yet
	}
}

func (a *Adapter) Health(ctx context.Context) error {
	_, err := exec.LookPath(a.BinaryPath)
	return err
}

func (a *Adapter) Execute(ctx context.Context, req unified.ExecuteRequest) (<-chan unified.StreamEvent, error) {
	if a.pm == nil {
		return nil, fmt.Errorf("process manager not configured")
	}
	args := []string{"--output-format", "stream-json"}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	// Persistent Identity P2 (CLI-native auto-compact):
	// gemini's CLI auto-compresses in-session, so no explicit flag is injected
	// (the CLI only injects the flag for codex). req.AutoCompactTokenLimit is
	// still consumed at the orchestration layer (history bounding).
	stdinInput := a.buildStdin(req)
	if a.registry != nil {
		// R1: same-invocation mid-stream fallback across the carrier's
		// transport chain. On a fatal mid-stream error the current tier's
		// output is abandoned and the prompt is re-run on the next transport.
		return unified.RunCarrierFallback(ctx, a.registry, a.carrierID, &unified.SpawnSpec{
			Command:    a.BinaryPath,
			Args:       args,
			WorkDir:    req.WorkDir,
			StdinInput: stdinInput,
			SessionID:  a.carrierID,
		}, func(h *unified.SpawnHandle) <-chan unified.StreamEvent {
			return a.streamEvents(h)
		})
	}
	handle, err := a.pm.Spawn(ctx, a.BinaryPath, args, stdinInput)
	if err != nil {
		return nil, err
	}
	return a.streamEvents(handle), nil
}

func (a *Adapter) buildStdin(req unified.ExecuteRequest) string {
	var sb strings.Builder
	if req.SystemPrompt != "" {
		sb.WriteString(req.SystemPrompt)
		sb.WriteString("\n\n")
	}
	for _, msg := range req.Messages {
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}
	return sb.String()
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
		for evt := range unified.ParseNDJSON(h.Stdout) {
			if unified.IsParseError(evt) {
				pe := evt.(unified.ParseError)
				sawError = true
				h.SetStreamError(pe.Line)
				ch <- unified.StreamEvent{Type: "error", Content: pe.Line}
				continue
			}
			obj := evt.(map[string]any)
			ch <- parseGeminiEvent(obj)
		}
		unified.EmitDiagnosticsIfNeeded(h, ch, sawError)
	}()
	return ch
}

func parseGeminiEvent(obj map[string]any) unified.StreamEvent {
	evtType, _ := obj["type"].(string)
	switch evtType {
	case "text":
		content, _ := obj["content"].(string)
		return unified.StreamEvent{Type: "text", Content: content}
	default:
		return unified.StreamEvent{Type: "text", Meta: obj}
	}
}
