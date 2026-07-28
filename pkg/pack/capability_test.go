package pack

import (
	"context"
	"testing"
)

func TestTaskInputWithPrevious(t *testing.T) {
	prevOutput := &TaskOutput{Approved: true, Reason: "ok"}
	input := TaskInput{
		Query:   "test query",
		Command: "ls",
		Path:    "/workspace",
		Context: &ExecutionContext{
			UserID:    "user-1",
			SessionID: "session-1",
			Workspace: "/workspace",
			TraceID:   "trace-1",
		},
		Previous: map[string]*TaskOutput{
			"step1": prevOutput,
		},
		CapabilityConfig: map[string]any{"top_k": 5},
	}

	if input.Query != "test query" {
		t.Errorf("Query = %q", input.Query)
	}
	if input.Context.UserID != "user-1" {
		t.Errorf("UserID = %q", input.Context.UserID)
	}
	if input.Previous["step1"] != prevOutput {
		t.Error("Previous[step1] not preserved")
	}
	if input.Previous["step1"].Approved != true {
		t.Error("Previous[step1].Approved should be true")
	}
	if input.CapabilityConfig["top_k"] != 5 {
		t.Errorf("CapabilityConfig top_k = %v", input.CapabilityConfig["top_k"])
	}
}

func TestTaskOutputDefaults(t *testing.T) {
	out := TaskOutput{}
	if out.Approved != false {
		t.Error("Approved should default to false")
	}
	if out.Reason != "" {
		t.Error("Reason should default to empty")
	}
	if out.Results != nil {
		t.Error("Results should default to nil")
	}
	if out.Data != nil {
		t.Error("Data should default to nil")
	}
}

func TestExecutionContextJSONTags(t *testing.T) {
	ctx := ExecutionContext{
		UserID:      "u1",
		SessionID:   "s1",
		Workspace:   "/ws",
		TraceID:     "t1",
		Permissions: []string{"read", "write"},
		Metadata:    map[string]string{"key": "value"},
	}
	if ctx.UserID != "u1" {
		t.Errorf("UserID = %q", ctx.UserID)
	}
	if len(ctx.Permissions) != 2 {
		t.Errorf("Permissions len = %d", len(ctx.Permissions))
	}
	if ctx.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %q", ctx.Metadata["key"])
	}
}

// mockCapability 用于测试的 mock Capability 实现
type mockCapability struct {
	name    string
	version string
}

func (m *mockCapability) Name() string                   { return m.name }
func (m *mockCapability) Version() string                { return m.version }
func (m *mockCapability) Init(ctx context.Context) error { return nil }
func (m *mockCapability) Run(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	return &TaskOutput{Approved: true}, nil
}
func (m *mockCapability) Health() error { return nil }
func (m *mockCapability) Close() error  { return nil }

func TestMockCapabilityImplementsInterface(t *testing.T) {
	var cap Capability = &mockCapability{name: "test", version: "v1"}
	if cap.Name() != "test" {
		t.Errorf("Name = %q", cap.Name())
	}
	if cap.Version() != "v1" {
		t.Errorf("Version = %q", cap.Version())
	}
	if err := cap.Init(context.Background()); err != nil {
		t.Errorf("Init: %v", err)
	}
	out, err := cap.Run(context.Background(), &TaskInput{})
	if err != nil {
		t.Errorf("Run: %v", err)
	}
	if !out.Approved {
		t.Error("Approved should be true")
	}
}
