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

func TestCompressHandoffFallbackNZero(t *testing.T) {
	c := NewContextCompressor()
	thread := &Thread{
		ID: "test",
		History: []Message{
			{Content: "msg1"}, {Content: "msg2"}, {Content: "msg3"},
		},
	}
	msgs := c.CompressHandoffFallback(thread, 0)
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages when n=0 (returns tail of length 0), got %d", len(msgs))
	}
}

func TestCompressHandoffFallbackNLargerThanHistory(t *testing.T) {
	c := NewContextCompressor()
	thread := &Thread{
		ID: "test",
		History: []Message{
			{Content: "msg1"}, {Content: "msg2"},
		},
	}
	msgs := c.CompressHandoffFallback(thread, 10)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages when n > len, got %d", len(msgs))
	}
	if msgs[0].Content != "msg1" {
		t.Errorf("expected msg1, got %s", msgs[0].Content)
	}
}

func TestCompressHandoffFallbackNOne(t *testing.T) {
	c := NewContextCompressor()
	thread := &Thread{
		ID: "test",
		History: []Message{
			{Content: "msg1"}, {Content: "msg2"}, {Content: "msg3"},
		},
	}
	msgs := c.CompressHandoffFallback(thread, 1)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "msg3" {
		t.Errorf("expected msg3 (last), got %s", msgs[0].Content)
	}
}

func TestCompressHandoffFallbackEmptyHistory(t *testing.T) {
	c := NewContextCompressor()
	thread := &Thread{
		ID:     "test",
		History: []Message{},
	}
	msgs := c.CompressHandoffFallback(thread, 5)
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages for empty history, got %d", len(msgs))
	}
}

func TestCompressHandoffFallbackNilHistory(t *testing.T) {
	c := NewContextCompressor()
	thread := &Thread{
		ID: "test",
	}
	msgs := c.CompressHandoffFallback(thread, 5)
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages for nil history, got %d", len(msgs))
	}
}

func TestCompressHandoffFallbackExactMatch(t *testing.T) {
	c := NewContextCompressor()
	thread := &Thread{
		ID: "test",
		History: []Message{
			{Content: "msg1"}, {Content: "msg2"}, {Content: "msg3"},
		},
	}
	msgs := c.CompressHandoffFallback(thread, 3)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages when n == len, got %d", len(msgs))
	}
	if msgs[0].Content != "msg1" {
		t.Errorf("expected msg1, got %s", msgs[0].Content)
	}
}

func TestCompressHandoffFallbackTableDriven(t *testing.T) {
	c := NewContextCompressor()
	history := []Message{
		{Content: "msg1"}, {Content: "msg2"}, {Content: "msg3"}, {Content: "msg4"}, {Content: "msg5"},
	}
	tests := []struct {
		name         string
		n            int
		expectedLen  int
		firstContent string
	}{
		{"n=1 returns last", 1, 1, "msg5"},
		{"n=2 returns last 2", 2, 2, "msg4"},
		{"n=3 returns last 3", 3, 3, "msg3"},
		{"n=5 exact match", 5, 5, "msg1"},
		{"n=10 larger than history", 10, 5, "msg1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &Thread{ID: "test", History: history}
			msgs := c.CompressHandoffFallback(thread, tt.n)
			if len(msgs) != tt.expectedLen {
				t.Fatalf("expected %d messages, got %d", tt.expectedLen, len(msgs))
			}
			if msgs[0].Content != tt.firstContent {
				t.Errorf("expected first content %s, got %s", tt.firstContent, msgs[0].Content)
			}
		})
	}
}
