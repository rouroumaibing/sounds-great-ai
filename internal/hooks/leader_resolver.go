package hooks

// LeaderRefResolver injects leader reference context.
// Fires when LeaderName is non-empty; skips otherwise.
type LeaderRefResolver struct{}

func (r *LeaderRefResolver) Resolve(input *AssemblerInput) ResolveResult {
	if input.LeaderName == "" {
		return ResolveResult{Status: "skipped", Reason: "no_leader_config"}
	}
	firstMention := input.LeaderFirstMention
	if firstMention == "" {
		firstMention = "@leader"
	}
	return ResolveResult{
		Status: "fired",
		Vars: map[string]string{
			"LeaderName":         input.LeaderName,
			"LeaderHandles":      input.LeaderHandles,
			"LeaderFirstMention": firstMention,
		},
	}
}
