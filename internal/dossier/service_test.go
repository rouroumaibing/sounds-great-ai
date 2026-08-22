package dossier

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupGitRepo creates a temp workspace with a git repo and the dossier
// file committed, returning the workspace and its initial hash.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.MkdirAll(filepath.Join(dir, "docs", "team"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, DossierRelativePath), []byte(testDossierContent()), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return dir
}

func newTestService(t *testing.T, workspace string) *Service {
	t.Helper()
	observations, _ := NewObservationStoreAt("")
	proposals, _ := NewProposalStoreAt("")
	loader := NewLoader()
	opportunities := NewInMemoryOpportunityStore()
	return NewService(proposals, observations, opportunities, NewCheckpoint(opportunities, nil), loader, workspace)
}

func TestServiceSeparationOfDuties(t *testing.T) {
	svc := newTestService(t, t.TempDir())

	in := validProposalInput("sod-1")
	in.CreatedBy = "xigou"
	p, _, err := svc.CreateProposal(in)
	if err != nil {
		t.Fatal(err)
	}

	// Self-approval refused.
	if _, err := svc.ApproveProposal(p.ProposalID, "xigou"); !errors.Is(err, ErrSeparationOfDuties) {
		t.Errorf("self-approve must be refused, got %v", err)
	}
	// Operator approves fine.
	if _, err := svc.ApproveProposal(p.ProposalID, "operator"); err != nil {
		t.Fatalf("operator approve: %v", err)
	}
	// Self-rejection refused on a fresh proposal.
	p2, _, err := svc.CreateProposal(validProposalInput("sod-2"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RejectProposal(p2.ProposalID, p2.CreatedBy, "no"); !errors.Is(err, ErrSeparationOfDuties) {
		t.Errorf("self-reject must be refused, got %v", err)
	}
}

func TestServiceExecuteApplyOnlyTargetDog(t *testing.T) {
	workspace := setupGitRepo(t)
	svc := newTestService(t, workspace)

	hash, err := svc.CurrentBaseHash()
	if err != nil {
		t.Fatal(err)
	}
	in := validProposalInput("apply-1")
	in.BaseHash = hash
	in.AfterDraft = "- **③ 坏直觉**：爱替队友做主（review 实证校准）"
	p, _, err := svc.CreateProposal(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveProposal(p.ProposalID, "operator"); err != nil {
		t.Fatal(err)
	}

	// Not the target dog.
	if _, err := svc.ExecuteApply(p.ProposalID, "jinmao"); !errors.Is(err, ErrNotTargetDog) {
		t.Errorf("non-target apply must be refused, got %v", err)
	}
	// Not yet approved → already approved here; apply before approve covered
	// by state tests. Target dog applies.
	result, err := svc.ExecuteApply(p.ProposalID, "bianmu")
	if err != nil {
		t.Fatalf("execute-apply: %v", err)
	}
	if result.CommitSHA == "" {
		t.Error("commit sha must be recorded")
	}

	content, err := os.ReadFile(filepath.Join(workspace, DossierRelativePath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "review 实证校准") {
		t.Error("dossier file must contain the applied draft")
	}

	// Commit landed with the structured message.
	logCmd := exec.Command("git", "log", "-1", "--format=%s")
	logCmd.Dir = workspace
	out, err := logCmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(out)), "docs(FT-DS-001): apply distillation to bianmu") {
		t.Errorf("unexpected commit subject: %q", out)
	}

	// Second apply must fail (already applied).
	if _, err := svc.ExecuteApply(p.ProposalID, "bianmu"); !errors.Is(err, ErrProposalState) {
		t.Errorf("double apply must fail, got %v", err)
	}
}

func TestServiceExecuteApplyStaleBaseHash(t *testing.T) {
	workspace := setupGitRepo(t)
	svc := newTestService(t, workspace)

	staleHash := ComputeFileHash(testDossierContent())
	in := validProposalInput("stale-1")
	in.BaseHash = staleHash
	p, _, err := svc.CreateProposal(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveProposal(p.ProposalID, "operator"); err != nil {
		t.Fatal(err)
	}

	// Mutate the dossier after proposal creation (simulating a parallel apply).
	dossierPath := filepath.Join(workspace, DossierRelativePath)
	if err := os.WriteFile(dossierPath, []byte(strings.Replace(testDossierContent(), "任务拆解", "任务拆解·新", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = svc.ExecuteApply(p.ProposalID, "bianmu")
	var applyErr *ApplyError
	if !errors.As(err, &applyErr) || applyErr.Code != ErrCodeBaseHashMismatch {
		t.Fatalf("expected BASE_HASH_MISMATCH, got %v", err)
	}
	// File must remain the mutated version (apply refused, no write).
	content, _ := os.ReadFile(dossierPath)
	if !strings.Contains(string(content), "任务拆解·新") {
		t.Error("stale apply must not touch the file")
	}
}
