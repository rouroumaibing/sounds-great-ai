package hooks

import "testing"

func TestAssemblerInput_Fields(t *testing.T) {
	input := AssemblerInput{
		BreedID:       "bianmu",
		BreedName:     "Border Collie",
		CurrentPhase:  "Phase 1",
		ToolCallCount: 3,
	}
	if input.BreedID != "bianmu" {
		t.Errorf("BreedID = %q", input.BreedID)
	}
	if input.ToolCallCount != 3 {
		t.Errorf("ToolCallCount = %d", input.ToolCallCount)
	}
}
