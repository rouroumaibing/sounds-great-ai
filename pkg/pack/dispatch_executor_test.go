package pack

import (
	"context"
	"errors"
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
		ID:           breedID,
		Name:         breedID,
		Capabilities: []CapabilityBinding{},
	}
	for _, c := range caps {
		breed.Capabilities = append(breed.Capabilities, CapabilityBinding{Name: c.Name(), Version: c.Version()})
		_ = p.RegisterCapability(c)
	}
	steps := []WorkflowStep{}
	for _, c := range caps {
		steps = append(steps, WorkflowStep{ID: c.Name(), CapabilityRef: c.Name() + ":" + c.Version()})
	}
	breed.Workflow = WorkflowConfig{Steps: steps}
	_ = p.Register(breed)
}

func TestExecuteDispatch_SameBreedMultiEntry_NoRace(t *testing.T) {
	p := New("test")
	cap1 := &stubCapability{id: "stub_cap", output: &TaskOutput{Data: map[string]any{"ok": true}}}
	registerStubBreed(p, "xigou", cap1)

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
	if atomic.LoadInt32(&cap1.calls) != 2 {
		t.Fatalf("stub called %d times, want 2", cap1.calls)
	}
}

func TestExecuteDispatch_TopologicalLayering_SerialDep(t *testing.T) {
	p := New("test")
	// sub-1's stub sleeps briefly so we can prove sub-2 did not start
	// until sub-1 finished (serial layering, not concurrent dispatch).
	cap1 := &stubCapability{id: "cap_a", delay: 50 * time.Millisecond}
	cap2 := &stubCapability{id: "cap_b"}
	registerStubBreed(p, "breed_a", cap1)
	registerStubBreed(p, "breed_b", cap2)

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

	// 1) Dependency injection: sub-2's Previous must contain sub-1's output,
	//    keyed by SubTaskID "sub-1". This proves the topo layering wired the
	//    upstream result into the downstream TaskInput.
	cap2.mu.Lock()
	captured := make(map[string]*TaskOutput, len(cap2.capturedPrevious))
	for k, v := range cap2.capturedPrevious {
		captured[k] = v
	}
	cap2.mu.Unlock()
	if len(captured) == 0 {
		t.Fatal("sub-2 stub did not capture a Previous map; topo layering did not inject upstream output")
	}
	upstream, ok := captured["sub-1"]
	if !ok {
		keys := make([]string, 0, len(captured))
		for k := range captured {
			keys = append(keys, k)
		}
		t.Fatalf("sub-2's Previous missing 'sub-1' key; got keys: %v", keys)
	}
	if upstream == nil {
		t.Fatal("sub-2's Previous['sub-1'] is nil")
	}
	// The injected value should be a shallow copy of results["sub-1"]
	// (sub-1's workflow output). Comparing Data maps proves the wiring
	// carried the actual upstream output, not a zero-value pointer.
	expected := results["sub-1"]
	if expected == nil {
		t.Fatal("results['sub-1'] is nil")
	}
	if !reflect.DeepEqual(upstream.Data, expected.Data) {
		t.Fatalf("sub-2's Previous['sub-1'].Data != results['sub-1'].Data; want shallow copy of sub-1's output")
	}

	// 2) Serialization: sub-2 must not start until sub-1 has finished.
	//    cap1.startedAt is captured before its 50ms delay; if layering is
	//    serial, cap2.startedAt - cap1.startedAt >= 50ms. If they ran
	//    concurrently the gap would be ~0.
	cap1.mu.Lock()
	cap1Start := cap1.startedAt
	cap1.mu.Unlock()
	cap2.mu.Lock()
	cap2Start := cap2.startedAt
	cap2.mu.Unlock()
	if cap1Start.IsZero() || cap2Start.IsZero() {
		t.Fatalf("missing start timestamps: cap1=%v cap2=%v", cap1Start, cap2Start)
	}
	elapsed := cap2Start.Sub(cap1Start)
	if elapsed < 50*time.Millisecond {
		t.Fatalf("sub-2 started %v after sub-1; want >= 50ms (proves serial layering, not concurrent)", elapsed)
	}
}

func TestExecuteDispatch_SingleEntryFailureDoesNotBreakOthers(t *testing.T) {
	p := New("test")
	capOK := &stubCapability{id: "ok_cap"}
	capFail := &stubCapability{id: "fail_cap", err: errors.New("boom")}
	registerStubBreed(p, "ok_breed", capOK)
	registerStubBreed(p, "fail_breed", capFail)

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

func TestExecuteDispatch_RecursionGuard_BlocksOrchestratorBreed(t *testing.T) {
	p := New("test")
	// Register a breed whose capability is named "dispatch_execute".
	// registerStubBreed registers the capability first so p.Register succeeds.
	dispatchCap := &stubCapability{id: "dispatch_execute"}
	registerStubBreed(p, "malign", dispatchCap)

	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "malign", SubTaskID: "sub-x", Status: "pending"},
		},
		MaxDepth: 3, Total: 1,
	}
	subtasks := []orchestrator.SubTask{{ID: "sub-x", Description: "recursion"}}

	results, entryErrors, err := p.ExecuteDispatch(context.Background(), plan, subtasks)
	if err != nil {
		t.Fatalf("err should be nil (per-entry failure, not whole-call): %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("no results expected, got %d", len(results))
	}
	msg, ok := entryErrors["sub-x"]
	if !ok || msg == "" {
		t.Fatal("expected disallowed_recursion error")
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
