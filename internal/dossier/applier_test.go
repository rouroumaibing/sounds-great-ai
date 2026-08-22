package dossier

import (
	"strings"
	"testing"
	"time"
)

func testDossierContent() string {
	return "# 档案\n\n" +
		"### 边牧 · @边牧 · `dog:bianmu`\n\n" +
		"- **① 原生峰值**：任务拆解\n- **③ 坏直觉**：爱替队友做主\n\n" +
		"```yaml\n# structured-profile: dog:bianmu\nentityId: \"bianmu\"\noneLiner: \"v1\"\n```\n\n" +
		"### 金毛 · @金毛 · `dog:jinmao`\n\n" +
		"- **① 原生峰值**：检索\n- **③ 坏直觉**：爱硬答\n\n" +
		"```yaml\n# structured-profile: dog:jinmao\nentityId: \"jinmao\"\noneLiner: \"v1\"\n```\n"
}

func approvedProposal(mutate func(*DistillationProposal)) DistillationProposal {
	p := DistillationProposal{
		ProposalID:     "dsp_test",
		Status:         ProposalApproved,
		SourceEvent:    SourceEventReviewComplete,
		SourceID:       "review-complete:t1:sha1:xigou",
		TargetDogID:    "bianmu",
		TargetFields:   []string{"badIntuitions"},
		BeforeSnapshot: "- **③ 坏直觉**：爱替队友做主",
		AfterDraft:     "- **③ 坏直觉**：爱替队友做主（2026-08 实证：连续两次 review 指出越界派单）",
		Rationale:      "两次跨犬 review 均指出越界",
		EvidenceRefs:   []EvidenceRef{{Type: "review", ID: "t1"}},
		BaseHash:       ComputeFileHash(testDossierContent()),
		CreatedAt:      time.Now(),
		CreatedBy:      "xigou",
		ApprovedBy:     "operator",
	}
	if mutate != nil {
		mutate(&p)
	}
	return p
}

func TestPrepareDraftHappyPath(t *testing.T) {
	result, err := PrepareDraft(approvedProposal(nil), testDossierContent())
	if err != nil {
		t.Fatalf("PrepareDraft: %v", err)
	}
	if !strings.Contains(result.ModifiedContent, "连续两次 review 指出越界派单") {
		t.Error("after draft not applied")
	}
	if strings.Contains(result.ModifiedContent, "- **③ 坏直觉**：爱替队友做主\n") {
		t.Error("before snapshot should be replaced")
	}
	// Other dog's section untouched.
	if !strings.Contains(result.ModifiedContent, "### 金毛") ||
		!strings.Contains(result.ModifiedContent, "- **③ 坏直觉**：爱硬答") {
		t.Error("jinmao section must remain intact")
	}
	if !strings.Contains(result.CommitMessage, "docs(FT-DS-001): apply distillation to bianmu") {
		t.Errorf("commit message malformed:\n%s", result.CommitMessage)
	}
}

func TestPrepareDraftNotApproved(t *testing.T) {
	_, err := PrepareDraft(approvedProposal(func(p *DistillationProposal) { p.Status = ProposalPending }), testDossierContent())
	var applyErr *ApplyError
	if !asApplyError(err, &applyErr) || applyErr.Code != ErrCodeNotApproved {
		t.Fatalf("expected NOT_APPROVED, got %v", err)
	}
}

func TestPrepareDraftBaseHashMismatch(t *testing.T) {
	changed := strings.Replace(testDossierContent(), "任务拆解", "任务拆解v2", 1)
	_, err := PrepareDraft(approvedProposal(nil), changed)
	var applyErr *ApplyError
	if !asApplyError(err, &applyErr) || applyErr.Code != ErrCodeBaseHashMismatch {
		t.Fatalf("expected BASE_HASH_MISMATCH, got %v", err)
	}
}

// The snapshot text exists — but only in ANOTHER dog's section. The anchored
// search must refuse rather than corrupt the wrong section.
func TestPrepareDraftSectionAnchoring(t *testing.T) {
	p := approvedProposal(func(p *DistillationProposal) {
		p.TargetDogID = "bianmu"
		// This text only exists in jinmao's section.
		p.BeforeSnapshot = "- **③ 坏直觉**：爱硬答"
	})
	_, err := PrepareDraft(p, testDossierContent())
	var applyErr *ApplyError
	if !asApplyError(err, &applyErr) || applyErr.Code != ErrCodeBeforeNotFound {
		t.Fatalf("expected BEFORE_SNAPSHOT_NOT_FOUND (anchored), got %v", err)
	}
}

func TestPrepareDraftUnknownSection(t *testing.T) {
	p := approvedProposal(func(p *DistillationProposal) { p.TargetDogID = "unknown-dog" })
	_, err := PrepareDraft(p, testDossierContent())
	var applyErr *ApplyError
	if !asApplyError(err, &applyErr) || applyErr.Code != ErrCodeSectionNotFound {
		t.Fatalf("expected SECTION_NOT_FOUND, got %v", err)
	}
}

func asApplyError(err error, target **ApplyError) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(*ApplyError)
	if ok {
		*target = ae
	}
	return ok
}
