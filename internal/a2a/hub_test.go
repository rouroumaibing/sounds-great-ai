package a2a

import "testing"

func TestThreadCreation(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("test-task", []string{"bianmu", "xigou"})
	if thread.ID == "" {
		t.Fatal("expected non-empty thread ID")
	}
	if len(thread.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(thread.Participants))
	}
	if thread.Status != "active" {
		t.Errorf("status = %s, want active", thread.Status)
	}
}

func TestThreadIsolation(t *testing.T) {
	hub := NewHub(nil)
	t1 := hub.CreateThread("task1", []string{"bianmu"})
	t2 := hub.CreateThread("task2", []string{"xigou"})
	if t1.ID == t2.ID {
		t.Fatal("threads should have different IDs")
	}
	got1 := hub.GetThread(t1.ID)
	if got1.ID != t1.ID {
		t.Fatal("GetThread returned wrong thread")
	}
}

func TestThreadReviewRoundCount(t *testing.T) {
	thread := &Thread{ID: "test", ReviewRoundCount: 0}
	thread.IncrementReviewRound()
	if thread.ReviewRoundCount != 1 {
		t.Fatalf("expected 1, got %d", thread.ReviewRoundCount)
	}
	thread.ResetReviewRounds()
	if thread.ReviewRoundCount != 0 {
		t.Fatalf("expected 0 after reset, got %d", thread.ReviewRoundCount)
	}
}
