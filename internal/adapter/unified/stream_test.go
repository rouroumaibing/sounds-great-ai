package unified

import "testing"

func TestStreamEventText(t *testing.T) {
	e := StreamEvent{Type: "text", Content: "hello"}
	if e.Type != "text" || e.Content != "hello" {
		t.Fatalf("unexpected event: %+v", e)
	}
}

func TestStreamEventIsError(t *testing.T) {
	e := StreamEvent{Type: "error", Content: "parse failed"}
	if !e.IsError() {
		t.Fatal("expected IsError()=true for type=error")
	}
	e2 := StreamEvent{Type: "text", Content: "ok"}
	if e2.IsError() {
		t.Fatal("expected IsError()=false for type=text")
	}
}

func TestStreamEventIsDone(t *testing.T) {
	e := StreamEvent{Type: "done"}
	if !e.IsDone() {
		t.Fatal("expected IsDone()=true for type=done")
	}
}
