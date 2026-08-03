package capability

import (
	"context"
	"errors"
	"testing"

	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/pack/orchestrator"
)

// stubExecutor implements pack.BreedExecutor for testing.
type stubExecutor struct {
	results     map[string]*pack.TaskOutput
	entryErrors map[string]string
	err         error
}

func (s *stubExecutor) ExecuteDispatch(ctx context.Context, plan orchestrator.DispatchPlan, subtasks []orchestrator.SubTask) (map[string]*pack.TaskOutput, map[string]string, error) {
	return s.results, s.entryErrors, s.err
}

func TestDispatchExecute_Name(t *testing.T) {
	de := NewDispatchExecute(&stubExecutor{})
	if de.Name() != "dispatch_execute" {
		t.Fatalf("name: %q", de.Name())
	}
	if de.Version() != "v1" {
		t.Fatalf("version: %q", de.Version())
	}
}

func TestDispatchExecute_Run_Success(t *testing.T) {
	exec := &stubExecutor{
		results: map[string]*pack.TaskOutput{
			"sub-1": {Data: map[string]any{"ok": true}},
		},
	}
	de := NewDispatchExecute(exec)

	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "xigou", SubTaskID: "sub-1", Status: "pending"},
		},
	}
	subtasks := []orchestrator.SubTask{{ID: "sub-1", Description: "task"}}

	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"dispatch":  {Data: map[string]any{"dispatch_plan": plan}},
			"decompose": {Data: map[string]any{"subtasks": subtasks}},
		},
	}
	out, err := de.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil || out.Data == nil {
		t.Fatal("nil output")
	}
	execs, ok := out.Data["dispatched_executions"].([]BreedExecution)
	if !ok {
		t.Fatalf("dispatched_executions wrong type: %T", out.Data["dispatched_executions"])
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].BreedID != "xigou" || execs[0].SubTaskID != "sub-1" {
		t.Fatalf("execution metadata wrong: %+v", execs[0])
	}
	if execs[0].Output == nil {
		t.Fatal("output nil")
	}
	// Control flag must be true
	if !out.Control {
		t.Fatal("Control must be true for dispatch_execute")
	}
}

func TestDispatchExecute_Run_MissingPlan(t *testing.T) {
	de := NewDispatchExecute(&stubExecutor{})
	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"decompose": {Data: map[string]any{"subtasks": []orchestrator.SubTask{}}},
		},
	}
	_, err := de.Run(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing dispatch_plan")
	}
}

func TestDispatchExecute_Run_MissingSubtasks(t *testing.T) {
	de := NewDispatchExecute(&stubExecutor{})
	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"dispatch": {Data: map[string]any{"dispatch_plan": orchestrator.DispatchPlan{}}},
		},
	}
	_, err := de.Run(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing subtasks")
	}
}

func TestDispatchExecute_Run_WholeCallError(t *testing.T) {
	exec := &stubExecutor{err: errors.New("max depth")}
	de := NewDispatchExecute(exec)

	plan := orchestrator.DispatchPlan{Entries: []orchestrator.DispatchEntry{{SubTaskID: "sub-1", Status: "pending"}}}
	subtasks := []orchestrator.SubTask{{ID: "sub-1"}}

	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"dispatch":  {Data: map[string]any{"dispatch_plan": plan}},
			"decompose": {Data: map[string]any{"subtasks": subtasks}},
		},
	}
	_, err := de.Run(context.Background(), input)
	if err == nil {
		t.Fatal("expected whole-call error to propagate")
	}
}

func TestDispatchExecute_Run_PartialErrorsDegrade(t *testing.T) {
	exec := &stubExecutor{
		results: map[string]*pack.TaskOutput{
			"sub-1": {Data: map[string]any{"ok": true}},
		},
		entryErrors: map[string]string{"sub-2": "boom"},
	}
	de := NewDispatchExecute(exec)

	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "xigou", SubTaskID: "sub-1", Status: "pending"},
			{BreedID: "demu", SubTaskID: "sub-2", Status: "pending"},
		},
	}
	subtasks := []orchestrator.SubTask{
		{ID: "sub-1"}, {ID: "sub-2"},
	}

	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"dispatch":  {Data: map[string]any{"dispatch_plan": plan}},
			"decompose": {Data: map[string]any{"subtasks": subtasks}},
		},
	}
	out, err := de.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("partial errors should degrade, not error: %v", err)
	}
	errs, ok := out.Data["errors"].(map[string]string)
	if !ok {
		t.Fatalf("errors wrong type: %T", out.Data["errors"])
	}
	if errs["sub-2"] != "boom" {
		t.Fatalf("expected boom, got %q", errs["sub-2"])
	}
}
