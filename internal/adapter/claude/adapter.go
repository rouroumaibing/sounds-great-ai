package claude

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"sounds-great-ai/internal/adapter/unified"
)

// Adapter implements AgentExecutor for Claude Code CLI.
type Adapter struct {
	BinaryPath string
	pm         *unified.ProcessManager
}

// New creates a Claude Code adapter.
func New(pm *unified.ProcessManager) *Adapter {
	return &Adapter{BinaryPath: "claude", pm: pm}
}

func (a *Adapter) Capabilities() unified.AgentCapabilities {
	return unified.AgentCapabilities{
		SupportsMCP:     true,
		SupportsTools:   true,
		SupportsFileOps: true,
		OutputFormat:    "stream-json",
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
	args := a.buildArgs(req.Model, req.WorkDir, req.MCPConfig)
	stdinInput := a.buildStdin(req)
	reader, err := a.pm.Spawn(ctx, a.BinaryPath, args, stdinInput)
	if err != nil {
		return nil, err
	}
	return a.streamEvents(reader), nil
}

func (a *Adapter) buildArgs(model, workDir string, mcp *unified.MCPConfig) []string {
	args := []string{"--output-format", "stream-json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if workDir != "" {
		args = append(args, "--cwd", workDir)
	}
	return args
}

func (a *Adapter) buildStdin(req unified.ExecuteRequest) string {
	var sb strings.Builder
	for _, msg := range req.Messages {
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (a *Adapter) streamEvents(r io.Reader) <-chan unified.StreamEvent {
	ch := make(chan unified.StreamEvent, 64)
	go func() {
		defer close(ch)
		for evt := range unified.ParseNDJSON(r) {
			if unified.IsParseError(evt) {
				pe := evt.(unified.ParseError)
				ch <- unified.StreamEvent{Type: "error", Content: pe.Line}
				continue
			}
			obj := evt.(map[string]any)
			ch <- parseClaudeEvent(obj)
		}
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
