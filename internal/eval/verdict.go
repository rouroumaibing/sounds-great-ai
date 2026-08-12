package eval

import (
	"fmt"
	"time"
)

// VerdictType is the judgment category.
type VerdictType string

const (
	VerdictFix          VerdictType = "fix"
	VerdictBuild        VerdictType = "build"
	VerdictKeepObserve  VerdictType = "keep_observe"
	VerdictDeleteSunset VerdictType = "delete_sunset"
)

// VerdictHandoffPacket is the structured output of an eval breed run.
type VerdictHandoffPacket struct {
	ID               string         `json:"id"`
	DomainID         string         `json:"domainId"`
	CreatedAt        time.Time      `json:"createdAt"`
	Phenomenon       string         `json:"phenomenon"`
	Verdict          VerdictType    `json:"verdict"`
	Evidence         EvidencePacket `json:"evidence"`
	RootCause        RootCause      `json:"rootCause"`
	OwnerAsk         OwnerAsk       `json:"ownerAsk"`
	Counterarguments []string       `json:"counterarguments"`
	AcceptanceReeval ReevalPlan     `json:"acceptanceReeval"`
}

type EvidencePacket struct {
	SnapshotRefs []string `json:"snapshotRefs"`
	MetricRefs   []string `json:"metricRefs"`
	TraceRefs    []string `json:"traceRefs"`
}

type RootCause struct {
	Summary      string   `json:"summary"`
	Confidence   string   `json:"confidence"` // low, medium, high
	Alternatives []string `json:"alternatives"`
}

type OwnerAsk struct {
	TargetFeatureID string `json:"targetFeatureId"`
	TargetOwner     string `json:"targetOwner"`
	RequestedAction string `json:"requestedAction"`
}

type ReevalPlan struct {
	NextEvalAt       time.Time `json:"nextEvalAt"`
	ClosureCondition string    `json:"closureCondition"`
}

var validVerdicts = map[VerdictType]bool{
	VerdictFix: true, VerdictBuild: true, VerdictKeepObserve: true, VerdictDeleteSunset: true,
}
var validConfidence = map[string]bool{"low": true, "medium": true, "high": true}

// ValidateVerdict checks that a verdict packet has required fields and valid enum values.
func ValidateVerdict(v *VerdictHandoffPacket) error {
	if v.ID == "" {
		return fmt.Errorf("verdict ID is empty")
	}
	if v.DomainID == "" {
		return fmt.Errorf("domainId is empty")
	}
	if !validVerdicts[v.Verdict] {
		return fmt.Errorf("invalid verdict type: %q", v.Verdict)
	}
	if len(v.Evidence.SnapshotRefs) == 0 && len(v.Evidence.MetricRefs) == 0 && len(v.Evidence.TraceRefs) == 0 {
		return fmt.Errorf("evidence is empty — at least one ref required")
	}
	if v.RootCause.Confidence != "" && !validConfidence[v.RootCause.Confidence] {
		return fmt.Errorf("invalid confidence: %q", v.RootCause.Confidence)
	}
	return nil
}
