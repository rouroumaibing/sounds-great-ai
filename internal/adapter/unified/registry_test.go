package unified

import (
	"testing"
)

func TestProcessRegistry_RegisterAndGet(t *testing.T) {
	r := NewProcessRegistry()
	rec := r.Register(12345, "claude", "claude")
	if rec.PID != 12345 {
		t.Errorf("PID = %d, want 12345", rec.PID)
	}
	if rec.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", rec.Status, StatusRunning)
	}
	got := r.Get(12345)
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Command != "claude" {
		t.Errorf("Command = %q, want claude", got.Command)
	}
}

func TestProcessRegistry_UpdateExit(t *testing.T) {
	r := NewProcessRegistry()
	r.Register(100, "codex", "codex")
	code := 0
	r.UpdateExit(100, &code, "")
	got := r.Get(100)
	if got.Status != StatusExited {
		t.Errorf("Status = %q, want %q", got.Status, StatusExited)
	}
	if got.ExitCode == nil || *got.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", got.ExitCode)
	}
	if got.EndedAt == nil {
		t.Error("EndedAt should be set")
	}
}

func TestProcessRegistry_List(t *testing.T) {
	r := NewProcessRegistry()
	r.Register(1, "claude", "claude")
	r.Register(2, "codex", "codex")
	r.Register(3, "gemini", "gemini")
	list := r.List()
	if len(list) != 3 {
		t.Errorf("List len = %d, want 3", len(list))
	}
}

func TestProcessRegistry_Remove(t *testing.T) {
	r := NewProcessRegistry()
	r.Register(1, "claude", "claude")
	r.Remove(1)
	if r.Get(1) != nil {
		t.Error("Get after Remove should return nil")
	}
}

func TestProcessRegistry_Concurrent(t *testing.T) {
	r := NewProcessRegistry()
	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func(pid int) {
			r.Register(pid, "claude", "claude")
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}
	if len(r.List()) != 100 {
		t.Errorf("List len = %d, want 100", len(r.List()))
	}
}
