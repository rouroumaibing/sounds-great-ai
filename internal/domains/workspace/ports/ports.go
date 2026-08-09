package ports

import (
	"context"
	"io"
)

// ExecRequest represents a workspace execution request.
type ExecRequest struct {
	Command string
	WorkDir string
	Stdin   io.Reader
}

// ExecResult represents the result of a workspace execution.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// IWorkspaceExecutor is the port for workspace command execution.
type IWorkspaceExecutor interface {
	Exec(ctx context.Context, req ExecRequest) (ExecResult, error)
}

// ISandboxManager is the port for sandbox management.
type ISandboxManager interface {
	Create(ctx context.Context, path string) (string, error)
	Destroy(ctx context.Context, sandboxID string) error
	List() []string
}
