package ports

import "context"

// HealthStatus represents the health of the system.
type HealthStatus struct {
	Status    string            `json:"status"`
	Uptime    int64             `json:"uptime"`
	Checks    map[string]string `json:"checks"`
}

// Diagnostics represents system diagnostics.
type Diagnostics struct {
	Pool    map[string]any `json:"pool"`
	Memory  map[string]any `json:"memory"`
	Goroutines int         `json:"goroutines"`
}

// GitStatus represents git repository status.
type GitStatus struct {
	Branch  string `json:"branch"`
	Clean   bool   `json:"clean"`
	Ahead   int    `json:"ahead"`
	Behind  int    `json:"behind"`
}

// IHealthService is the port for health checks.
type IHealthService interface {
	Health(ctx context.Context) (HealthStatus, error)
	Ready(ctx context.Context) (HealthStatus, error)
}

// IDiagnosticsService is the port for diagnostics.
type IDiagnosticsService interface {
	Diagnostics(ctx context.Context) (Diagnostics, error)
}

// IGitService is the port for git operations.
type IGitService interface {
	Status(ctx context.Context) (GitStatus, error)
}
