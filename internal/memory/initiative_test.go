package memory

import "testing"

func TestInitiativeAdoptSeed(t *testing.T) {
	ie := NewInitiativeEngine()
	seed := ie.AdoptSeed("bianmu", "user prefers concise responses", "cue", LaneTaste)
	if seed.BreedID != "bianmu" {
		t.Errorf("expected bianmu, got %s", seed.BreedID)
	}
	if seed.Origin != "cue" {
		t.Errorf("expected cue, got %s", seed.Origin)
	}
	got, ok := ie.GetSeed(seed.ID)
	if !ok {
		t.Fatal("seed not found")
	}
	if got.Content != "user prefers concise responses" {
		t.Errorf("unexpected content: %s", got.Content)
	}
}

func TestInitiativeSeedsForBreed(t *testing.T) {
	ie := NewInitiativeEngine()
	ie.AdoptSeed("bianmu", "seed-1", "cue", LaneTaste)
	ie.AdoptSeed("bianmu", "seed-2", "observation", LaneProfile)
	ie.AdoptSeed("jinmao", "seed-3", "cue", LaneEntity)

	bianmuSeeds := ie.SeedsForBreed("bianmu")
	if len(bianmuSeeds) != 2 {
		t.Fatalf("expected 2 seeds for bianmu, got %d", len(bianmuSeeds))
	}
	jinmaoSeeds := ie.SeedsForBreed("jinmao")
	if len(jinmaoSeeds) != 1 {
		t.Fatalf("expected 1 seed for jinmao, got %d", len(jinmaoSeeds))
	}
}

func TestInitiativeSeedsForLane(t *testing.T) {
	ie := NewInitiativeEngine()
	ie.AdoptSeed("bianmu", "seed-1", "cue", LaneTaste)
	ie.AdoptSeed("bianmu", "seed-2", "observation", LaneProfile)
	ie.AdoptSeed("jinmao", "seed-3", "cue", LaneTaste)

	tasteSeeds := ie.SeedsForLane(LaneTaste)
	if len(tasteSeeds) != 2 {
		t.Fatalf("expected 2 taste seeds, got %d", len(tasteSeeds))
	}
}

func TestInitiativeRecordAction(t *testing.T) {
	ie := NewInitiativeEngine()
	seed := ie.AdoptSeed("bianmu", "user wants tests", "cue", LaneLesson)
	action := ie.RecordAction(seed.ID, "ensure tests exist", "run go test ./...", 0.9)
	if action.SeedID != seed.ID {
		t.Error("action should reference seed")
	}
	if action.Confidence != 0.9 {
		t.Errorf("expected 0.9, got %f", action.Confidence)
	}
	actions := ie.ActionsForSeed(seed.ID)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
}

func TestInitiativeForgetSeed(t *testing.T) {
	ie := NewInitiativeEngine()
	seed := ie.AdoptSeed("bianmu", "temporary thought", "observation", LaneTaste)
	if !ie.ForgetSeed(seed.ID) {
		t.Fatal("forget failed")
	}
	if _, ok := ie.GetSeed(seed.ID); ok {
		t.Error("seed should be gone after forget")
	}
	if ie.ForgetSeed("nonexistent") {
		t.Error("forget nonexistent should return false")
	}
}

func TestInitiativeAllSeeds(t *testing.T) {
	ie := NewInitiativeEngine()
	ie.AdoptSeed("bianmu", "s1", "cue", LaneTaste)
	ie.AdoptSeed("jinmao", "s2", "cue", LaneEntity)
	all := ie.AllSeeds()
	if len(all) != 2 {
		t.Fatalf("expected 2 seeds, got %d", len(all))
	}
}
