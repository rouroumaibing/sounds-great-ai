package capability

import (
	"context"
	"errors"
	"testing"

	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"
)

// fakeSink is a test EventSink.
type fakeSink struct {
	sent    []*protocol.Event
	failErr error
}

func (f *fakeSink) Send(ctx context.Context, ev *protocol.Event) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.sent = append(f.sent, ev)
	return nil
}

func TestStreamResponse_Name(t *testing.T) {
	c := NewStreamResponse()
	if c.Name() != "stream_response" || c.Version() != "v1" {
		t.Fatalf("name/version: %q/%q", c.Name(), c.Version())
	}
}

func TestStreamResponse_Run_NilSink_Degrades(t *testing.T) {
	c := NewStreamResponse()
	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"render": {Data: map[string]any{"markdown": "# hello"}},
		},
		// Sink is nil
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	streamed, ok := out.Data["streamed"].(bool)
	if !ok || streamed {
		t.Fatalf("expected streamed=false, got %v", out.Data["streamed"])
	}
}

func TestStreamResponse_Run_SendsEvent(t *testing.T) {
	c := NewStreamResponse()
	sink := &fakeSink{}
	input := &pack.TaskInput{
		Sink: sink,
		Previous: map[string]*pack.TaskOutput{
			"render": {Data: map[string]any{"markdown": "# hello"}},
		},
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	streamed, _ := out.Data["streamed"].(bool)
	if !streamed {
		t.Fatal("expected streamed=true")
	}
	if len(sink.sent) == 0 {
		t.Fatal("no events sent")
	}
}

func TestStreamResponse_Run_SinkError_Degrades(t *testing.T) {
	c := NewStreamResponse()
	sink := &fakeSink{failErr: errors.New("conn closed")}
	input := &pack.TaskInput{
		Sink: sink,
		Previous: map[string]*pack.TaskOutput{
			"render": {Data: map[string]any{"markdown": "# hello"}},
		},
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err should be nil on sink error: %v", err)
	}
	streamed, _ := out.Data["streamed"].(bool)
	if streamed {
		t.Fatal("expected streamed=false on sink error")
	}
	if _, ok := out.Data["send_error"]; !ok {
		t.Fatal("expected send_error field")
	}
}
