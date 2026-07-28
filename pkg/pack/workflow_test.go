package pack

import (
	"context"
	"sort"
	"testing"
)

func TestTopologicalSortLinear(t *testing.T) {
	steps := []WorkflowStep{
		{ID: "a", CapabilityRef: "cap:v1"},
		{ID: "b", CapabilityRef: "cap:v1", Depends: []string{"a"}},
		{ID: "c", CapabilityRef: "cap:v1", Depends: []string{"b"}},
	}
	layers, err := topologicalSort(steps)
	if err != nil {
		t.Fatalf("topologicalSort: %v", err)
	}
	if len(layers) != 3 {
		t.Fatalf("layers count = %d, want 3", len(layers))
	}
	if layers[0][0].ID != "a" {
		t.Errorf("layer 0 = %q, want %q", layers[0][0].ID, "a")
	}
	if layers[1][0].ID != "b" {
		t.Errorf("layer 1 = %q, want %q", layers[1][0].ID, "b")
	}
	if layers[2][0].ID != "c" {
		t.Errorf("layer 2 = %q, want %q", layers[2][0].ID, "c")
	}
}

func TestTopologicalSortParallel(t *testing.T) {
	// WolfDog pattern: trace → [diagnose, profile]
	steps := []WorkflowStep{
		{ID: "trace", CapabilityRef: "cap:v1"},
		{ID: "diagnose", CapabilityRef: "cap:v1", Depends: []string{"trace"}},
		{ID: "profile", CapabilityRef: "cap:v1", Depends: []string{"trace"}},
	}
	layers, err := topologicalSort(steps)
	if err != nil {
		t.Fatalf("topologicalSort: %v", err)
	}
	if len(layers) != 2 {
		t.Fatalf("layers count = %d, want 2", len(layers))
	}
	if len(layers[0]) != 1 {
		t.Fatalf("layer 0 size = %d, want 1", len(layers[0]))
	}
	if len(layers[1]) != 2 {
		t.Fatalf("layer 1 size = %d, want 2", len(layers[1]))
	}
	// Verify layer 1 contains both diagnose and profile
	layer1IDs := []string{layers[1][0].ID, layers[1][1].ID}
	sort.Strings(layer1IDs)
	if layer1IDs[0] != "diagnose" || layer1IDs[1] != "profile" {
		t.Errorf("layer 1 = %v, want [diagnose profile]", layer1IDs)
	}
}

func TestTopologicalSortCycleDetection(t *testing.T) {
	steps := []WorkflowStep{
		{ID: "a", CapabilityRef: "cap:v1", Depends: []string{"b"}},
		{ID: "b", CapabilityRef: "cap:v1", Depends: []string{"a"}},
	}
	_, err := topologicalSort(steps)
	if err == nil {
		t.Error("expected cycle detection error, got nil")
	}
}

func TestTopologicalSortMissingDependency(t *testing.T) {
	steps := []WorkflowStep{
		{ID: "a", CapabilityRef: "cap:v1", Depends: []string{"nonexistent"}},
	}
	_, err := topologicalSort(steps)
	if err == nil {
		t.Error("expected missing dependency error, got nil")
	}
}

func TestTopologicalSortEmpty(t *testing.T) {
	layers, err := topologicalSort([]WorkflowStep{})
	if err != nil {
		t.Fatalf("topologicalSort empty: %v", err)
	}
	if len(layers) != 0 {
		t.Errorf("layers count = %d, want 0", len(layers))
	}
}

func TestBarkLinearWorkflow(t *testing.T) {
	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	breed := &BreedConfig{
		ID:           "breed1",
		Capabilities: []CapabilityBinding{{Name: "cap1", Version: "v1"}},
		Workflow: WorkflowConfig{
			Steps: []WorkflowStep{
				{ID: "s1", CapabilityRef: "cap1:v1"},
				{ID: "s2", CapabilityRef: "cap1:v1", Depends: []string{"s1"}},
			},
		},
		Source: BreedSourceUser,
	}
	p.Register(breed)

	out, err := p.Bark(context.Background(), "breed1", &TaskInput{})
	if err != nil {
		t.Fatalf("Bark: %v", err)
	}
	steps := out.Data["steps"].(map[string]*TaskOutput)
	if len(steps) != 2 {
		t.Errorf("steps count = %d, want 2", len(steps))
	}
	if !steps["s1"].Approved || !steps["s2"].Approved {
		t.Error("both steps should be approved")
	}
}

func TestBarkParallelWorkflow(t *testing.T) {
	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	breed := &BreedConfig{
		ID:           "demu",
		Capabilities: []CapabilityBinding{{Name: "cap1", Version: "v1"}},
		Workflow: WorkflowConfig{
			Steps: []WorkflowStep{
				{ID: "trace", CapabilityRef: "cap1:v1"},
				{ID: "diagnose", CapabilityRef: "cap1:v1", Depends: []string{"trace"}},
				{ID: "profile", CapabilityRef: "cap1:v1", Depends: []string{"trace"}},
			},
		},
		Source: BreedSourceUser,
	}
	p.Register(breed)

	out, err := p.Bark(context.Background(), "demu", &TaskInput{})
	if err != nil {
		t.Fatalf("Bark: %v", err)
	}
	steps := out.Data["steps"].(map[string]*TaskOutput)
	if len(steps) != 3 {
		t.Errorf("steps count = %d, want 3", len(steps))
	}
}

func TestBarkBreedNotFound(t *testing.T) {
	p := New("test")
	_, err := p.Bark(context.Background(), "nonexistent", &TaskInput{})
	if err == nil {
		t.Error("expected error for nonexistent breed, got nil")
	}
}

func TestBarkPreviousMapPopulated(t *testing.T) {
	p := New("test")
	p.RegisterCapability(&prevRecordingCap{name: "cap1", version: "v1"})

	breed := &BreedConfig{
		ID:           "breed1",
		Capabilities: []CapabilityBinding{{Name: "cap1", Version: "v1"}},
		Workflow: WorkflowConfig{
			Steps: []WorkflowStep{
				{ID: "a", CapabilityRef: "cap1:v1"},
				{ID: "b", CapabilityRef: "cap1:v1", Depends: []string{"a"}},
				{ID: "c", CapabilityRef: "cap1:v1", Depends: []string{"b"}},
			},
		},
		Source: BreedSourceUser,
	}
	p.Register(breed)

	out, err := p.Bark(context.Background(), "breed1", &TaskInput{})
	if err != nil {
		t.Fatalf("Bark: %v", err)
	}
	steps := out.Data["steps"].(map[string]*TaskOutput)

	// Step "a" has no previous
	aPrev := steps["a"].Data["prev"].([]string)
	if len(aPrev) != 0 {
		t.Errorf("step a prev = %v, want empty", aPrev)
	}

	// Step "b" has previous "a"
	bPrev := steps["b"].Data["prev"].([]string)
	if len(bPrev) != 1 || bPrev[0] != "a" {
		t.Errorf("step b prev = %v, want [a]", bPrev)
	}

	// Step "c" has previous "b"
	cPrev := steps["c"].Data["prev"].([]string)
	if len(cPrev) != 1 || cPrev[0] != "b" {
		t.Errorf("step c prev = %v, want [b]", cPrev)
	}
}

func TestBarkCapabilityConfigInjected(t *testing.T) {
	p := New("test")
	p.RegisterCapability(&configRecordingCap{name: "cap1", version: "v1"})

	breed := &BreedConfig{
		ID:           "breed1",
		Capabilities: []CapabilityBinding{{Name: "cap1", Version: "v1", Config: map[string]any{"top_k": float64(5)}}},
		Workflow: WorkflowConfig{
			Steps: []WorkflowStep{{ID: "s1", CapabilityRef: "cap1:v1"}},
		},
		Source: BreedSourceUser,
	}
	p.Register(breed)

	out, err := p.Bark(context.Background(), "breed1", &TaskInput{})
	if err != nil {
		t.Fatalf("Bark: %v", err)
	}
	steps := out.Data["steps"].(map[string]*TaskOutput)
	topK := steps["s1"].Data["top_k"]
	if topK != float64(5) {
		t.Errorf("top_k = %v, want 5", topK)
	}
}

// prevRecordingCap 记录 Previous map keys 的 mock capability
type prevRecordingCap struct {
	name    string
	version string
}

func (c *prevRecordingCap) Name() string                   { return c.name }
func (c *prevRecordingCap) Version() string                { return c.version }
func (c *prevRecordingCap) Init(ctx context.Context) error { return nil }
func (c *prevRecordingCap) Run(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	out := &TaskOutput{Approved: true, Data: make(map[string]any)}
	prevKeys := make([]string, 0, len(input.Previous))
	for k := range input.Previous {
		prevKeys = append(prevKeys, k)
	}
	sort.Strings(prevKeys)
	out.Data["prev"] = prevKeys
	return out, nil
}
func (c *prevRecordingCap) Health() error { return nil }
func (c *prevRecordingCap) Close() error  { return nil }

// configRecordingCap 记录 CapabilityConfig 的 mock capability
type configRecordingCap struct {
	name    string
	version string
}

func (c *configRecordingCap) Name() string                   { return c.name }
func (c *configRecordingCap) Version() string                { return c.version }
func (c *configRecordingCap) Init(ctx context.Context) error { return nil }
func (c *configRecordingCap) Run(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	out := &TaskOutput{Approved: true, Data: make(map[string]any)}
	if input.CapabilityConfig != nil {
		for k, v := range input.CapabilityConfig {
			out.Data[k] = v
		}
	}
	return out, nil
}
func (c *configRecordingCap) Health() error { return nil }
func (c *configRecordingCap) Close() error  { return nil }
