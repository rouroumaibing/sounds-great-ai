package ops

import (
	"strings"
	"testing"
)

func TestLogBuffer_AddAndRecent(t *testing.T) {
	lb := NewLogBuffer(5)
	for i := 0; i < 3; i++ {
		lb.Add("INFO", "msg"+string(rune('0'+i)))
	}
	entries := lb.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	for i, e := range entries {
		expected := "msg" + string(rune('0'+i))
		if e.Message != expected {
			t.Errorf("entry %d: expected %q, got %q", i, expected, e.Message)
		}
	}
}

func TestLogBuffer_Overflow(t *testing.T) {
	lb := NewLogBuffer(3)
	for i := 0; i < 5; i++ {
		lb.Add("INFO", "msg"+string(rune('0'+i)))
	}
	entries := lb.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (capacity), got %d", len(entries))
	}
	// Should have the last 3: msg2, msg3, msg4
	expected := []string{"msg2", "msg3", "msg4"}
	for i, e := range entries {
		if e.Message != expected[i] {
			t.Errorf("entry %d: expected %q, got %q", i, expected[i], e.Message)
		}
	}
}

func TestLogBuffer_RecentN(t *testing.T) {
	lb := NewLogBuffer(10)
	for i := 0; i < 5; i++ {
		lb.Add("INFO", "msg"+string(rune('0'+i)))
	}
	entries := lb.Recent(2)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Message != "msg3" || entries[1].Message != "msg4" {
		t.Errorf("expected msg3,msg4 got %s,%s", entries[0].Message, entries[1].Message)
	}
}

func TestLogBuffer_Empty(t *testing.T) {
	lb := NewLogBuffer(5)
	entries := lb.Recent(10)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestLogBuffer_LargeOverflow(t *testing.T) {
	lb := NewLogBuffer(100)
	for i := 0; i < 250; i++ {
		lb.Add("INFO", "msg")
	}
	if lb.Len() != 100 {
		t.Errorf("expected Len=100, got %d", lb.Len())
	}
	entries := lb.Recent(50)
	if len(entries) != 50 {
		t.Fatalf("expected 50 entries, got %d", len(entries))
	}
}

func TestLogBuffer_WrapMultiple(t *testing.T) {
	lb := NewLogBuffer(4)
	// Write 10 entries (wraps 2.5 times)
	for i := 0; i < 10; i++ {
		lb.Add("INFO", "entry"+string(rune('A'+i)))
	}
	entries := lb.Recent(4)
	expected := []string{"entryG", "entryH", "entryI", "entryJ"}
	for i, e := range entries {
		if e.Message != expected[i] {
			t.Errorf("entry %d: expected %q, got %q", i, expected[i], e.Message)
		}
	}
}

func TestLogWriter_Tee(t *testing.T) {
	lb := NewLogBuffer(10)
	var buf strings.Builder
	w := NewLogWriter(&buf, lb)

	w.Write([]byte("hello\n"))
	w.Write([]byte("world\n"))

	if buf.String() != "hello\nworld\n" {
		t.Errorf("passthrough failed: got %q", buf.String())
	}
	entries := lb.Recent(10)
	if len(entries) != 2 {
		t.Fatalf("expected 2 buffered entries, got %d", len(entries))
	}
	if entries[0].Message != "hello" {
		t.Errorf("entry 0: expected 'hello', got %q", entries[0].Message)
	}
	if entries[1].Message != "world" {
		t.Errorf("entry 1: expected 'world', got %q", entries[1].Message)
	}
}

func TestLogWriter_StripsPrefix(t *testing.T) {
	lb := NewLogBuffer(10)
	var buf strings.Builder
	w := NewLogWriter(&buf, lb)

	w.Write([]byte("2026/08/08 10:00:00 my message\n"))

	entries := lb.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Message != "my message" {
		t.Errorf("expected 'my message', got %q", entries[0].Message)
	}
}
