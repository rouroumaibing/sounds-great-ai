package capability

import (
	"context"

	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"
)

type StreamResponse struct{}

func NewStreamResponse() *StreamResponse { return &StreamResponse{} }

func (s *StreamResponse) Name() string    { return "stream_response" }
func (s *StreamResponse) Version() string { return "v1" }

func (s *StreamResponse) Init(ctx context.Context) error { return nil }
func (s *StreamResponse) Health() error                  { return nil }
func (s *StreamResponse) Close() error                   { return nil }

// Run reads Previous["render"].Data["markdown"] and pushes a BARK_RESULT
// event via input.Sink. Degrades gracefully when Sink is nil or Send fails.
func (s *StreamResponse) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	markdown := ""
	if input.Previous != nil {
		if rOut, ok := input.Previous["render"]; ok && rOut != nil {
			if v, ok := rOut.Data["markdown"]; ok {
				if s, ok := v.(string); ok {
					markdown = s
				}
			}
		}
	}

	if input.Sink == nil {
		return &pack.TaskOutput{
			Data: map[string]any{"streamed": false},
		}, nil
	}

	sessionID := ""
	if input.Context != nil {
		sessionID = input.Context.SessionID
	}

	// Build BARK_RESULT event (protocol package already defines event types).
	ev := protocol.NewEvent(protocol.EventBarkResult, sessionID, map[string]any{"markdown": markdown})
	if err := input.Sink.Send(ctx, ev); err != nil {
		return &pack.TaskOutput{
			Data: map[string]any{
				"streamed":   false,
				"send_error": err.Error(),
			},
		}, nil
	}

	return &pack.TaskOutput{
		Data: map[string]any{"streamed": true},
	}, nil
}
