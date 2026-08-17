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

func TestGetThreadNonexistent(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.GetThread("nonexistent-id")
	if thread != nil {
		t.Fatalf("expected nil for nonexistent thread, got %v", thread)
	}
}

func TestNewHubWithPrepopulatedThreads(t *testing.T) {
	preexisting := map[string]*Thread{
		"pre-1": {ID: "pre-1", Task: "existing task", Status: "active"},
	}
	hub := NewHub(preexisting)
	thread := hub.GetThread("pre-1")
	if thread == nil {
		t.Fatal("expected to find pre-populated thread")
	}
	if thread.Task != "existing task" {
		t.Errorf("expected task 'existing task', got %s", thread.Task)
	}
}

func TestCreateThreadEmptyParticipants(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("task", []string{})
	if thread == nil {
		t.Fatal("expected non-nil thread")
	}
	if len(thread.Participants) != 0 {
		t.Errorf("expected 0 participants, got %d", len(thread.Participants))
	}
	if thread.Status != "active" {
		t.Errorf("expected active status, got %s", thread.Status)
	}
}

func TestCreateThreadEmptyTask(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("", []string{"bianmu"})
	if thread == nil {
		t.Fatal("expected non-nil thread")
	}
	if thread.Task != "" {
		t.Errorf("expected empty task, got %s", thread.Task)
	}
}

func TestMultipleHandoffsAccumulateHistory(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("multi-handoff", []string{"bianmu"})

	hub.Handoff(thread, Handoff{FromBreed: "bianmu", ToBreed: "xigou", Artifact: "first"})
	hub.Handoff(thread, Handoff{FromBreed: "xigou", ToBreed: "jinmao", Artifact: "second"})
	hub.Handoff(thread, Handoff{FromBreed: "jinmao", ToBreed: "demu", Artifact: "third"})

	if len(thread.History) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(thread.History))
	}
	if thread.History[0].Content != "first" {
		t.Errorf("expected first artifact, got %s", thread.History[0].Content)
	}
	if thread.History[1].Content != "second" {
		t.Errorf("expected second artifact, got %s", thread.History[1].Content)
	}
	if thread.History[2].Content != "third" {
		t.Errorf("expected third artifact, got %s", thread.History[2].Content)
	}
	if thread.ReviewRoundCount != 3 {
		t.Errorf("expected 3 review rounds, got %d", thread.ReviewRoundCount)
	}
}

func TestHandoffEmptyArtifact(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("task", []string{"bianmu"})

	result, err := hub.Handoff(thread, Handoff{FromBreed: "bianmu", ToBreed: "xigou", Artifact: ""})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(thread.History) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(thread.History))
	}
	if thread.History[0].Content != "" {
		t.Errorf("expected empty content, got %s", thread.History[0].Content)
	}
}

func TestHandoffEmptyBreeds(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("task", []string{})

	result, err := hub.Handoff(thread, Handoff{FromBreed: "", ToBreed: "", Artifact: "code"})
	if err != nil {
		t.Fatalf("expected no error with empty breeds, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(thread.History) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(thread.History))
	}
	if thread.History[0].FromBreed != "" {
		t.Errorf("expected empty FromBreed, got %s", thread.History[0].FromBreed)
	}
}

func TestHandoffReturnsThread(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("task", []string{"bianmu"})

	result, err := hub.Handoff(thread, Handoff{FromBreed: "bianmu", ToBreed: "xigou", Artifact: "code"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != thread {
		t.Fatal("expected returned thread to be the same pointer as input")
	}
}

func TestHandoffAddsMultipleParticipants(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("task", []string{"bianmu"})

	hub.Handoff(thread, Handoff{FromBreed: "bianmu", ToBreed: "xigou", Artifact: "a"})
	hub.Handoff(thread, Handoff{FromBreed: "xigou", ToBreed: "jinmao", Artifact: "b"})
	hub.Handoff(thread, Handoff{FromBreed: "jinmao", ToBreed: "demu", Artifact: "c"})

	if len(thread.Participants) != 4 {
		t.Fatalf("expected 4 participants, got %d", len(thread.Participants))
	}
}

func TestHandoffWithReviewFlag(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("task", []string{"bianmu"})

	hub.Handoff(thread, Handoff{FromBreed: "bianmu", ToBreed: "xigou", Artifact: "review", ReviewFlag: true})

	if thread.ReviewRoundCount != 1 {
		t.Errorf("expected 1 review round, got %d", thread.ReviewRoundCount)
	}
}

func TestThreadIncrementMultipleRounds(t *testing.T) {
	thread := &Thread{ID: "test", ReviewRoundCount: 0}
	for i := 0; i < 5; i++ {
		thread.IncrementReviewRound()
	}
	if thread.ReviewRoundCount != 5 {
		t.Fatalf("expected 5 review rounds, got %d", thread.ReviewRoundCount)
	}
	thread.ResetReviewRounds()
	if thread.ReviewRoundCount != 0 {
		t.Fatalf("expected 0 after reset, got %d", thread.ReviewRoundCount)
	}
}

// TestHandoffRecordsVariants asserts that the executing variant identity is
// carried through the handoff into the thread history. The cross-model review
// gate keys on the variant-level dog_id, so recording the variant here is what
// lets a same-breed, different-model handoff be distinguished from a self
// handoff (the two resolve to different dog_ids).
func TestHandoffRecordsVariants(t *testing.T) {
	hub := NewHub(nil)
	thread := hub.CreateThread("task", []string{"bianmu"})

	hub.Handoff(thread, Handoff{
		FromBreed:   "bianmu",
		FromVariant: "bianmu-codex",
		ToBreed:     "bianmu",
		ToVariant:   "bianmu-claude",
		Artifact:    "review this",
	})

	msg := thread.History[len(thread.History)-1]
	if msg.FromVariant != "bianmu-codex" {
		t.Errorf("FromVariant = %q, want bianmu-codex", msg.FromVariant)
	}
	if msg.ToVariant != "bianmu-claude" {
		t.Errorf("ToVariant = %q, want bianmu-claude", msg.ToVariant)
	}
	if msg.FromBreed != "bianmu" || msg.ToBreed != "bianmu" {
		t.Errorf("breed labels should be preserved: from=%q to=%q", msg.FromBreed, msg.ToBreed)
	}
}
