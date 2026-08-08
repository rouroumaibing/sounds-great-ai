package pack

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sounds-great-ai/pkg/pack/orchestrator"
)

func TestShallowCopyOutput_Nil(t *testing.T) {
	if got := (*Pack)(nil).shallowCopyOutput(nil); got != nil {
		t.Fatalf("nil input should return nil, got %+v", got)
	}
}

func TestShallowCopyOutput_DataIsolation(t *testing.T) {
	src := &TaskOutput{
		Approved: true,
		Reason:   "src",
		Data:     map[string]any{"k1": "v1", "k2": "v2"},
		Results:  []any{"a", "b"},
	}
	cp := (*Pack)(nil).shallowCopyOutput(src)

	if cp == src {
		t.Fatal("returned same pointer")
	}
	if cp.Approved != src.Approved || cp.Reason != src.Reason {
		t.Fatalf("scalar fields not copied: %+v", cp)
	}
	// Mutate copy's Data — src must be unaffected
	cp.Data["k3"] = "v3"
	delete(cp.Data, "k1")
	if _, ok := src.Data["k1"]; !ok {
		t.Fatal("src Data mutated through copy")
	}
	if _, ok := src.Data["k3"]; ok {
		t.Fatal("src Data mutated through copy")
	}
	// Mutate copy's Results — src must be unaffected
	cp.Results[0] = "MUTATED"
	if src.Results[0] != "a" {
		t.Fatalf("src Results mutated: %v", src.Results)
	}
}

func TestShallowCopyOutput_SharedValueReference(t *testing.T) {
	// Confirm: shallow copy shares value references (not deep copy).
	// This is fine under the read-only contract.
	shared := []any{"x", "y"}
	src := &TaskOutput{Data: map[string]any{"list": shared}}
	cp := (*Pack)(nil).shallowCopyOutput(src)
	cpList, _ := cp.Data["list"].([]any)
	// Same underlying slice header value is shared; contract forbids mutation.
	if !reflect.DeepEqual(cpList, shared) {
		t.Fatalf("expected shared value, got %v vs %v", cpList, shared)
	}
}

// stubCapability is a test-only capability returning a fixed output.
type stubCapability struct {
	id     string
	output *TaskOutput
	err    error
	calls  int32
	delay  time.Duration

	// Captured for test inspection. Guarded by mu because the same stub
	// may be invoked concurrently (e.g. SameBreedMultiEntry test).
	mu               sync.Mutex
	startedAt        time.Time
	capturedPrevious map[string]*TaskOutput
}

func (s *stubCapability) Name() string                   { return s.id }
func (s *stubCapability) Version() string                { return "v1" }
func (s *stubCapability) Init(ctx context.Context) error { return nil }
func (s *stubCapability) Health() error                  { return nil }
func (s *stubCapability) Close() error                   { return nil }
func (s *stubCapability) Run(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	atomic.AddInt32(&s.calls, 1)

	// Snapshot start time + Previous map for post-test inspection.
	s.mu.Lock()
	s.startedAt = time.Now()
	if input.Previous != nil {
		s.capturedPrevious = make(map[string]*TaskOutput, len(input.Previous))
		for k, v := range input.Previous {
			s.capturedPrevious[k] = v
		}
	}
	s.mu.Unlock()

	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	// Return a copy so callers can detect mutation
	return &TaskOutput{
		Data: map[string]any{
			"subtask_id": input.Query,
			"breed":      s.id,
		},
	}, nil
}

// registerStubBreed wires up a breed config + capability, returns the pack.
func registerStubBreed(p *Pack, breedID string, caps ...Capability) {
	breed := &BreedConfig{
		ID:               breedID,
		Name:             breedID,
		DefaultVariantID: "v1",
		Variants: []Variant{{
			ID:           "v1",
			ClientID:     "test",
			DefaultModel: "test-model",
		}},
	}
	for _, c := range caps {
		_ = p.RegisterCapability(c)
	}
	_ = p.Register(breed)
}

func TestExecuteDispatch_SameBreedMultiEntry_NoRace(t *testing.T) {
	p := New("test")
	registerStubBreed(p, "xigou")

	// Two entries both routed to xigou (same breed, different subtasks)
	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "xigou", SubTaskID: "sub-1", Status: "pending"},
			{BreedID: "xigou", SubTaskID: "sub-2", Status: "pending"},
		},
		MaxDepth: 3,
		Total:    2,
	}
	subtasks := []orchestrator.SubTask{
		{ID: "sub-1", Description: "task one"},
		{ID: "sub-2", Description: "task two"},
	}

	results, entryErrors, err := p.ExecuteDispatch(context.Background(), plan, subtasks)
	if err != nil {
		t.Fatalf("ExecuteDispatch err: %v", err)
	}
	if len(entryErrors) != 0 {
		t.Fatalf("unexpected entryErrors: %v", entryErrors)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (keyed by SubTaskID), got %d: %v", len(results), results)
	}
	if _, ok := results["sub-1"]; !ok {
		t.Fatal("missing sub-1 result")
	}
	if _, ok := results["sub-2"]; !ok {
		t.Fatal("missing sub-2 result")
	}
}

func TestExecuteDispatch_TopologicalLayering_SerialDep(t *testing.T) {
	p := New("test")
	registerStubBreed(p, "breed_a")
	registerStubBreed(p, "breed_b")

	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "breed_b", SubTaskID: "sub-2", Status: "pending", DependsOn: []string{"sub-1"}},
			{BreedID: "breed_a", SubTaskID: "sub-1", Status: "pending"},
		},
		MaxDepth: 3, Total: 2,
	}
	subtasks := []orchestrator.SubTask{
		{ID: "sub-1", Description: "first"},
		{ID: "sub-2", Description: "second"},
	}

	results, entryErrors, err := p.ExecuteDispatch(context.Background(), plan, subtasks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(entryErrors) != 0 || len(results) != 2 {
		t.Fatalf("expected 2 results, no errors. got results=%d errors=%d", len(results), len(entryErrors))
	}
	if _, ok := results["sub-2"]; !ok {
		t.Fatal("missing sub-2")
	}
	if _, ok := results["sub-1"]; !ok {
		t.Fatal("missing sub-1")
	}
}

func TestExecuteDispatch_SingleEntryFailureDoesNotBreakOthers(t *testing.T) {
	p := New("test")
	registerStubBreed(p, "ok_breed")
	// "fail_breed" is not registered, so Bark will return an error for it.

	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "ok_breed", SubTaskID: "sub-1", Status: "pending"},
			{BreedID: "fail_breed", SubTaskID: "sub-2", Status: "pending"},
		},
		MaxDepth: 3, Total: 2,
	}
	subtasks := []orchestrator.SubTask{
		{ID: "sub-1", Description: "ok task"},
		{ID: "sub-2", Description: "fail task"},
	}

	results, entryErrors, err := p.ExecuteDispatch(context.Background(), plan, subtasks)
	if err != nil {
		t.Fatalf("whole-call err should be nil: %v", err)
	}
	if _, ok := results["sub-1"]; !ok {
		t.Fatal("sub-1 should have succeeded")
	}
	if msg, ok := entryErrors["sub-2"]; !ok || msg == "" {
		t.Fatal("sub-2 should have error in entryErrors")
	}
}

func TestExecuteDispatch_MaxDepthExceeded(t *testing.T) {
	p := New("test")
	cap1 := &stubCapability{id: "cap"}
	registerStubBreed(p, "b", cap1)

	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "b", SubTaskID: "sub-1", Status: "pending"},
		},
		MaxDepth: 2, Total: 1,
	}
	// Context with depth already at 3 (> MaxDepth)
	ctx := context.WithValue(context.Background(), dispatchDepthKey{}, 3)
	_, _, err := p.ExecuteDispatch(ctx, plan, []orchestrator.SubTask{{ID: "sub-1"}})
	if err != ErrMaxDepthExceeded {
		t.Fatalf("expected ErrMaxDepthExceeded, got %v", err)
	}
}
