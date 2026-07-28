package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sounds-great-ai/internal/a2a/client"
	"sounds-great-ai/internal/a2a/server"
	"sounds-great-ai/pkg/a2a"
)

// Orchestrator drives the multi-agent conversation
type Orchestrator struct {
	agents   map[string]*server.AgentServer
	clients  map[string]*client.AgentClient
	urls     map[string]string
	worklist *Worklist
	script   *Script
	traceID  string
}

// NewOrchestrator creates a new Orchestrator
func NewOrchestrator(
	agents map[string]*server.AgentServer,
	clients map[string]*client.AgentClient,
	urls map[string]string,
	script *Script,
) *Orchestrator {
	return &Orchestrator{
		agents:   agents,
		clients:  clients,
		urls:     urls,
		worklist: NewWorklist(),
		script:   script,
		traceID:  uuid.NewString(),
	}
}

// Run executes the conversation script
func (o *Orchestrator) Run(ctx context.Context) error {
	contextID := uuid.NewString()
	var history []a2a.Message
	parentSpanID := ""

	for i, turn := range o.script.Turns {
		// Generate new SpanID for this turn
		spanID := uuid.NewString()
		trace := &a2a.TraceInfo{
			TraceID:      o.traceID,
			SpanID:       spanID,
			ParentSpanID: parentSpanID,
		}
		parentSpanID = spanID

		// Build task
		task := a2a.Task{
			ID:        uuid.NewString(),
			ContextID: contextID,
			Status:    a2a.TaskStatusSubmitted,
			History:   history,
			Trace:     trace,
		}

		// First turn: inject prompt as user message
		if i == 0 && turn.Prompt != "" {
			task.History = []a2a.Message{
				{
					ID:         uuid.NewString(),
					Role:       "user",
					SenderName: "Orchestrator",
					Parts:      []a2a.Part{{Type: "text", Text: turn.Prompt}},
				},
			}
		}

		// Send to target agent
		c := o.clients[turn.ToAgent]
		if c == nil {
			return fmt.Errorf("no client for agent %s", turn.ToAgent)
		}

		fmt.Printf("[Turn %d] → %s\n", i+1, turn.ToAgent)
		result, err := c.SendTaskSync(ctx, task, 30*time.Second)
		if err != nil {
			return fmt.Errorf("turn %d: %w", i+1, err)
		}

		// Handle status
		switch result.Status {
		case a2a.TaskStatusCompleted:
			// Extract agent reply
			if len(result.History) > 0 {
				lastMsg := result.History[len(result.History)-1]
				history = result.History
				fmt.Printf("[Turn %d] %s [completed]: %s\n", i+1, turn.ToAgent, lastMsg.ExtractText())

				// Check for @mentions
				mentions := parseMentions(lastMsg.ExtractText())
				for _, mentioned := range mentions {
					if _, exists := o.clients[mentioned]; exists {
						visitedChain := append([]string{}, turn.ToAgent)
						err := o.worklist.Add(WorklistEntry{
							TaskID:       uuid.NewString(),
							ContextID:    contextID,
							FromAgent:    turn.ToAgent,
							ToAgent:      mentioned,
							VisitedChain: visitedChain,
							Content:      lastMsg.ExtractText(),
						})
						if err != nil {
							fmt.Printf("[Worklist] rejected: %v\n", err)
						}
					}
				}
			}
		case a2a.TaskStatusInputRequired:
			fmt.Printf("[Turn %d] %s [input-required]: waiting for input\n", i+1, turn.ToAgent)
			// Suspend: in test mode, skip to next turn
			history = result.History
		case a2a.TaskStatusFailed:
			return fmt.Errorf("turn %d: agent %s failed", i+1, turn.ToAgent)
		case a2a.TaskStatusCanceled:
			return fmt.Errorf("turn %d: agent %s canceled", i+1, turn.ToAgent)
		}
	}

	// Process dynamic @mention tasks
	for o.worklist.HasPending() {
		entry := o.worklist.Next()
		if entry == nil {
			break
		}
		fmt.Printf("[Worklist] %s → %s\n", entry.FromAgent, entry.ToAgent)

		spanID := uuid.NewString()
		task := a2a.Task{
			ID:        entry.TaskID,
			ContextID: entry.ContextID,
			Status:    a2a.TaskStatusSubmitted,
			History:   history,
			Trace: &a2a.TraceInfo{
				TraceID:      o.traceID,
				SpanID:       spanID,
				ParentSpanID: parentSpanID,
			},
		}

		c := o.clients[entry.ToAgent]
		result, err := c.SendTaskSync(ctx, task, 30*time.Second)
		if err != nil {
			fmt.Printf("[Worklist] error: %v\n", err)
			o.worklist.MarkDone(entry.TaskID)
			continue
		}
		if result.Status == a2a.TaskStatusCompleted && len(result.History) > 0 {
			lastMsg := result.History[len(result.History)-1]
			history = result.History
			fmt.Printf("[Worklist] %s [completed]: %s\n", entry.ToAgent, lastMsg.ExtractText())
		}
		o.worklist.MarkDone(entry.TaskID)
	}

	// Print trace summary
	fmt.Printf("[Trace] TraceID=%s SpanChain: %s\n", o.traceID, traceChainSummary(o.script))

	return nil
}

func traceChainSummary(script *Script) string {
	var chain string
	for i, turn := range script.Turns {
		if i > 0 {
			chain += " → "
		}
		chain += turn.ToAgent
	}
	return chain
}
