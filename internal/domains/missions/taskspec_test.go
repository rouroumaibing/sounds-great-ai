package missions

import (
	"errors"
	"testing"
)

func TestTaskSpec_Transitions(t *testing.T) {
	cases := []struct {
		from, to TaskStatus
		ok       bool
	}{
		{TaskOpen, TaskScheduled, true},
		{TaskOpen, TaskCancelled, true},
		{TaskScheduled, TaskRunning, true},
		{TaskScheduled, TaskCancelled, true},
		{TaskRunning, TaskDone, true},
		{TaskRunning, TaskFailed, true},
		{TaskDone, TaskScheduled, true}, // recurring reschedule
		{TaskFailed, TaskScheduled, true},
		{TaskCancelled, TaskOpen, false}, // terminal
		{TaskOpen, TaskRunning, false},   // must schedule first
		{TaskRunning, TaskOpen, false},
	}
	for _, c := range cases {
		task := &TaskSpec{ID: "x", Status: c.from}
		err := task.Transition(c.to)
		if c.ok && err != nil {
			t.Fatalf("%s->%s: expected ok, got %v", c.from, c.to, err)
		}
		if !c.ok {
			if err == nil {
				t.Fatalf("%s->%s: expected error", c.from, c.to)
			}
			if task.Status != c.from {
				t.Fatalf("%s->%s: status changed on invalid move", c.from, c.to)
			}
		}
	}
}

func TestTaskSpec_TimestampsAndError(t *testing.T) {
	task := NewTaskSpec("t1", "title", "u1")
	if task.Status != TaskOpen {
		t.Fatal("new task must be open")
	}
	_ = task.Transition(TaskScheduled)
	if task.ScheduledAt.IsZero() {
		t.Fatal("scheduled at must be set")
	}
	_ = task.Transition(TaskRunning)
	_ = task.Transition(TaskFailed)
	if task.FinishedAt.IsZero() {
		t.Fatal("finished at must be set")
	}
	if task.LastError != "" {
		t.Fatal("failed transition should clear last error only on running")
	}
}

func TestMissionHub_RegisterGetList(t *testing.T) {
	h := NewMissionHub()
	t1 := NewTaskSpec("a", "one", "u1")
	if err := h.Register(t1); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := h.Register(t1); !errors.Is(err, ErrTaskExists) {
		t.Fatal("duplicate register must error")
	}
	got, err := h.Get("a")
	if err != nil || got != t1 {
		t.Fatalf("get: %v %v", got, err)
	}
	if _, err := h.Get("missing"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatal("missing get must error")
	}
	if len(h.List()) != 1 {
		t.Fatal("list length")
	}
}

func TestMissionHub_ListForThread(t *testing.T) {
	h := NewMissionHub()
	a := NewTaskSpec("a", "one", "u1")
	a.ThreadID = "th1"
	b := NewTaskSpec("b", "two", "u1")
	b.ThreadID = "th2"
	_ = h.Register(a)
	_ = h.Register(b)
	if len(h.ListForThread("th1")) != 1 {
		t.Fatal("thread filter")
	}
}

func TestMissionHub_UpdateStatus(t *testing.T) {
	h := NewMissionHub()
	t1 := NewTaskSpec("a", "one", "u1")
	_ = h.Register(t1)
	if err := h.UpdateStatus("a", TaskScheduled); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := h.Get("a")
	if got.Status != TaskScheduled {
		t.Fatal("status not updated")
	}
	// scheduled -> running is a valid transition
	if err := h.UpdateStatus("a", TaskRunning); err != nil {
		t.Fatalf("scheduled->running should be valid: %v", err)
	}
	// running -> done is valid
	if err := h.UpdateStatus("a", TaskDone); err != nil {
		t.Fatalf("running->done should be valid: %v", err)
	}
	// done is terminal: done -> running must be invalid
	if err := h.UpdateStatus("a", TaskRunning); err == nil {
		t.Fatal("done->running must be invalid (terminal)")
	}
	// negative: open -> done directly is invalid
	t2 := NewTaskSpec("b", "two", "u1")
	_ = h.Register(t2)
	if err := h.UpdateStatus("b", TaskDone); err == nil {
		t.Fatal("open->done must be invalid")
	}
}
