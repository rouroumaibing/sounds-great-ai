package eval

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/platform"
)

// EvalRunner coordinates eval domain execution.
type EvalRunner struct {
	domains  []EvalDomain
	platform *platform.Platform
	store    *ResultStore
	scorer   *Scorer
}

// NewEvalRunner creates an EvalRunner.
func NewEvalRunner(pl *platform.Platform, store *ResultStore, domains []EvalDomain) *EvalRunner {
	return &EvalRunner{
		domains:  domains,
		platform: pl,
		store:    store,
		scorer:   &Scorer{},
	}
}

// RunDomain executes one eval domain: build instructions → run eval breed → parse verdict → store.
func (r *EvalRunner) RunDomain(ctx context.Context, domain EvalDomain) (*VerdictHandoffPacket, error) {
	// 1. Build eval instructions (domain description + metric snapshot placeholder)
	instructions := fmt.Sprintf(
		"你是评估 breed。评估 domain: %s\n描述: %s\n"+
			"分析遥测趋势，产出 ```json-verdict``` 结构化 verdict。\n"+
			"verdict type: fix | build | keep_observe | delete_sunset\n"+
			"必须包含 evidence (snapshotRefs/metricRefs/traceRefs) 和 rootCause。",
		domain.DisplayName, domain.Description,
	)

	// 2. Get eval breed's CLI adapter (follows ws_handler.go pattern)
	breed := r.platform.GetBreed(domain.EvalBreed)
	if breed == nil {
		return nil, fmt.Errorf("eval breed %q not found", domain.EvalBreed)
	}
	variant := breed.DefaultVariant()
	if variant == nil {
		return nil, fmt.Errorf("eval breed %q has no variant configured", domain.EvalBreed)
	}
	adapter, err := r.platform.GetAdapter(variant.ClientID)
	if err != nil {
		return nil, fmt.Errorf("get adapter for breed %q: %w", domain.EvalBreed, err)
	}

	// 3. Execute eval breed
	req := unified.ExecuteRequest{
		Messages:     []*schema.Message{{Role: "user", Content: instructions}},
		SystemPrompt: variant.SystemPrompt,
		Model:        variant.DefaultModel,
		WorkDir:      r.platform.WorkspaceDir,
	}
	stream, err := adapter.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute eval breed: %w", err)
	}

	// 4. Collect output
	var output string
	for ev := range stream {
		if ev.Type == "text" {
			output += ev.Content
		}
	}

	// 5. Parse verdict
	verdict, err := r.scorer.ParseVerdict(output)
	if err != nil {
		return nil, fmt.Errorf("parse verdict: %w", err)
	}

	// 6. Store verdict + evidence bundle
	if err := r.store.SaveVerdict(verdict); err != nil {
		log.Printf("Warning: save verdict failed: %v", err)
	}
	return verdict, nil
}

// Domains returns the loaded eval domains.
func (r *EvalRunner) Domains() []EvalDomain {
	return r.domains
}
