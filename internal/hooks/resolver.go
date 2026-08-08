package hooks

type ResolveResult struct {
	Status string
	Reason string
	Vars   map[string]string
}

type Resolver interface {
	Resolve(input *AssemblerInput) ResolveResult
}

// IdentityResolver always fires, injecting breed identity.
type IdentityResolver struct{}

func (r *IdentityResolver) Resolve(input *AssemblerInput) ResolveResult {
	return ResolveResult{
		Status: "fired",
		Vars: map[string]string{
			"BreedID":         input.BreedID,
			"BreedName":       input.BreedName,
			"RoleDescription": input.RoleDescription,
		},
	}
}

// AlwaysFireResolver fires unconditionally (for restrictions, iron-laws, guardrails).
type AlwaysFireResolver struct{}

func (r *AlwaysFireResolver) Resolve(input *AssemblerInput) ResolveResult {
	return ResolveResult{Status: "fired"}
}

// PhaseAnchorResolver fires unconditionally, injecting current Phase.
type PhaseAnchorResolver struct{}

func (r *PhaseAnchorResolver) Resolve(input *AssemblerInput) ResolveResult {
	return ResolveResult{
		Status: "fired",
		Vars: map[string]string{
			"CurrentPhase": input.CurrentPhase,
		},
	}
}

// ReAnchorResolver fires only when ToolCallCount > 5.
type ReAnchorResolver struct{}

func (r *ReAnchorResolver) Resolve(input *AssemblerInput) ResolveResult {
	if input.ToolCallCount > 5 {
		return ResolveResult{Status: "fired"}
	}
	return ResolveResult{Status: "skipped", Reason: "tool_call_count_below_threshold"}
}

func DefaultResolvers() map[string]Resolver {
	return map[string]Resolver{
		"IdentityResolver":    &IdentityResolver{},
		"AlwaysFireResolver":  &AlwaysFireResolver{},
		"PhaseAnchorResolver": &PhaseAnchorResolver{},
		"ReAnchorResolver":    &ReAnchorResolver{},
		"LeaderRefResolver":   &LeaderRefResolver{},
	}
}
