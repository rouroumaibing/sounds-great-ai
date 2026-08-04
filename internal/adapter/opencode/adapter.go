package opencode

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"sounds-great-ai/internal/adapter/unified"
)

// Adapter implements AgentExecutor for opencode CLI.
type Adapter struct {
	BinaryPath string
	pm         *unified.ProcessManager
}

func New(pm *unified.ProcessManager) *Adapter {
	return &Adapter{BinaryPath: "opencode", pm: pm}
}

func (a *Adapter) Capabilities() unified.AgentCapabilities {
	return unified.AgentCapabilities{
		SupportsMCP:     true,
		SupportsTools:   true,
		SupportsFileOps: true,
		OutputFormat:    "ndjson",
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
	args := []string{"--output", "ndjson"}
	stdinInput := a.buildStdin(req)
	reader, err := a.pm.Spawn(ctx, a.BinaryPath, args, stdinInput)
	if err != nil {
		return nil, err
	}
	return a.streamEvents(reader), nil
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
			ch <- parseOpencodeEvent(obj)
		}
	}()
	return ch
}

func parseOpencodeEvent(obj map[string]any) unified.StreamEvent {
	evtType, _ := obj["type"].(string)
	switch evtType {
	case "message":
		content, _ := obj["content"].(string)
		return unified.StreamEvent{Type: "text", Content: content}
	case "tool":
		return unified.StreamEvent{Type: "tool_call", Meta: obj}
	case "session":
		return unified.StreamEvent{Type: "done", Meta: obj}
	default:
		return unified.StreamEvent{Type: "text", Meta: obj}
	}
}
