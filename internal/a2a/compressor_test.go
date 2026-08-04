package a2a

import "testing"

func TestCompressHandoffFallback(t *testing.T) {
	c := NewContextCompressor()
	thread := &Thread{
		ID: "test",
		History: []Message{
			{Content: "msg1"}, {Content: "msg2"}, {Content: "msg3"},
		},
	}
	msgs := c.CompressHandoffFallback(thread, 2)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "msg2" {
		t.Errorf("expected msg2, got %s", msgs[0].Content)
	}
}
