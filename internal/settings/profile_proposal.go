package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// This file holds the Approval-Hub machinery for relationship capsules
// (Persistent Identity P1-b, homologous "养熟" governance). A capsule
// update is never applied blindly: a candidate is written as a *pending
// proposal* file, and only an explicit operator approval promotes it to the
// active capsule. This mirrors the propose → Approval Hub → write flow
// without ever invoking an LLM inside the platform (reasoning about *what* to
// write is delegated to the operator or a CLI agent, per docs/decisions/irreversible-decisions.md §4.1).

func (r *ProfileRepository) proposalPath(key string) string {
	return filepath.Join(r.root, sanitizeKey(key)+proposalSuffix)
}

// WriteProposal writes a candidate capsule as a pending proposal. It does NOT
// touch the active capsule. The body is capped the same as an active capsule.
func (r *ProfileRepository) WriteProposal(key string, c *RelationshipCapsule) error {
	if key == "" {
		return fmt.Errorf("proposal requires a non-empty relationship_key")
	}
	if c == nil || strings.TrimSpace(c.Body) == "" {
		return fmt.Errorf("proposal requires a non-empty body")
	}
	if n := capsuleStrippedRuneCount(c.Body); n > MaxCapsuleBodyLen {
		return fmt.Errorf("%w: body is %d visible runes (after stripping whitespace), limit is %d", ErrCapsuleTooLong, n, MaxCapsuleBodyLen)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	proposal := &RelationshipCapsule{
		RelationshipKey: key,
		OwnerDog:        c.OwnerDog,
		Status:          "proposed",
		SourceRef:       c.SourceRef,
		CorrectionPath:  c.CorrectionPath,
		Body:            c.Body,
		UpdatedAt:       time.Now().UnixMilli(),
	}
	return writeAtomicRaw(r.proposalPath(key), renderCapsule(proposal), 0o644)
}

// ReadProposal loads the pending proposal for a key. The bool reports whether
// one exists; an error is returned only on I/O or parse failure.
func (r *ProfileRepository) ReadProposal(key string) (*RelationshipCapsule, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.readProposalLocked(key)
}

func (r *ProfileRepository) readProposalLocked(key string) (*RelationshipCapsule, bool, error) {
	raw, err := os.ReadFile(r.proposalPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read proposal %q: %w", key, err)
	}
	c, err := parseCapsule(key, raw)
	if err != nil {
		return nil, false, fmt.Errorf("parse proposal %q: %w", key, err)
	}
	return c, true, nil
}

// HasProposal reports whether a pending proposal exists for the key.
func (r *ProfileRepository) HasProposal(key string) (bool, error) {
	_, ok, err := r.ReadProposal(key)
	return ok, err
}

// DeleteProposal removes a pending proposal (idempotent, not an error if none).
func (r *ProfileRepository) DeleteProposal(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	err := os.Remove(r.proposalPath(key))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete proposal %q: %w", key, err)
	}
	return nil
}

// ApproveProposal promotes the pending proposal to the active capsule. The
// active capsule's body is replaced, source_ref / correction_path are
// carried over, eval_approvals is incremented, and the proposal file is
// removed. Returns the new active capsule.
func (r *ProfileRepository) ApproveProposal(key string) (*RelationshipCapsule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prop, ok, err := r.readProposalLocked(key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no pending proposal for %q", key)
	}
	active, _, _ := r.readCapsuleLocked(key)
	now := time.Now().UnixMilli()
	if active == nil {
		active = &RelationshipCapsule{RelationshipKey: key, Status: "active"}
	}
	active.Body = prop.Body
	if prop.OwnerDog != "" {
		active.OwnerDog = prop.OwnerDog
	}
	if prop.SourceRef != "" {
		active.SourceRef = prop.SourceRef
	} else {
		active.SourceRef = "approval-hub:proposed"
	}
	if prop.CorrectionPath != "" {
		active.CorrectionPath = strings.TrimSpace(active.CorrectionPath + "\n" + prop.CorrectionPath)
	}
	active.Status = "active"
	active.EvalApprovals++
	active.ProposedAt = prop.UpdatedAt
	active.ApprovedAt = now
	active.UpdatedAt = now
	if err := writeAtomicRaw(r.capsulePath(key), renderCapsule(active), 0o644); err != nil {
		return nil, err
	}
	_ = os.Remove(r.proposalPath(key))
	return active, nil
}

// RejectProposal discards the pending proposal and bumps eval_rejections on the
// active capsule (if one exists). Returns the (possibly nil) active capsule.
func (r *ProfileRepository) RejectProposal(key string) (*RelationshipCapsule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok, err := r.readProposalLocked(key); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("no pending proposal for %q", key)
	}
	active, _, _ := r.readCapsuleLocked(key)
	now := time.Now().UnixMilli()
	if active != nil {
		active.EvalRejections++
		active.RejectedAt = now
		active.UpdatedAt = now
		if err := writeAtomicRaw(r.capsulePath(key), renderCapsule(active), 0o644); err != nil {
			return nil, err
		}
	}
	if err := os.Remove(r.proposalPath(key)); err != nil && !os.IsNotExist(err) {
		return active, fmt.Errorf("delete proposal %q: %w", key, err)
	}
	return active, nil
}
