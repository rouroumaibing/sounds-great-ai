package ports

import "context"

// EvalRun represents an evaluation run.
type EvalRun struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

// EvalResult represents the result of an evaluation.
type EvalResult struct {
	RunID   string                 `json:"run_id"`
	Score   float64                `json:"score"`
	Metrics map[string]float64     `json:"metrics"`
	Detail  map[string]any         `json:"detail,omitempty"`
}

// IResultStore is the port for eval result storage.
type IResultStore interface {
	Save(ctx context.Context, result EvalResult) error
	Get(ctx context.Context, runID string) (EvalResult, error)
	List(ctx context.Context, domain string) ([]EvalResult, error)
}

// IEvalStore is the port for eval run storage.
type IEvalStore interface {
	Create(ctx context.Context, run EvalRun) error
	Get(ctx context.Context, id string) (EvalRun, error)
	Update(ctx context.Context, run EvalRun) error
}
