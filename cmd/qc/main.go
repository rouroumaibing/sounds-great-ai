// Command qc runs the Sounds Great AI 7-step QC loop (internal/sop.QCLoop) as a
// developer-facing gate, mirroring clowder's `pnpm qc`. It is the runtime
// wiring for the QC loop (previously dead code) and the caller that exercises
// SelectReviewPanel to auto-pick the three-role cross-model review panel
// (Layer 2 reviewer + Layer 3 final approver) when not supplied explicitly.
//
// Usage:
//
//	go run ./cmd/qc --author <breed> [--reviewer <breed>] [--approver <breed>] \
//	                [--workdir .] [--feature <name>] [--template <path>] \
//	                [--fix] [--fix-commit]
//	go run ./cmd/qc report            # aggregate eval:qc telemetry
//
// Exit code 0 when the loop passes, 1 otherwise. A per-run metrics record is
// appended to <ConfigRoot>/qc-metrics.jsonl for the eval:qc domain to aggregate.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"sounds-great-ai/internal/sop"
	"sounds-great-ai/pkg/pack"
)

func main() {
	var (
		author    = flag.String("author", "", "author breed id (required)")
		reviewer  = flag.String("reviewer", "", "reviewer breed id (auto-picked if empty)")
		approver  = flag.String("approver", "", "final approver breed id (auto-picked if empty)")
		workDir   = flag.String("workdir", ".", "repository / working directory")
		feature   = flag.String("feature", "", "feature name for evidence manifest")
		template  = flag.String("template", "packs/default/breeds/dog-template.json", "breed template file")
		fix       = flag.Bool("fix", false, "auto-fix hygiene via gofmt -w (clowder F253 A1)")
		fixCommit = flag.Bool("fix-commit", false, "commit auto-fixes with [qc-bot] (implies --fix)")
	)
	flag.Parse()

	// `go run ./cmd/qc report` aggregates the eval:qc telemetry (control plane).
	if args := flag.Args(); len(args) > 0 && args[0] == "report" {
		runReport(*workDir)
		return
	}

	if *author == "" {
		fmt.Fprintln(os.Stderr, "error: --author is required")
		os.Exit(2)
	}
	if *fixCommit {
		*fix = true
	}

	// Load breed catalog to build candidate reviewers / approvers.
	p := pack.New("qc")
	tmplPath := *template
	if !filepath.IsAbs(tmplPath) {
		tmplPath = filepath.Join(*workDir, tmplPath)
	}
	candidates := []sop.BreedInfo{}
	if err := p.LoadFromFile(tmplPath, pack.LoadPolicySkipInvalid); err == nil {
		for _, b := range p.List() {
			candidates = append(candidates, sop.BreedInfo{
				ID:        b.ID,
				Roles:     b.Roles,
				Available: true,
			})
		}
	}

	policy := sop.ReviewPolicy{RequireDifferentBreed: true, ExcludeUnavailable: true}
	authorInfo := sop.BreedInfo{ID: *author, Available: true}

	// Auto-pick the three-role panel when not supplied (item: SelectReviewPanel caller).
	if *reviewer == "" || *approver == "" {
		if panel, err := sop.SelectReviewPanel(authorInfo, candidates, policy); err == nil && panel != nil {
			if *reviewer == "" && panel.Reviewer != nil {
				*reviewer = panel.Reviewer.ID
			}
			if *approver == "" && panel.FinalApprover != nil {
				*approver = panel.FinalApprover.ID
			}
		} else if *reviewer == "" || *approver == "" {
			fmt.Fprintf(os.Stderr, "warn: could not auto-pick review panel (%v); running with advisory review\n", err)
		}
	}

	loop := sop.NewQCLoop(*workDir)
	loop.StatePath = qcStatePath(*workDir)
	result := loop.Run(sop.QCLoopInput{
		WorkDir:             *workDir,
		AuthorBreed:         *author,
		ReviewerBreed:       *reviewer,
		FinalApproverBreed:  *approver,
		FeatureName:         *feature,
		Fix:                 *fix,
		FixCommit:           *fixCommit,
	})

	fmt.Println("QC Loop results:")
	for _, s := range result.Steps {
		status := "PASS"
		if !s.Passed {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %d. %-20s %s\n", status, s.Step, s.Name, s.Message)
	}

	fmt.Printf("\n  Risk tier : %s\n", result.RiskTier)
	if result.ReviewedSha != "" {
		short := result.ReviewedSha
		if len(short) > 8 {
			short = short[:8]
		}
		fmt.Printf("  HEAD      : %s\n", short)
		if result.Stale {
			fmt.Println("  Stale     : yes — HEAD moved since last QC; prior verdict invalidated")
		}
	}

	// Emit eval:qc runtime metrics (item: eval:qc emitter).
	rec := sop.QCMetricsRecord{
		WorkDir:            *workDir,
		Feature:            *feature,
		AuthorBreed:        *author,
		ReviewerBreed:      *reviewer,
		FinalApproverBreed: *approver,
		Passed:             result.Passed,
		Steps:              result.Steps,
	}
	if err := sop.RecordQCMetrics(qcMetricsPath(*workDir), rec); err != nil {
		fmt.Fprintf(os.Stderr, "warn: failed to record qc metrics: %v\n", err)
	}

	if !result.Passed {
		fmt.Println("\nQC loop FAILED — resolve failing steps before merge-gate.")
		os.Exit(1)
	}
	fmt.Println("\nQC loop PASSED.")
}

// runReport prints the aggregated eval:qc telemetry (the control-plane consumer
// that closes the "no aggregation" gap vs clowder's F192 rollup).
func runReport(workDir string) {
	agg, err := sop.AggregateQCMetrics(qcMetricsPath(workDir))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read qc metrics (%v)\n", err)
		os.Exit(1)
	}
	fmt.Println("eval:qc aggregate (control plane)")
	fmt.Printf("  total runs       : %d\n", agg.TotalRuns)
	fmt.Printf("  passed runs      : %d\n", agg.PassedRuns)
	fmt.Printf("  pass rate        : %.1f%%\n", agg.PassRate*100)
	fmt.Printf("  avg reviewer Δ   : %.2f\n", agg.AvgReviewerDelta)
	fmt.Println("  runs by author breed:")
	for b, n := range agg.RunsByAuthorBreed {
		fmt.Printf("    %-12s %d\n", b, n)
	}
}

// qcMetricsPath resolves the eval:qc metrics file path following the same
// three-tier ConfigRoot resolution as internal/settings.ConfigRoot:
// env SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT -> <workdir>/.sounds-great-ai ->
// <home>/.sounds-great-ai.
func qcMetricsPath(workDir string) string {
	if v := os.Getenv("SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT"); v != "" {
		return filepath.Join(expandHome(v), "qc-metrics.jsonl")
	}
	local := filepath.Join(workDir, ".sounds-great-ai", "qc-metrics.jsonl")
	if _, err := os.Stat(filepath.Join(workDir, ".sounds-great-ai")); err == nil {
		return local
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".sounds-great-ai", "qc-metrics.jsonl")
	}
	return local
}

// qcStatePath resolves the QC state file path with the same three-tier
// ConfigRoot resolution as qcMetricsPath.
func qcStatePath(workDir string) string {
	if v := os.Getenv("SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT"); v != "" {
		return filepath.Join(expandHome(v), "qc-state.json")
	}
	local := filepath.Join(workDir, ".sounds-great-ai", "qc-state.json")
	if _, err := os.Stat(filepath.Join(workDir, ".sounds-great-ai")); err == nil {
		return local
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".sounds-great-ai", "qc-state.json")
	}
	return local
}

func expandHome(p string) string {
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
	}
	if len(p) > 2 && p[:2] == "~/" {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}
