package dossier

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ErrProposalNotFound marks a missing proposal.
var ErrProposalNotFound = errors.New("dossier: proposal not found")

// ErrProposalState marks a CAS transition rejected by wrong status.
var ErrProposalState = errors.New("dossier: proposal not in expected status")

// sqliteProposalStore implements ProposalStore backed by SQLite. Proposals
// are durable team state (the approval ledger) — TTL=0, main database.
type sqliteProposalStore struct {
	db *sql.DB
}

// ProposalStore is the proposal port.
type ProposalStore interface {
	// Create stores a proposal. Idempotent on sourceId: if a proposal with
	// the same sourceId exists it is returned with created=false.
	Create(in CreateProposalInput) (DistillationProposal, bool, error)
	Get(id string) (DistillationProposal, error)
	GetBySourceID(sourceID string) (DistillationProposal, bool, error)
	ListPending(limit int) ([]DistillationProposal, error)
	ListByDog(dogID string, limit int) ([]DistillationProposal, error)
	MarkApproved(id, approvedBy string) (DistillationProposal, error)
	MarkRejected(id, rejectedBy, reason string) (DistillationProposal, error)
	MarkApplied(id, appliedBy, commitSHA string) (DistillationProposal, error)
}

// NewSQLiteProposalStore opens (creating if needed) the proposal schema.
func NewSQLiteProposalStore(path string) (ProposalStore, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir %q: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS dossier_distillation_proposals (
		proposal_id TEXT PRIMARY KEY,
		status TEXT NOT NULL,
		source_event TEXT NOT NULL,
		source_id TEXT NOT NULL,
		target_dog_id TEXT NOT NULL,
		target_fields TEXT NOT NULL,
		before_snapshot TEXT NOT NULL,
		after_draft TEXT NOT NULL,
		rationale TEXT NOT NULL,
		evidence_refs TEXT NOT NULL,
		base_hash TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		created_by TEXT NOT NULL,
		approved_by TEXT,
		approved_at INTEGER,
		rejected_by TEXT,
		rejected_at INTEGER,
		reject_reason TEXT,
		applied_by TEXT,
		applied_at INTEGER,
		applied_commit_sha TEXT
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_dossier_proposals_source ON dossier_distillation_proposals(source_id);
	CREATE INDEX IF NOT EXISTS idx_dossier_proposals_status ON dossier_distillation_proposals(status, created_at);
	CREATE INDEX IF NOT EXISTS idx_dossier_proposals_dog ON dossier_distillation_proposals(target_dog_id, created_at);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create dossier_distillation_proposals schema: %w", err)
	}
	return &sqliteProposalStore{db: db}, nil
}

// Create persists a validated proposal (validation happens in the service
// layer before this call).
func (s *sqliteProposalStore) Create(in CreateProposalInput) (DistillationProposal, bool, error) {
	if existing, ok, err := s.GetBySourceID(in.SourceID); err != nil {
		return DistillationProposal{}, false, err
	} else if ok {
		return existing, false, nil
	}

	fields, _ := json.Marshal(in.TargetFields)
	refs, err := json.Marshal(in.EvidenceRefs)
	if err != nil {
		return DistillationProposal{}, false, fmt.Errorf("marshal evidenceRefs: %w", err)
	}
	now := time.Now()
	p := DistillationProposal{
		ProposalID:     "dsp_" + now.Format("20060102150405") + "_" + fmt.Sprintf("%d", now.UnixNano()%1e9),
		Status:         ProposalPending,
		SourceEvent:    in.SourceEvent,
		SourceID:       in.SourceID,
		TargetDogID:    in.TargetDogID,
		TargetFields:   in.TargetFields,
		BeforeSnapshot: in.BeforeSnapshot,
		AfterDraft:     in.AfterDraft,
		Rationale:      in.Rationale,
		EvidenceRefs:   in.EvidenceRefs,
		BaseHash:       in.BaseHash,
		CreatedAt:      now,
		CreatedBy:      in.CreatedBy,
	}
	_, err = s.db.Exec(
		`INSERT INTO dossier_distillation_proposals
		 (proposal_id, status, source_event, source_id, target_dog_id, target_fields,
		  before_snapshot, after_draft, rationale, evidence_refs, base_hash,
		  created_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProposalID, string(p.Status), p.SourceEvent, p.SourceID, p.TargetDogID, string(fields),
		p.BeforeSnapshot, p.AfterDraft, p.Rationale, string(refs), p.BaseHash,
		p.CreatedAt.UnixMilli(), p.CreatedBy)
	if err != nil {
		return DistillationProposal{}, false, fmt.Errorf("insert proposal: %w", err)
	}
	return p, true, nil
}

// Get returns a proposal by id.
func (s *sqliteProposalStore) Get(id string) (DistillationProposal, error) {
	rows, err := s.db.Query(proposalSelect+` WHERE proposal_id = ?`, id)
	if err != nil {
		return DistillationProposal{}, fmt.Errorf("get proposal: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return DistillationProposal{}, ErrProposalNotFound
	}
	return scanProposal(rows)
}

// GetBySourceID returns the proposal for an idempotency key.
func (s *sqliteProposalStore) GetBySourceID(sourceID string) (DistillationProposal, bool, error) {
	rows, err := s.db.Query(proposalSelect+` WHERE source_id = ?`, sourceID)
	if err != nil {
		return DistillationProposal{}, false, fmt.Errorf("get proposal by source: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return DistillationProposal{}, false, rows.Err()
	}
	p, err := scanProposal(rows)
	return p, err == nil, err
}

// ListPending returns pending proposals, newest first.
func (s *sqliteProposalStore) ListPending(limit int) ([]DistillationProposal, error) {
	return s.listWhere(` WHERE status = ?`, ProposalPending, limit)
}

// ListByDog returns a dog's proposals (any status), newest first.
func (s *sqliteProposalStore) ListByDog(dogID string, limit int) ([]DistillationProposal, error) {
	return s.listWhere(` WHERE target_dog_id = ?`, dogID, limit)
}

func (s *sqliteProposalStore) listWhere(where string, arg any, limit int) ([]DistillationProposal, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(proposalSelect+where+` ORDER BY created_at DESC LIMIT ?`, arg, limit)
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	defer rows.Close()
	var out []DistillationProposal
	for rows.Next() {
		p, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MarkApproved CAS-transitions pending → approved. approvedBy must differ
// from createdBy (separation of duties) — enforced by the caller (service
// layer has the policy context), the store enforces the state machine.
func (s *sqliteProposalStore) MarkApproved(id, approvedBy string) (DistillationProposal, error) {
	return s.transition(id, ProposalPending, func(p *DistillationProposal) {
		p.Status = ProposalApproved
		p.ApprovedBy = approvedBy
		now := time.Now()
		p.ApprovedAt = &now
	})
}

// MarkRejected CAS-transitions pending → rejected.
func (s *sqliteProposalStore) MarkRejected(id, rejectedBy, reason string) (DistillationProposal, error) {
	return s.transition(id, ProposalPending, func(p *DistillationProposal) {
		p.Status = ProposalRejected
		p.RejectedBy = rejectedBy
		p.RejectReason = reason
		now := time.Now()
		p.RejectedAt = &now
	})
}

// MarkApplied CAS-transitions approved → applied.
func (s *sqliteProposalStore) MarkApplied(id, appliedBy, commitSHA string) (DistillationProposal, error) {
	return s.transition(id, ProposalApproved, func(p *DistillationProposal) {
		p.Status = ProposalApplied
		p.AppliedBy = appliedBy
		p.AppliedCommitSHA = commitSHA
		now := time.Now()
		p.AppliedAt = &now
	})
}

func (s *sqliteProposalStore) transition(id string, from ProposalStatus, mutate func(*DistillationProposal)) (DistillationProposal, error) {
	p, err := s.Get(id)
	if err != nil {
		return DistillationProposal{}, err
	}
	if p.Status != from {
		return DistillationProposal{}, fmt.Errorf("%w: proposal %s is %s, expected %s", ErrProposalState, id, p.Status, from)
	}
	mutate(&p)
	res, err := s.db.Exec(
		`UPDATE dossier_distillation_proposals
		 SET status=?, approved_by=?, approved_at=?, rejected_by=?, rejected_at=?, reject_reason=?, applied_by=?, applied_at=?, applied_commit_sha=?
		 WHERE proposal_id=? AND status=?`,
		string(p.Status), nullStr(p.ApprovedBy), nullTime(p.ApprovedAt), nullStr(p.RejectedBy),
		nullTime(p.RejectedAt), nullStr(p.RejectReason), nullStr(p.AppliedBy), nullTime(p.AppliedAt),
		nullStr(p.AppliedCommitSHA), id, string(from))
	if err != nil {
		return DistillationProposal{}, fmt.Errorf("update proposal: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return DistillationProposal{}, fmt.Errorf("%w: proposal %s raced", ErrProposalState, id)
	}
	return p, nil
}

const proposalSelect = `SELECT proposal_id, status, source_event, source_id, target_dog_id, target_fields,
	before_snapshot, after_draft, rationale, evidence_refs, base_hash,
	created_at, created_by, approved_by, approved_at, rejected_by, rejected_at, reject_reason,
	applied_by, applied_at, applied_commit_sha FROM dossier_distillation_proposals`

func scanProposal(rows interface{ Scan(dest ...any) error }) (DistillationProposal, error) {
	var p DistillationProposal
	var fieldsJSON, refsJSON string
	var status string
	var createdAt int64
	var approvedBy, rejectedBy, rejectReason, appliedBy, appliedCommit sql.NullString
	var approvedAt, rejectedAt, appliedAt sql.NullInt64
	if err := rows.Scan(
		&p.ProposalID, &status, &p.SourceEvent, &p.SourceID, &p.TargetDogID, &fieldsJSON,
		&p.BeforeSnapshot, &p.AfterDraft, &p.Rationale, &refsJSON, &p.BaseHash,
		&createdAt, &p.CreatedBy, &approvedBy, &approvedAt, &rejectedBy, &rejectedAt, &rejectReason,
		&appliedBy, &appliedAt, &appliedCommit,
	); err != nil {
		return DistillationProposal{}, fmt.Errorf("scan proposal: %w", err)
	}
	p.Status = ProposalStatus(status)
	p.CreatedAt = time.UnixMilli(createdAt)
	if err := json.Unmarshal([]byte(fieldsJSON), &p.TargetFields); err != nil {
		return DistillationProposal{}, fmt.Errorf("unmarshal targetFields: %w", err)
	}
	if err := json.Unmarshal([]byte(refsJSON), &p.EvidenceRefs); err != nil {
		return DistillationProposal{}, fmt.Errorf("unmarshal evidenceRefs: %w", err)
	}
	if approvedBy.Valid {
		p.ApprovedBy = approvedBy.String
	}
	if approvedAt.Valid {
		t := time.UnixMilli(approvedAt.Int64)
		p.ApprovedAt = &t
	}
	if rejectedBy.Valid {
		p.RejectedBy = rejectedBy.String
	}
	if rejectedAt.Valid {
		t := time.UnixMilli(rejectedAt.Int64)
		p.RejectedAt = &t
	}
	if rejectReason.Valid {
		p.RejectReason = rejectReason.String
	}
	if appliedBy.Valid {
		p.AppliedBy = appliedBy.String
	}
	if appliedAt.Valid {
		t := time.UnixMilli(appliedAt.Int64)
		p.AppliedAt = &t
	}
	if appliedCommit.Valid {
		p.AppliedCommitSHA = appliedCommit.String
	}
	return p, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UnixMilli()
}
