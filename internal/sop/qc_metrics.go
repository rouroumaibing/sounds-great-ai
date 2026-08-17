package sop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// QCMetricsRecord is one per-run emission of the QC loop. It is the runtime
// data source for the eval:qc domain (packs/default/evals/eval-qc.yaml).
//
// Honest parity note: clowder's F253 Phase C is also a pipeline skeleton with a
// zero-baseline bootstrap — its qcMetrics data source is documented as "not yet
// wired to production review telemetry events". SG's emitter is the same shape:
// it records what the loop actually observed (per-step pass/fail + reviewer
// delta when parseable) so the eval:qc domain has something to aggregate.
type QCMetricsRecord struct {
	Timestamp           string         `json:"timestamp"`
	WorkDir             string         `json:"work_dir"`
	Feature             string         `json:"feature,omitempty"`
	AuthorBreed         string         `json:"author_breed"`
	ReviewerBreed       string         `json:"reviewer_breed"`
	FinalApproverBreed  string         `json:"final_approver_breed,omitempty"`
	Passed              bool           `json:"passed"`
	Steps               []QCStepResult `json:"steps"`
	ReviewerDelta       *ReviewerDelta `json:"reviewer_delta,omitempty"`
}

// RecordQCMetrics appends a single QC metrics record as a JSONL line to the
// given path (creating parent dirs as needed). JSONL keeps the file append-only
// and cheap to aggregate later via the eval:qc domain.
func RecordQCMetrics(path string, rec QCMetricsRecord) error {
	if rec.Timestamp == "" {
		rec.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(rec)
}

// QCAggregate is the aggregated view of all recorded QC runs — the consumer
// side of the eval:qc domain (clowder F192 rolls up per-run qcMetrics into a
// control-plane summary). SG's emitter writes raw JSONL; this aggregates it so
// the domain has a non-zero-baseline report to act on.
type QCAggregate struct {
	TotalRuns       int               `json:"total_runs"`
	PassedRuns      int               `json:"passed_runs"`
	PassRate        float64           `json:"pass_rate"`
	AvgReviewerDelta float64          `json:"avg_reviewer_delta"`
	RunsByAuthorBreed map[string]int  `json:"runs_by_author_breed"`
}

// AggregateQCMetrics reads the per-run JSONL and folds it into a QCAggregate.
// Returns an error if the metrics file does not exist yet (no runs recorded).
func AggregateQCMetrics(path string) (*QCAggregate, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	agg := &QCAggregate{RunsByAuthorBreed: map[string]int{}}
	dec := json.NewDecoder(f)
	for dec.More() {
		var rec QCMetricsRecord
		if err := dec.Decode(&rec); err != nil {
			return nil, err
		}
		agg.TotalRuns++
		if rec.Passed {
			agg.PassedRuns++
		}
		if rec.ReviewerDelta != nil {
			agg.AvgReviewerDelta += rec.ReviewerDelta.Ratio
		}
		agg.RunsByAuthorBreed[rec.AuthorBreed]++
	}
	if agg.TotalRuns > 0 {
		agg.PassRate = float64(agg.PassedRuns) / float64(agg.TotalRuns)
		agg.AvgReviewerDelta /= float64(agg.TotalRuns)
	}
	return agg, nil
}
