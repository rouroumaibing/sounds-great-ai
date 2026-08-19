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
	// Query 是当前用户查询，供 skill-trigger 等 resolver 做动态匹配（8.4）。
	Query string

	// Carrier 是当前执行目标 CLI 的 clientID（claude/codex/...），供 skill-trigger
	// resolver 按挂载范围过滤（G5）。
	Carrier string

	// Leader context — populated by ws_handler from platform.Leader
	LeaderName         string
	LeaderHandles      string
	LeaderFirstMention string
}
