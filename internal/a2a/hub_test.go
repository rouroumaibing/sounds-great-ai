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

func TestHandoffIncrementsReviewRound(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("review task", []string{"bianmu"})
	initialCount := thread.ReviewRoundCount

	hub.Handoff(thread, Handoff{FromBreed: "bianmu", ToBreed: "xigou", Artifact: "review this code"})

	if thread.ReviewRoundCount != initialCount+1 {
		t.Fatalf("expected review round %d, got %d", initialCount+1, thread.ReviewRoundCount)
	}
}

func TestHandoffAppendsHistory(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("review task", []string{"bianmu"})

	hub.Handoff(thread, Handoff{FromBreed: "bianmu", ToBreed: "xigou", Artifact: "review this code"})

	if len(thread.History) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(thread.History))
	}
	msg := thread.History[0]
	if msg.FromBreed != "bianmu" {
		t.Errorf("FromBreed = %q, want bianmu", msg.FromBreed)
	}
	if msg.Content != "review this code" {
		t.Errorf("Content = %q, want review this code", msg.Content)
	}
	if msg.Role != "handoff" {
		t.Errorf("Role = %q, want handoff", msg.Role)
	}
}

func TestHandoffAddsParticipant(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("review task", []string{"bianmu"})

	hub.Handoff(thread, Handoff{FromBreed: "bianmu", ToBreed: "xigou", Artifact: "code"})

	found := false
	for _, p := range thread.Participants {
		if p == "xigou" {
			found = true
		}
	}
	if !found {
		t.Fatal("xigou not found in participants after handoff")
	}
}

func TestHandoffDoesNotDuplicateParticipant(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("review task", []string{"bianmu", "xigou"})

	hub.Handoff(thread, Handoff{FromBreed: "bianmu", ToBreed: "xigou", Artifact: "code"})

	count := 0
	for _, p := range thread.Participants {
		if p == "xigou" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 occurrence of xigou, got %d", count)
	}
}
