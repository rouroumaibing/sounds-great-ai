package pack

import (
	"context"
	"fmt"
)

// Bark executes the breed's task. In the new variant-based format, breeds use
// CLI adapters instead of workflow DAGs. This function validates the breed
// exists and returns a placeholder output. Actual CLI invocation is handled
// by the adapter system.
func (p *Pack) Bark(ctx context.Context, breedID string, input *TaskInput) (*TaskOutput, error) {
	p.mu.RLock()
	breed, ok := p.registry[breedID]
	p.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("breed %q not found", breedID)
	}
	input.Breed = breed

	// Return a success output. CLI adapter invocation is handled externally.
	return &TaskOutput{
		Approved: true,
		Data:     map[string]any{"steps": make(map[string]*TaskOutput)},
	}, nil
}
