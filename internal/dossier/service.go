package dossier

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Service orchestrates the dossier domain: observation staging, checkpoint
// opportunities, and the distillation proposal pipeline. The handler layer
// stays thin; policy decisions (validation, separation of duties, apply
// orchestration) live here.
type Service struct {
	Proposals     ProposalStore
	Observations  ObservationStore
	Opportunities OpportunityStore
	Checkpoint    *Checkpoint
	Loader        *Loader
	// WorkspaceDir is the repo root holding docs/team/dog-dossier.md.
	WorkspaceDir string
}

// NewService assembles the domain service.
func NewService(proposals ProposalStore, observations ObservationStore, opportunities OpportunityStore, checkpoint *Checkpoint, loader *Loader, workspaceDir string) *Service {
	return &Service{
		Proposals:     proposals,
		Observations:  observations,
		Opportunities: opportunities,
		Checkpoint:    checkpoint,
		Loader:        loader,
		WorkspaceDir:  workspaceDir,
	}
}

// ErrSeparationOfDuties marks self-approval / self-rejection attempts.
var ErrSeparationOfDuties = errors.New("dossier: cannot approve or reject your own proposal")

// ErrNotTargetDog marks apply attempts by anyone but the target dog.
var ErrNotTargetDog = errors.New("dossier: only the target dog can apply a proposal")

// CreateProposal validates and persists a proposal. Idempotent on sourceId.
func (s *Service) CreateProposal(in CreateProposalInput) (DistillationProposal, bool, error) {
	if in.CreatedBy == "" {
		in.CreatedBy = "operator"
	}
	if err := ValidateCreateProposal(in); err != nil {
		return DistillationProposal{}, false, err
	}
	return s.Proposals.Create(in)
}

// ApproveProposal transitions pending → approved, refusing self-approval.
func (s *Service) ApproveProposal(id, approver string) (DistillationProposal, error) {
	if approver == "" {
		approver = "operator"
	}
	p, err := s.Proposals.Get(id)
	if err != nil {
		return DistillationProposal{}, err
	}
	if p.CreatedBy == approver {
		return DistillationProposal{}, ErrSeparationOfDuties
	}
	return s.Proposals.MarkApproved(id, approver)
}

// RejectProposal transitions pending → rejected, refusing self-rejection.
func (s *Service) RejectProposal(id, rejecter, reason string) (DistillationProposal, error) {
	if rejecter == "" {
		rejecter = "operator"
	}
	p, err := s.Proposals.Get(id)
	if err != nil {
		return DistillationProposal{}, err
	}
	if p.CreatedBy == rejecter {
		return DistillationProposal{}, ErrSeparationOfDuties
	}
	return s.Proposals.MarkRejected(id, rejecter, reason)
}

// ApplyResult reports the apply outcome, including partial success (commit
// landed but push/marking raced).
type ApplyResult struct {
	Proposal  DistillationProposal
	CommitSHA string
}

// ExecuteApply runs the full apply pipeline for an approved proposal:
// validate baseHash → write file → git add+commit (no push) → mark applied →
// invalidate the loader cache. Only the target dog may apply.
//
// Two-phase git safety: if commit fails, the file is restored and the change
// unstaged so a retry doesn't hit BASE_HASH_MISMATCH against a dirty tree.
func (s *Service) ExecuteApply(proposalID, appliedBy string) (ApplyResult, error) {
	if appliedBy == "" {
		appliedBy = "operator"
	}
	p, err := s.Proposals.Get(proposalID)
	if err != nil {
		return ApplyResult{}, err
	}
	if p.TargetDogID != appliedBy {
		return ApplyResult{}, ErrNotTargetDog
	}
	if p.Status != ProposalApproved {
		return ApplyResult{}, fmt.Errorf("%w: proposal is %s, expected approved", ErrProposalState, p.Status)
	}

	dossierPath := filepath.Join(s.WorkspaceDir, DossierRelativePath)
	original, err := os.ReadFile(dossierPath)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("read dossier: %w", err)
	}

	draft, err := PrepareDraft(p, string(original))
	if err != nil {
		return ApplyResult{}, err
	}

	if err := os.WriteFile(dossierPath, []byte(draft.ModifiedContent), 0o644); err != nil {
		return ApplyResult{}, fmt.Errorf("write dossier: %w", err)
	}

	commitSHA, err := s.gitCommit(draft.CommitMessage)
	if err != nil {
		// Rollback: restore content and unstage so retry starts clean.
		_ = os.WriteFile(dossierPath, original, 0o644)
		_, _ = s.git("reset", "HEAD", "--", DossierRelativePath)
		return ApplyResult{}, fmt.Errorf("git commit failed (file rolled back): %w", err)
	}

	applied, err := s.Proposals.MarkApplied(proposalID, appliedBy, commitSHA)
	if err != nil {
		// Commit landed but the store raced — report the committed state;
		// the ledger may be stale but the file history is authoritative.
		applied = p
		applied.Status = ProposalApplied
		applied.AppliedBy = appliedBy
		applied.AppliedCommitSHA = commitSHA
		now := time.Now()
		applied.AppliedAt = &now
	}

	s.Loader.Invalidate(s.WorkspaceDir)
	return ApplyResult{Proposal: applied, CommitSHA: commitSHA}, nil
}

func (s *Service) gitCommit(message string) (string, error) {
	if _, err := s.git("add", DossierRelativePath); err != nil {
		return "", err
	}
	if _, err := s.git("commit", "-m", message); err != nil {
		return "", err
	}
	out, err := s.git("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *Service) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.WorkspaceDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// CurrentBaseHash returns the dossier file hash for proposal creators.
func (s *Service) CurrentBaseHash() (string, error) {
	content, err := os.ReadFile(filepath.Join(s.WorkspaceDir, DossierRelativePath))
	if err != nil {
		return "", fmt.Errorf("read dossier: %w", err)
	}
	return ComputeFileHash(string(content)), nil
}
