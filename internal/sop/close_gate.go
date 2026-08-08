package sop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Resolution is the three-way close decision.
type Resolution string

const (
	ResolutionShip    Resolution = "ship"
	ResolutionIterate Resolution = "iterate"
	ResolutionSunset  Resolution = "sunset"
)

// AcceptanceCriterion is one AC with pass/fail status.
type AcceptanceCriterion struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Passed bool   `json:"passed"`
}

// CloseEvidence collects all evidence for the close report.
type CloseEvidence struct {
	Commits          []string           `json:"commits"`
	TestResults      []CheckResult      `json:"test_results"`
	ReviewProvenance []ReviewProvenance `json:"review_provenance"`
}

// CloseGateReport is the final report for a feature close.
type CloseGateReport struct {
	FeatureName    string               `json:"feature_name"`
	GeneratedAt    time.Time            `json:"generated_at"`
	ACMatrix       []AcceptanceCriterion `json:"ac_matrix"`
	Evidence       CloseEvidence        `json:"evidence"`
	QualityGate    QualityGateResult    `json:"quality_gate"`
	MergeGate      MergeGateResult      `json:"merge_gate"`
	Guardian       GuardianResult       `json:"guardian"`
	Resolution     Resolution           `json:"resolution"`
}

// CloseGate generates and persists close gate reports.
type CloseGate struct {
	reportsDir string
}

// NewCloseGate creates a CloseGate with the given reports directory.
func NewCloseGate(reportsDir string) *CloseGate {
	return &CloseGate{reportsDir: reportsDir}
}

// CloseGateInput provides all data needed to generate a close report.
type CloseGateInput struct {
	FeatureName    string
	ACMatrix       []AcceptanceCriterion
	Evidence       CloseEvidence
	QualityGate    QualityGateResult
	MergeGate      MergeGateResult
	Guardian       GuardianResult
}

// Generate creates a CloseGateReport from the input and determines the resolution.
func (c *CloseGate) Generate(input CloseGateInput) (*CloseGateReport, error) {
	report := &CloseGateReport{
		FeatureName: input.FeatureName,
		GeneratedAt: time.Now(),
		ACMatrix:    input.ACMatrix,
		Evidence:    input.Evidence,
		QualityGate: input.QualityGate,
		MergeGate:   input.MergeGate,
		Guardian:    input.Guardian,
		Resolution:  c.determineResolution(input),
	}
	return report, nil
}

// determineResolution decides ship/iterate/sunset based on results.
func (c *CloseGate) determineResolution(input CloseGateInput) Resolution {
	// Check if all ACs pass
	allACsPass := true
	for _, ac := range input.ACMatrix {
		if !ac.Passed {
			allACsPass = false
			break
		}
	}

	// If quality gate, merge gate, and guardian all pass → ship
	if input.QualityGate.Passed && input.MergeGate.Passed && input.Guardian.SignedOff && allACsPass {
		return ResolutionShip
	}

	// If guardian explicitly failed and ACs don't pass → sunset
	if !input.Guardian.SignedOff && !allACsPass {
		return ResolutionSunset
	}

	// Default: iterate
	return ResolutionIterate
}

// Persist writes the report to a JSON file.
func (c *CloseGate) Persist(report *CloseGateReport) error {
	dir := c.reportsDir
	if dir == "" {
		dir = filepath.Join("docs", "superpowers", "close-reports")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create reports dir: %w", err)
	}
	filename := fmt.Sprintf("%s.json", report.FeatureName)
	path := filepath.Join(dir, filename)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// Load reads a close report from file.
func (c *CloseGate) Load(featureName string) (*CloseGateReport, error) {
	dir := c.reportsDir
	if dir == "" {
		dir = filepath.Join("docs", "superpowers", "close-reports")
	}
	path := filepath.Join(dir, fmt.Sprintf("%s.json", featureName))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}
	var report CloseGateReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("unmarshal report: %w", err)
	}
	return &report, nil
}

// GenerateAndPersist is a convenience method that generates and saves.
func (c *CloseGate) GenerateAndPersist(input CloseGateInput) (*CloseGateReport, error) {
	report, err := c.Generate(input)
	if err != nil {
		return nil, err
	}
	if err := c.Persist(report); err != nil {
		return nil, err
	}
	return report, nil
}
