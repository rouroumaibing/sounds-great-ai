package orchestrator

// Turn represents one conversation turn
type Turn struct {
	FromAgent string // sender
	ToAgent   string // receiver
	Prompt    string // prompt (only first turn is hardcoded)
}

// Script holds the conversation script
type Script struct {
	Turns []Turn
}

// NewTestScript returns the 4-turn test script
func NewTestScript() *Script {
	return &Script{
		Turns: []Turn{
			{FromAgent: "", ToAgent: "AgentA", Prompt: "请向 Agent B 问好，并询问它是什么大模型"},
			{FromAgent: "AgentA", ToAgent: "AgentB"},
			{FromAgent: "AgentB", ToAgent: "AgentA"},
			{FromAgent: "AgentA", ToAgent: "AgentB"},
		},
	}
}
