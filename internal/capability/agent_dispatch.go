package capability

import (
	"context"

	"sounds-great-ai/pkg/pack"
)

// AgentDispatch is a capability adapter that generates a routing plan from subtasks.
// Pure logic — no LLM calls.
type AgentDispatch struct{}

// NewAgentDispatch creates a new AgentDispatch capability.
func NewAgentDispatch() *AgentDispatch { return &AgentDispatch{} }

func (d *AgentDispatch) Name() string    { return "agent_dispatch" }
func (d *AgentDispatch) Version() string { return "v1" }

func (d *AgentDispatch) Init(ctx context.Context) error { return nil }
func (d *AgentDispatch) Health() error                  { return nil }
func (d *AgentDispatch) Close() error                   { return nil }

// Run generates a dispatch plan from decomposed subtasks.
func (d *AgentDispatch) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	subtasks := extractSubtasks(input)
	availableBreeds := getAvailableBreeds(input)
	maxDepth := getIntConfig(input.CapabilityConfig, "max_depth", 3)

	// Phase 1: Topological sort with cycle detection
	sorted, unresolved := topologicalSortDetectCycle(subtasks)
	unresolvedSet := make(map[string]bool, len(unresolved))
	for _, u := range unresolved {
		unresolvedSet[u.ID] = true
	}
	allTasks := append(sorted, unresolved...)

	plan := DispatchPlan{MaxDepth: maxDepth}
	seen := map[string]bool{}
	breedDepth := map[string]int{}
	subtaskStatus := map[string]string{}

	// Phase 2: Apply guards
	for i, st := range allTasks {
		entry := DispatchEntry{
			BreedID:   st.SuggestBreed,
			SubTaskID: st.ID,
			Priority:  i,
			DependsOn: st.DependsOn,
		}

		switch {
		case unresolvedSet[st.ID]:
			entry.Status = "blocked"
			entry.SkipReason = "cyclic_dependency"
		case hasSkippedDependency(st, subtaskStatus):
			entry.Status = "blocked"
			entry.SkipReason = "dependency_skipped"
		case !availableBreeds[st.SuggestBreed]:
			entry.Status = "skipped"
			entry.SkipReason = "invalid_breed"
		case breedDepth[st.SuggestBreed] >= maxDepth:
			entry.Status = "skipped"
			entry.SkipReason = "depth"
		case seen[dedupKey(st)]:
			entry.Status = "skipped"
			entry.SkipReason = "dedup"
		case isPingPong(st, plan.Entries):
			entry.Status = "skipped"
			entry.SkipReason = "pingpong"
		default:
			entry.Status = "pending"
			seen[dedupKey(st)] = true
			breedDepth[st.SuggestBreed]++
		}
		subtaskStatus[st.ID] = entry.Status
		plan.Entries = append(plan.Entries, entry)
	}
	plan.Total = len(plan.Entries)
	plan.Skipped = countSkipped(plan.Entries)

	return &pack.TaskOutput{
		Control: true,
		Data: map[string]any{
			"dispatch_plan": plan,
		},
	}, nil
}

// extractSubtasks pulls subtasks from the decompose step output.
func extractSubtasks(input *pack.TaskInput) []SubTask {
	if input.Previous == nil {
		return nil
	}
	decomposeOut, ok := input.Previous["decompose"]
	if !ok || decomposeOut == nil {
		return nil
	}
	subtasksAny, ok := decomposeOut.Data["subtasks"]
	if !ok {
		return nil
	}
	var subtasks []SubTask
	if err := decodeData(subtasksAny, &subtasks); err != nil {
		return nil
	}
	return subtasks
}

// topologicalSortDetectCycle returns sorted tasks and unresolved (cyclic) tasks.
func topologicalSortDetectCycle(subtasks []SubTask) (sorted []SubTask, unresolved []SubTask) {
	// Build adjacency and in-degree
	idMap := make(map[string]SubTask, len(subtasks))
	inDegree := make(map[string]int, len(subtasks))
	dependents := make(map[string][]string, len(subtasks))

	for _, st := range subtasks {
		idMap[st.ID] = st
		inDegree[st.ID] = 0
	}
	for _, st := range subtasks {
		for _, dep := range st.DependsOn {
			if _, ok := idMap[dep]; ok {
				dependents[dep] = append(dependents[dep], st.ID)
				inDegree[st.ID]++
			}
		}
	}

	// Kahn's algorithm
	queue := []string{}
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, idMap[id])
		for _, dep := range dependents[id] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// Remaining nodes with in-degree > 0 are in cycles
	for _, st := range subtasks {
		if inDegree[st.ID] > 0 {
			unresolved = append(unresolved, st)
		}
	}
	return sorted, unresolved
}

// hasSkippedDependency checks if any dependency was skipped or blocked.
func hasSkippedDependency(st SubTask, subtaskStatus map[string]string) bool {
	for _, depID := range st.DependsOn {
		s := subtaskStatus[depID]
		if s == "skipped" || s == "blocked" {
			return true
		}
	}
	return false
}

// isPingPong detects A→B + B→A patterns in existing entries.
func isPingPong(st SubTask, entries []DispatchEntry) bool {
	for _, e := range entries {
		if e.Status != "pending" {
			continue
		}
		// Check if e depends on st and st depends on e
		for _, dep := range st.DependsOn {
			if dep == e.SubTaskID {
				for _, eDep := range e.DependsOn {
					if eDep == st.ID {
						return true
					}
				}
			}
		}
	}
	return false
}

// countSkipped counts entries with status "skipped" or "blocked".
func countSkipped(entries []DispatchEntry) int {
	count := 0
	for _, e := range entries {
		if e.Status == "skipped" || e.Status == "blocked" {
			count++
		}
	}
	return count
}
