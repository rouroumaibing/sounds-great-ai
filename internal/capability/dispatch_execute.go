package capability

import (
	"context"
	"errors"

	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/pack/orchestrator"
)

// BreedExecution is one successfully-executed dispatch entry, carrying the
// breed_id metadata so result_merge can aggregate by breed without matching
// stepID strings.
type BreedExecution struct {
	SubTaskID string           `json:"subtask_id"`
	BreedID   string           `json:"breed_id"`
	Output    *pack.TaskOutput `json:"output"`
}

// DispatchExecute is the thin capability adapter that decodes the dispatch
// plan + subtasks from Previous and delegates to Pack.ExecuteDispatch.
type DispatchExecute struct {
	executor pack.BreedExecutor
}

// NewDispatchExecute constructs a DispatchExecute with the given executor
// (typically *Pack, but mockable in tests).
func NewDispatchExecute(executor pack.BreedExecutor) *DispatchExecute {
	return &DispatchExecute{executor: executor}
}

func (d *DispatchExecute) Name() string    { return "dispatch_execute" }
func (d *DispatchExecute) Version() string { return "v1" }

func (d *DispatchExecute) Init(ctx context.Context) error { return nil }
func (d *DispatchExecute) Health() error                  { return nil }
func (d *DispatchExecute) Close() error                   { return nil }

// Run decodes dispatch_plan + subtasks from Previous, calls ExecuteDispatch,
// and assembles a control-flow output (Control=true).
func (d *DispatchExecute) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	if input.Previous == nil {
		return nil, errors.New("dispatch_execute: no Previous steps")
	}

	// 1. Decode dispatch_plan
	dispatchOut, ok := input.Previous["dispatch"]
	if !ok || dispatchOut == nil {
		return nil, errors.New("dispatch_execute: missing 'dispatch' in Previous")
	}
	planRaw, ok := dispatchOut.Data["dispatch_plan"]
	if !ok {
		return nil, errors.New("dispatch_execute: missing dispatch_plan in dispatch output")
	}
	var plan orchestrator.DispatchPlan
	if err := decodeData(planRaw, &plan); err != nil {
		return nil, errors.New("dispatch_execute: decode dispatch_plan: " + err.Error())
	}

	// 2. Decode subtasks
	decomposeOut, ok := input.Previous["decompose"]
	if !ok || decomposeOut == nil {
		return nil, errors.New("dispatch_execute: missing 'decompose' in Previous")
	}
	subtasksRaw, ok := decomposeOut.Data["subtasks"]
	if !ok {
		return nil, errors.New("dispatch_execute: missing subtasks in decompose output")
	}
	var subtasks []orchestrator.SubTask
	if err := decodeData(subtasksRaw, &subtasks); err != nil {
		return nil, errors.New("dispatch_execute: decode subtasks: " + err.Error())
	}

	// 3. Execute
	results, entryErrors, err := d.executor.ExecuteDispatch(ctx, plan, subtasks)
	if err != nil {
		// Whole-call failure → halt workflow
		return nil, errors.New("dispatch_execute: " + err.Error())
	}

	// 4. Build BreedExecution list by joining results with plan.Entries
	entryBySubtask := make(map[string]orchestrator.DispatchEntry, len(plan.Entries))
	for _, e := range plan.Entries {
		entryBySubtask[e.SubTaskID] = e
	}
	var executions []BreedExecution
	for subtaskID, out := range results {
		if out == nil {
			continue
		}
		entry := entryBySubtask[subtaskID]
		executions = append(executions, BreedExecution{
			SubTaskID: subtaskID,
			BreedID:   entry.BreedID,
			Output:    out,
		})
	}

	// 5. Return control-flow output (Control=true so result_merge skips it
	//    by structural marker, not stepID string matching).
	return &pack.TaskOutput{
		Control: true,
		Data: map[string]any{
			"dispatched_executions": executions,
			"errors":                entryErrors,
		},
	}, nil
}
