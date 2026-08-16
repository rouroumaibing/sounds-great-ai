package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ContinuityDigest is a short, persistent note of what a breed was last doing
// (Persistent Identity P3, homologous F211 continuity bootstrap). In
// long cascade sessions the digest is re-injected when a cascade
// rotates personas mid-flight (F211 AC-A13), so the next leg does not
// cold-start. SG is one-shot (each task is a fresh process), so by default we
// persist a per-breed, per-rotation "last session" note and inject it on the
// next spawn. The store keeps a short ring of rotation checkpoints so that once
// a long (warm) session exists, each rotation can re-bootstrap from its own
// checkpoint — making continuity session-rotation-aware rather than only
// task-level.
type ContinuityDigest struct {
	BreedID       string `json:"breed_id"`
	RotationIndex int    `json:"rotation_index"`
	Summary       string `json:"summary"`
	ThreadID      string `json:"thread_id,omitempty"`
	UpdatedAt     int64  `json:"updated_at"`
}

// ContinuityCheckpoint is one rotation's persisted note.
type ContinuityCheckpoint struct {
	RotationIndex int    `json:"rotation_index"`
	Summary       string `json:"summary"`
	ThreadID      string `json:"thread_id,omitempty"`
	UpdatedAt     int64  `json:"updated_at"`
}

// continuityDoc is the on-disk envelope: a breed's ring of checkpoints.
type continuityDoc struct {
	BreedID    string                `json:"breed_id"`
	Checkpoints []ContinuityCheckpoint `json:"checkpoints"`
}

// maxCheckpoints bounds the ring so a chatty long session cannot grow the file
// without limit.
const maxCheckpoints = 8

// On-disk layout: <ConfigRoot>/continuity/<breedID>.json (atomic, per-breed).
const continuityDirName = "continuity"

// ContinuityStore persists the last-session digest per breed, keyed by rotation
// index. Safe for concurrent use (a single RWMutex guards directory
// reads/writes). Writes are atomic (tmp + rename) via writeAtomic.
type ContinuityStore struct {
	root string
	mu   sync.RWMutex
}

// NewContinuityStore creates (or opens) the continuity store under configRoot.
func NewContinuityStore(configRoot string) *ContinuityStore {
	root := filepath.Join(configRoot, continuityDirName)
	_ = os.MkdirAll(root, 0o755)
	return &ContinuityStore{root: root}
}

func (s *ContinuityStore) digestPath(breedID string) string {
	return filepath.Join(s.root, sanitizeKey(breedID)+".json")
}

// loadDoc reads and tolerates both the new (checkpoint ring) and legacy (single
// digest) on-disk formats, so files written before rotation support still load.
func (s *ContinuityStore) loadDoc(breedID string) (continuityDoc, error) {
	raw, err := os.ReadFile(s.digestPath(breedID))
	if err != nil {
		return continuityDoc{}, err
	}
	var doc continuityDoc
	if err := json.Unmarshal(raw, &doc); err == nil && len(doc.Checkpoints) > 0 {
		return doc, nil
	}
	// Legacy format: a single ContinuityDigest.
	var legacy ContinuityDigest
	if err := json.Unmarshal(raw, &legacy); err == nil && legacy.Summary != "" {
		return continuityDoc{
			BreedID: legacy.BreedID,
			Checkpoints: []ContinuityCheckpoint{{
				RotationIndex: legacy.RotationIndex,
				Summary:       legacy.Summary,
				ThreadID:      legacy.ThreadID,
				UpdatedAt:     legacy.UpdatedAt,
			}},
		}, nil
	}
	// Unknown/corrupt: treat as empty rather than hard-fail.
	return continuityDoc{}, nil
}

// RecordRotation writes (or upserts) the digest for a breed at a given rotation
// index. An empty breedID or summary is rejected. The checkpoint ring is capped
// at maxCheckpoints (oldest dropped).
func (s *ContinuityStore) RecordRotation(breedID, summary, threadID string, rotationIndex int) error {
	if breedID == "" {
		return fmt.Errorf("continuity: breedID required")
	}
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("continuity: summary required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadDoc(breedID)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("continuity: load %q: %w", breedID, err)
	}
	doc.BreedID = breedID
	cp := ContinuityCheckpoint{
		RotationIndex: rotationIndex,
		Summary:       strings.TrimSpace(summary),
		ThreadID:      threadID,
		UpdatedAt:     time.Now().UnixMilli(),
	}
	replaced := false
	for i := range doc.Checkpoints {
		if doc.Checkpoints[i].RotationIndex == rotationIndex {
			doc.Checkpoints[i] = cp
			replaced = true
			break
		}
	}
	if !replaced {
		doc.Checkpoints = append(doc.Checkpoints, cp)
	}
	sort.SliceStable(doc.Checkpoints, func(i, j int) bool {
		return doc.Checkpoints[i].RotationIndex < doc.Checkpoints[j].RotationIndex
	})
	if len(doc.Checkpoints) > maxCheckpoints {
		doc.Checkpoints = doc.Checkpoints[len(doc.Checkpoints)-maxCheckpoints:]
	}
	return writeAtomic(s.digestPath(breedID), doc, 0o644)
}

// Record is the one-shot backward-compatible entry point: it writes rotation 0
// (the degenerate single-session case). Long sessions should call
// RecordRotation with an incremented index per cascade rotation.
func (s *ContinuityStore) Record(breedID, summary, threadID string) error {
	return s.RecordRotation(breedID, summary, threadID, 0)
}

// RecordNextRotation is the rotation-aware spawn entry point. It writes the
// digest at (latest rotation index + 1), so each spawn becomes its own
// rotation and the ring fills with the last maxCheckpoints spawns. This makes
// continuity genuinely rotation-aware (F211 AC-A13: a continuity
// bootstrap fires on every rotation, re-injecting the prior rotation's digest)
// instead of the one-shot degenerate case where every spawn overwrites a
// single index-0 slot. Because the prompt builder reads the digest BEFORE this
// call (it runs at spawn start), the next spawn re-injects THIS spawn's summary
// as its "续接上下文" — exactly the bootstrap-on-rotation semantics.
//
// It returns the index it wrote so callers (e.g. a long-session carrier) can
// thread the rotation index forward. A missing breed doc starts the ring at 0.
func (s *ContinuityStore) RecordNextRotation(breedID, summary, threadID string) (int, error) {
	if breedID == "" {
		return 0, fmt.Errorf("continuity: breedID required")
	}
	if strings.TrimSpace(summary) == "" {
		return 0, fmt.Errorf("continuity: summary required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadDoc(breedID)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("continuity: load %q: %w", breedID, err)
	}
	doc.BreedID = breedID
	next := 0
	if cp, ok := doc.latest(); ok {
		next = cp.RotationIndex + 1
	}
	cp := ContinuityCheckpoint{
		RotationIndex: next,
		Summary:       strings.TrimSpace(summary),
		ThreadID:      threadID,
		UpdatedAt:     time.Now().UnixMilli(),
	}
	replaced := false
	for i := range doc.Checkpoints {
		if doc.Checkpoints[i].RotationIndex == next {
			doc.Checkpoints[i] = cp
			replaced = true
			break
		}
	}
	if !replaced {
		doc.Checkpoints = append(doc.Checkpoints, cp)
	}
	sort.SliceStable(doc.Checkpoints, func(i, j int) bool {
		return doc.Checkpoints[i].RotationIndex < doc.Checkpoints[j].RotationIndex
	})
	if len(doc.Checkpoints) > maxCheckpoints {
		doc.Checkpoints = doc.Checkpoints[len(doc.Checkpoints)-maxCheckpoints:]
	}
	if err := writeAtomic(s.digestPath(breedID), doc, 0o644); err != nil {
		return next, err
	}
	return next, nil
}

// latest returns the checkpoint with the highest rotation index.
func (doc continuityDoc) latest() (ContinuityCheckpoint, bool) {
	if len(doc.Checkpoints) == 0 {
		return ContinuityCheckpoint{}, false
	}
	return doc.Checkpoints[len(doc.Checkpoints)-1], true
}

// Last returns the persisted digest (latest rotation) for a breed. The bool
// reports whether one exists; an error is returned only on I/O or parse failure.
func (s *ContinuityStore) Last(breedID string) (ContinuityDigest, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, err := s.loadDoc(breedID)
	if err != nil {
		if os.IsNotExist(err) {
			return ContinuityDigest{}, false, nil
		}
		return ContinuityDigest{}, false, fmt.Errorf("continuity: read %q: %w", breedID, err)
	}
	cp, ok := doc.latest()
	if !ok {
		return ContinuityDigest{}, false, nil
	}
	return ContinuityDigest{
		BreedID:       breedID,
		RotationIndex: cp.RotationIndex,
		Summary:       cp.Summary,
		ThreadID:      cp.ThreadID,
		UpdatedAt:     cp.UpdatedAt,
	}, true, nil
}

// LastDigest returns just the persisted summary (latest rotation) for a breed
// (satisfies the prompt.ContinuityReader interface used by the prompt builder).
func (s *ContinuityStore) LastDigest(breedID string) (string, bool, error) {
	d, ok, err := s.Last(breedID)
	if err != nil || !ok {
		return "", ok, err
	}
	return d.Summary, true, nil
}

// LastDigestForRotation returns the persisted summary at a specific rotation
// index (session-rotation-aware re-injection). A missing rotation is not an
// error — it reports ok=false so the caller can fall back to the latest.
func (s *ContinuityStore) LastDigestForRotation(breedID string, rotationIndex int) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, err := s.loadDoc(breedID)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("continuity: read %q: %w", breedID, err)
	}
	for _, cp := range doc.Checkpoints {
		if cp.RotationIndex == rotationIndex {
			return cp.Summary, true, nil
		}
	}
	return "", false, nil
}

// GetDoc returns the full checkpoint ring for inspection (continuity API).
func (s *ContinuityStore) GetDoc(breedID string) (continuityDoc, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, err := s.loadDoc(breedID)
	if err != nil {
		if os.IsNotExist(err) {
			return continuityDoc{}, false, nil
		}
		return continuityDoc{}, false, fmt.Errorf("continuity: read %q: %w", breedID, err)
	}
	return doc, true, nil
}

// Clear removes a breed's continuity digest (e.g. when a task is explicitly
// closed). Clearing a non-existent digest is a no-op.
func (s *ContinuityStore) Clear(breedID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.digestPath(breedID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("continuity: clear %q: %w", breedID, err)
	}
	return nil
}

// List returns the breeds that currently have a continuity digest (sorted).
func (s *ContinuityStore) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("continuity: list: %w", err)
	}
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		keys = append(keys, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(keys)
	return keys, nil
}
