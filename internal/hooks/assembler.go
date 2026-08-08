package hooks

type AssemblerInput struct {
	BreedID         string
	BreedName       string
	RoleDescription string
	Personality     string
	CurrentPhase    string
	PhasePrereqs    []string
	ToolCallCount   int
	TaskID          string

	// Leader context — populated by ws_handler from platform.Leader
	LeaderName         string
	LeaderHandles      string
	LeaderFirstMention string
}
