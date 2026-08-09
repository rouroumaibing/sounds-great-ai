package ports

import "context"

// WorkflowState represents the current state of a SOP workflow.
type WorkflowState struct {
	FeatureID    string `json:"feature_id"`
	Stage        string `json:"stage"`
	BatonHolder  string `json:"baton_holder"`
	UpdatedAt    int64  `json:"updated_at"`
}

// GateResult represents the result of a gate check.
type GateResult struct {
	Passed  bool   `json:"passed"`
	Reason  string `json:"reason"`
	Details string `json:"details,omitempty"`
}

// IWorkflowStore is the port for workflow state storage.
type IWorkflowStore interface {
	Get(ctx context.Context, featureID string) (WorkflowState, error)
	Save(ctx context.Context, state WorkflowState) error
	List(ctx context.Context) ([]WorkflowState, error)
}

// ISOPGuardian is the port for SOP gate evaluation.
type ISOPGuardian interface {
	EvaluateGate(ctx context.Context, featureID string, gate string) (GateResult, error)
	CanAdvance(ctx context.Context, featureID string, fromStage, toStage string) (GateResult, error)
}

// IQualityGate is the port for quality gate evaluation.
type IQualityGate interface {
	Run(ctx context.Context, featureID string) (GateResult, error)
}
