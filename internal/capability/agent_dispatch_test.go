package capability

import (
	"context"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func makeDispatchInput(subtasks []SubTask) *pack.TaskInput {
	return &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"decompose": {
				Data: map[string]any{
					"subtasks": subtasks,
				},
			},
		},
	}
}

func TestAgentDispatchBasic(t *testing.T) {
	t.Parallel()
	d := NewAgentDispatch()
	input := makeDispatchInput([]SubTask{
		{ID: "sub-1", Title: "Search", SuggestBreed: "xigou"},
		{ID: "sub-2", Title: "Analyze", SuggestBreed: "xigou", DependsOn: []string{"sub-1"}},
		{ID: "sub-3", Title: "Retrieve", SuggestBreed: "jinmao"},
	})

	out, err := d.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var plan DispatchPlan
	if err := decodeData(out.Data["dispatch_plan"], &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	pending := 0
	for _, e := range plan.Entries {
		if e.Status == "pending" {
			pending++
		}
	}
	if pending != 3 {
		t.Errorf("expected 3 pending, got %d", pending)
	}
}

func TestAgentDispatchDepthLimit(t *testing.T) {
	t.Parallel()
	d := NewAgentDispatch()
	input := makeDispatchInput([]SubTask{
		{ID: "sub-1", Title: "A", SuggestBreed: "xigou"},
		{ID: "sub-2", Title: "B", SuggestBreed: "xigou"},
		{ID: "sub-3", Title: "C", SuggestBreed: "xigou"},
		{ID: "sub-4", Title: "D", SuggestBreed: "xigou"},
	})
	input.CapabilityConfig = map[string]any{"max_depth": float64(2)}

	out, _ := d.Run(context.Background(), input)
	var plan DispatchPlan
	decodeData(out.Data["dispatch_plan"], &plan)
	depthSkipped := 0
	for _, e := range plan.Entries {
		if e.SkipReason == "depth" {
			depthSkipped++
		}
	}
	if depthSkipped != 2 {
		t.Errorf("expected 2 depth-skipped, got %d", depthSkipped)
	}
}

func TestAgentDispatchDedup(t *testing.T) {
	t.Parallel()
	d := NewAgentDispatch()
	input := makeDispatchInput([]SubTask{
		{ID: "sub-1", Title: "Search", Description: "Find code", SuggestBreed: "xigou"},
		{ID: "sub-2", Title: "Search", Description: "Find code", SuggestBreed: "xigou"},
	})

	out, _ := d.Run(context.Background(), input)
	var plan DispatchPlan
	decodeData(out.Data["dispatch_plan"], &plan)
	dedupSkipped := 0
	for _, e := range plan.Entries {
		if e.SkipReason == "dedup" {
			dedupSkipped++
		}
	}
	if dedupSkipped != 1 {
		t.Errorf("expected 1 dedup-skipped, got %d", dedupSkipped)
	}
}

func TestAgentDispatchPingPong(t *testing.T) {
	t.Parallel()
	d := NewAgentDispatch()
	input := makeDispatchInput([]SubTask{
		{ID: "sub-1", Title: "A to B", SuggestBreed: "bianmu", DependsOn: []string{"sub-2"}},
		{ID: "sub-2", Title: "B to A", SuggestBreed: "xigou", DependsOn: []string{"sub-1"}},
	})

	out, _ := d.Run(context.Background(), input)
	var plan DispatchPlan
	decodeData(out.Data["dispatch_plan"], &plan)
	// At least one should be blocked as cyclic
	blocked := 0
	for _, e := range plan.Entries {
		if e.Status == "blocked" {
			blocked++
		}
	}
	if blocked == 0 {
		t.Error("expected at least 1 blocked for cyclic dependency")
	}
}

func TestAgentDispatchInvalidBreed(t *testing.T) {
	t.Parallel()
	d := NewAgentDispatch()
	input := makeDispatchInput([]SubTask{
		{ID: "sub-1", Title: "Task", SuggestBreed: "nonexistent_breed"},
	})

	out, _ := d.Run(context.Background(), input)
	var plan DispatchPlan
	decodeData(out.Data["dispatch_plan"], &plan)
	if plan.Entries[0].SkipReason != "invalid_breed" {
		t.Errorf("expected invalid_breed, got %q", plan.Entries[0].SkipReason)
	}
}

func TestAgentDispatchTopologicalSort(t *testing.T) {
	t.Parallel()
	d := NewAgentDispatch()
	input := makeDispatchInput([]SubTask{
		{ID: "sub-1", Title: "First", SuggestBreed: "xigou", DependsOn: []string{"sub-2"}},
		{ID: "sub-2", Title: "Second", SuggestBreed: "jinmao"},
	})

	out, _ := d.Run(context.Background(), input)
	var plan DispatchPlan
	decodeData(out.Data["dispatch_plan"], &plan)
	// sub-2 should come before sub-1 (it has no deps)
	if plan.Entries[0].SubTaskID != "sub-2" {
		t.Errorf("expected sub-2 first, got %s", plan.Entries[0].SubTaskID)
	}
}

func TestAgentDispatchCyclicDependency(t *testing.T) {
	t.Parallel()
	d := NewAgentDispatch()
	input := makeDispatchInput([]SubTask{
		{ID: "sub-1", Title: "A", SuggestBreed: "xigou", DependsOn: []string{"sub-2"}},
		{ID: "sub-2", Title: "B", SuggestBreed: "jinmao", DependsOn: []string{"sub-1"}},
	})

	out, _ := d.Run(context.Background(), input)
	var plan DispatchPlan
	decodeData(out.Data["dispatch_plan"], &plan)
	cyclic := 0
	for _, e := range plan.Entries {
		if e.SkipReason == "cyclic_dependency" {
			cyclic++
		}
	}
	if cyclic != 2 {
		t.Errorf("expected 2 cyclic_dependency, got %d", cyclic)
	}
	// All entries must be present
	if plan.Total != 2 {
		t.Errorf("expected Total=2, got %d", plan.Total)
	}
}

func TestAgentDispatchCascadeSkip(t *testing.T) {
	t.Parallel()
	d := NewAgentDispatch()
	input := makeDispatchInput([]SubTask{
		{ID: "sub-1", Title: "Invalid", SuggestBreed: "nonexistent"},
		{ID: "sub-2", Title: "Depends", SuggestBreed: "xigou", DependsOn: []string{"sub-1"}},
	})

	out, _ := d.Run(context.Background(), input)
	var plan DispatchPlan
	decodeData(out.Data["dispatch_plan"], &plan)
	if plan.Entries[1].SkipReason != "dependency_skipped" {
		t.Errorf("expected dependency_skipped for sub-2, got %q", plan.Entries[1].SkipReason)
	}
}
