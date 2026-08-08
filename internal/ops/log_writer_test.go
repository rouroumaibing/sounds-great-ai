package ops

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogWriter_BasicWrite(t *testing.T) {
	var buf bytes.Buffer
	lb := NewLogBuffer(100)
	w := NewLogWriter(&buf, lb)
	n, err := w.Write([]byte("2026/01/01 12:00:00 test message\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 33 {
		t.Errorf("n = %d, want 33", n)
	}
	entries := lb.All()
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if !strings.Contains(entries[0].Message, "test message") {
		t.Errorf("Message = %q", entries[0].Message)
	}
}

func TestLogWriter_PartialLine(t *testing.T) {
	var buf bytes.Buffer
	lb := NewLogBuffer(100)
	w := NewLogWriter(&buf, lb)
	w.Write([]byte("partial line"))
	entries := lb.All()
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0 for partial line", len(entries))
	}
	w.Write([]byte(" continued\n"))
	entries = lb.All()
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 after completion", len(entries))
	}
}

func TestLogWriter_WithLevel(t *testing.T) {
	var buf bytes.Buffer
	lb := NewLogBuffer(100)
	w := NewLogWriterWithLevel(&buf, lb, "ERROR")
	w.Write([]byte("error message\n"))
	entries := lb.All()
	if entries[0].Level != "ERROR" {
		t.Errorf("Level = %q, want %q", entries[0].Level, "ERROR")
	}
}

func TestLogWriter_Flush(t *testing.T) {
	var buf bytes.Buffer
	lb := NewLogBuffer(100)
	w := NewLogWriter(&buf, lb)
	w.Write([]byte("unflushed line"))
	w.Flush()
	entries := lb.All()
	if len(entries) != 1 {
		t.Fatalf("got %d entries after Flush, want 1", len(entries))
	}
}
