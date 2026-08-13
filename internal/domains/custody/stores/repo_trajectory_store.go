package stores

import (
	"encoding/json"
	"os"
	"sync"
)

// RepoEvent is one append-only event in the code-repo trajectory timeline
// (project archive source, G8). SG has no GitHub PR API, so the only reliable,
// cheap signal from `git ls-remote` is branch presence + head SHA change.
type RepoEvent struct {
	Kind    string `json:"kind"`     // branch_pushed | branch_updated
	Branch  string `json:"branch"`   // branch name (without refs/heads/)
	HeadSHA string `json:"head_sha"` // head commit SHA at collection time
	At      int64  `json:"at"`       // unix seconds when collected
}

// RepoTrajectoryStore persists repo events to an append-only JSON file under
// the settings config root. Writes are atomic (tmp + rename), matching the
// other custody stores.
type RepoTrajectoryStore struct {
	mu   sync.Mutex
	path string
	evts []RepoEvent
}

// NewRepoTrajectoryStore creates (and loads) the repo trajectory store.
func NewRepoTrajectoryStore(path string) *RepoTrajectoryStore {
	s := &RepoTrajectoryStore{path: path}
	s.load()
	return s
}

func (s *RepoTrajectoryStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return // fresh store when file absent
	}
	var evts []RepoEvent
	if json.Unmarshal(data, &evts) == nil {
		s.evts = evts
	}
}

// Append adds a repo event and flushes atomically.
func (s *RepoTrajectoryStore) Append(e RepoEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evts = append(s.evts, e)
	return s.flush()
}

// List returns a snapshot of all repo events (oldest first).
func (s *RepoTrajectoryStore) List() []RepoEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RepoEvent, len(s.evts))
	copy(out, s.evts)
	return out
}

func (s *RepoTrajectoryStore) flush() error {
	data, err := json.MarshalIndent(s.evts, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
