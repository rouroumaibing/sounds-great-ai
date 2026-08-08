package memory

import "testing"

func TestDeltaProducerDetect(t *testing.T) {
	dp := NewDeltaProducer()
	messages := []SessionMessage{
		{Role: "user", Content: "I decided to use PostgreSQL for the database."},
		{Role: "assistant", Content: "Actually, we should use SQLite for simplicity."},
		{Role: "user", Content: "I am a full-stack developer."},
		{Role: "user", Content: "I prefer dark mode in the editor."},
	}
	delta := dp.Detect("session-1", messages)
	if len(delta.Decisions) == 0 {
		t.Error("expected decisions to be detected")
	}
	if len(delta.Corrections) == 0 {
		t.Error("expected corrections to be detected")
	}
	if len(delta.IdentityChanges) == 0 {
		t.Error("expected identity changes to be detected")
	}
	if len(delta.Preferences) == 0 {
		t.Error("expected preferences to be detected")
	}
}

func TestDeltaProducerProduce(t *testing.T) {
	dp := NewDeltaProducer()
	delta := &SessionDelta{
		SessionID:       "s1",
		Decisions:       []string{"decided to use Go"},
		Corrections:     []string{"actually, use SQLite"},
		IdentityChanges: []string{"I am a developer"},
		Preferences:     []string{"I prefer dark mode"},
	}
	candidates := dp.Produce(delta)
	if len(candidates) != 4 {
		t.Fatalf("expected 4 candidates, got %d", len(candidates))
	}
	// Verify lane mapping
	laneMap := map[string]LaneType{}
	for _, c := range candidates {
		laneMap[c.Content] = c.Lane
	}
	if laneMap["decided to use Go"] != LaneDecision {
		t.Error("decision should map to LaneDecision")
	}
	if laneMap["actually, use SQLite"] != LaneLesson {
		t.Error("correction should map to LaneLesson")
	}
	if laneMap["I am a developer"] != LaneProfile {
		t.Error("identity should map to LaneProfile")
	}
	if laneMap["I prefer dark mode"] != LaneTaste {
		t.Error("preference should map to LaneTaste")
	}
}

func TestDeltaProducerSubmitCandidates(t *testing.T) {
	dp := NewDeltaProducer()
	reg := NewLaneRegistry()
	candidates := []DeltaCandidate{
		{Lane: LaneDecision, Content: "decided to use Go", Source: "session:s1"},
		{Lane: LaneTaste, Content: "prefer dark mode", Source: "session:s1"},
	}
	ids := dp.SubmitCandidates(reg, candidates)
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}
	// Verify entries are pending
	for _, id := range ids {
		for _, lt := range reg.LaneTypes() {
			lane := reg.Lane(lt)
			if e, ok := lane.Get(id); ok {
				if e.Status != StatusPending {
					t.Errorf("entry %s should be pending, got %s", id, e.Status)
				}
			}
		}
	}
}

func TestDeltaProducerDetectAndSubmit(t *testing.T) {
	dp := NewDeltaProducer()
	reg := NewLaneRegistry()
	messages := []SessionMessage{
		{Role: "user", Content: "I decided to use React for frontend."},
	}
	ids := dp.DetectAndSubmit(reg, "session-1", messages)
	if len(ids) == 0 {
		t.Fatal("expected at least 1 submitted entry")
	}
}

func TestFirstSentence(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"hello world. rest", "hello world"},
		{"hello world! rest", "hello world"},
		{"hello world? rest", "hello world"},
		{"hello world\nrest", "hello world"},
		{"", ""},
	}
	for _, tc := range tests {
		got := firstSentence(tc.input)
		if got != tc.expect {
			t.Errorf("firstSentence(%q) = %q, want %q", tc.input, got, tc.expect)
		}
	}
}
