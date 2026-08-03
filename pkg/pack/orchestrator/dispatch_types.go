// Package orchestrator holds the protocol types for the orchestrator chain
// (decompose → dispatch → execute → merge). It lives under pkg/pack so the
// framework layer (Pack.ExecuteDispatch) can depend on it without creating a
// cycle, while keeping orchestrator-specific business types out of the
// pkg/pack root.
package orchestrator

// SubTask — task_decompose output unit
type SubTask struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	SuggestBreed string   `json:"suggest_breed"`
	DependsOn    []string `json:"depends_on"`
}

// DispatchEntry — agent_dispatch routing plan unit
type DispatchEntry struct {
	BreedID    string   `json:"breed_id"`
	SubTaskID  string   `json:"subtask_id"`
	Priority   int      `json:"priority"`
	DependsOn  []string `json:"depends_on"`
	Status     string   `json:"status"`
	SkipReason string   `json:"skip_reason"`
}

// DispatchPlan — agent_dispatch complete output
type DispatchPlan struct {
	Entries  []DispatchEntry `json:"entries"`
	MaxDepth int             `json:"max_depth"`
	Total    int             `json:"total"`
	Skipped  int             `json:"skipped"`
}
