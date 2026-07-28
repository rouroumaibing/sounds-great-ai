package capability

import (
	"context"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestPathValidateNameVersion(t *testing.T) {
	p := NewPathValidate()
	if p.Name() != "path_validate" {
		t.Errorf("Name = %q, want %q", p.Name(), "path_validate")
	}
	if p.Version() != "v1" {
		t.Errorf("Version = %q, want %q", p.Version(), "v1")
	}
}

func TestPathValidateConfigFileNeedsApproval(t *testing.T) {
	p := NewPathValidate()
	tests := []struct {
		name string
		path string
	}{
		{"env", ".env"},
		{"go_mod", "go.mod"},
		{"go_sum", "go.sum"},
		{"yaml", "config.yaml"},
		{"config_json", "config.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &pack.TaskInput{Path: tt.path}
			out, err := p.Run(context.Background(), input)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if out.Approved {
				t.Errorf("writing %q should need approval, got Approved = true", tt.path)
			}
		})
	}
}

func TestPathValidateAllowsNormalPath(t *testing.T) {
	p := NewPathValidate()
	input := &pack.TaskInput{Path: "/workspace/src/main.go"}
	out, err := p.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !out.Approved {
		t.Errorf("normal path should be allowed, got Approved = false, Reason = %q", out.Reason)
	}
}

func TestPathValidateEmptyPath(t *testing.T) {
	p := NewPathValidate()
	input := &pack.TaskInput{Path: ""}
	out, err := p.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !out.Approved {
		t.Error("empty path should be allowed for write")
	}
}

func TestPathValidateInitHealthClose(t *testing.T) {
	p := NewPathValidate()
	if err := p.Init(context.Background()); err != nil {
		t.Errorf("Init: %v", err)
	}
	if err := p.Health(); err != nil {
		t.Errorf("Health: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
