package claude

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sounds-great-ai/internal/adapter/unified"
)

// Adapter implements AgentExecutor for Claude Code CLI.
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

// New creates a Claude Code adapter.
func New(pm *unified.ProcessManager) *Adapter {
	return &Adapter{BinaryPath: "claude", pm: pm}
}

func (a *Adapter) Capabilities() unified.AgentCapabilities {
	return unified.AgentCapabilities{
		SupportsMCP:      true,
		SupportsTools:    true,
		SupportsFileOps:  true,
		OutputFormat:     "stream-json",
		SupportsNativeL0: true, // --append-system-prompt
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
	args, mcpPath := a.buildArgs(req.Model, req.WorkDir, req.MCPConfig, req.SystemPromptL0)
	stdinInput := a.buildStdin(req)
	// Persistent Identity P2 (CLI-native auto-compact):
	// claude's CLI auto-compresses in-session, so no explicit flag is injected
	// (the CLI behaves the same — it only injects the flag for codex). The value
	// is available on req.AutoCompactTokenLimit and is still consumed at the
	// orchestration layer (history bounding in execution.go).
	if a.registry != nil {
		// R1: same-invocation mid-stream fallback across the carrier's
		// transport chain (e.g. bg_daemon -> print_sdk). On a fatal mid-stream
		// error the current tier's output is abandoned and the prompt is
		// re-run on the next transport.
		return unified.RunCarrierFallback(ctx, a.registry, a.carrierID, &unified.SpawnSpec{
			Command:    a.BinaryPath,
			Args:       args,
			WorkDir:    req.WorkDir,
			StdinInput: stdinInput,
			SessionID:  a.carrierID,
		}, func(h *unified.SpawnHandle) <-chan unified.StreamEvent {
			if mcpPath != "" {
				// G5: ephemeral MCP config lives outside the repo; remove it
				// once the process has exited.
				h.OnExit(func() { _ = os.Remove(mcpPath) })
			}
			return a.streamEvents(h)
		})
	}
	handle, err := a.pm.Spawn(ctx, a.BinaryPath, args, stdinInput)
	if err != nil {
		return nil, err
	}
	if mcpPath != "" {
		handle.OnExit(func() { _ = os.Remove(mcpPath) })
	}
	return a.streamEvents(handle), nil
}

func (a *Adapter) buildArgs(model, workDir string, mcp *unified.MCPConfig, systemPromptL0 string) ([]string, string) {
	args := []string{"--output-format", "stream-json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if workDir != "" {
		args = append(args, "--cwd", workDir)
	}
	if systemPromptL0 != "" {
		args = append(args, "--append-system-prompt", systemPromptL0)
	}
	var mcpPath string
	if mcp != nil && len(mcp.Servers) > 0 && workDir != "" {
		if configPath, err := unified.WriteMCPConfigFile(mcp, workDir); err == nil && configPath != "" {
			mcpPath = configPath
			args = append(args, "--mcp-config", configPath)
		}
	}
	return args, mcpPath
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
	// R8: surface liveness-probe state changes to the client. Non-blocking so a
	// stray late probe callback can never panic on a closed channel.
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
			ch <- parseClaudeEvent(obj)
		}
		// G2: surface a sanitized, classified diagnosis if the CLI failed.
		unified.EmitDiagnosticsIfNeeded(h, ch, sawError)
	}()
	return ch
}

func parseClaudeEvent(obj map[string]any) unified.StreamEvent {
	evtType, _ := obj["type"].(string)
	switch evtType {
	case "assistant_response":
		content, _ := obj["content"].(string)
		return unified.StreamEvent{Type: "text", Content: content}
	case "tool_use":
		return unified.StreamEvent{Type: "tool_call", Meta: obj}
	case "tool_result":
		return unified.StreamEvent{Type: "tool_result", Meta: obj}
	case "result":
		return unified.StreamEvent{Type: "done", Meta: obj}
	default:
		return unified.StreamEvent{Type: "text", Meta: obj}
	}
}
