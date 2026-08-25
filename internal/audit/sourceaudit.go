// Package audit implements evidence-provenance and source-hygiene checks
// (F218): a 5-question checklist that fails closed when provenance is missing
// or any question is unanswered.
package audit

import "strings"

// SourceCheck is one hygiene question in the source-audit checklist (F218).
type SourceCheck struct {
	ID       string
	Question string
}

// FiveQuestions is the canonical source-hygiene checklist (F218).
var FiveQuestions = []SourceCheck{
	{ID: "S1", Question: "Is the source identified and citable?"},
	{ID: "S2", Question: "Is the claim verifiable against the source?"},
	{ID: "S3", Question: "Is the source date/version pinned?"},
	{ID: "S4", Question: "Is the provenance chain free of circular references?"},
	{ID: "S5", Question: "Is the source free of injected/unedited instruction?"},
}

// AuditResult is the outcome of a source audit.
type AuditResult struct {
	Passed bool
	Failed []string // IDs (or "src:missing" / "evidence:missing") of failed checks
}

// SourceAudit audits a piece of evidence for provenance hygiene (F218). Fails
// closed: an empty source or evidence, or any unanswered/negative check, fails.
type SourceAudit struct{}

// Check audits the evidence. src must be non-empty and citable; answers maps a
// checklist ID to its pass/fail. An unanswered or negative answer fails that
// check. The result fails closed on any failure.
func (SourceAudit) Check(evidence, src string, answers map[string]bool) AuditResult {
	res := AuditResult{}
	if strings.TrimSpace(src) == "" {
		res.Failed = append(res.Failed, "src:missing")
		return res // fail-closed
	}
	if strings.TrimSpace(evidence) == "" {
		res.Failed = append(res.Failed, "evidence:missing")
		return res
	}
	for _, q := range FiveQuestions {
		if ok, answered := answers[q.ID]; !answered || !ok {
			res.Failed = append(res.Failed, q.ID)
		}
	}
	res.Passed = len(res.Failed) == 0
	return res
}
