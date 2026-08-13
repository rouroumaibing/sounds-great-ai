package services

import (
	"context"
	"path/filepath"
	"testing"

	custodyStores "sounds-great-ai/internal/domains/custody/stores"
)

// mockGitRunner is an injectable GitRunner for deterministic collector tests.
type mockGitRunner struct {
	heads map[string]string
	err   error
}

func (m mockGitRunner) LsRemote(_ context.Context, _ string) (map[string]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.heads, nil
}

func newTestCollector(t *testing.T, runner GitRunner) (*GitRefCollector, *custodyStores.RepoTrajectoryStore) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "repo-trajectory.json")
	store := custodyStores.NewRepoTrajectoryStore(path)
	return NewGitRefCollector(store, runner), store
}

// G8: empty repo_url is a no-op (disabled) and produces no events.
func TestGitRefCollector_EmptyRepoURL_ReturnsZero(t *testing.T) {
	c, store := newTestCollector(t, mockGitRunner{heads: map[string]string{"main": "abc"}})
	n, err := c.Collect(context.Background(), "")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 events for empty repo_url, got %d", n)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expected no events persisted, got %d", len(got))
	}
}

// G8: first collection marks every branch as branch_pushed.
func TestGitRefCollector_FirstCollect_PushesAllBranches(t *testing.T) {
	c, store := newTestCollector(t, mockGitRunner{heads: map[string]string{
		"main":    "sha-main",
		"feature": "sha-feat",
	}})
	n, err := c.Collect(context.Background(), "https://example.com/repo.git")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 pushed events, got %d", n)
	}
	evts := store.List()
	if len(evts) != 2 {
		t.Fatalf("expected 2 stored events, got %d", len(evts))
	}
	for _, e := range evts {
		if e.Kind != "branch_pushed" {
			t.Fatalf("expected branch_pushed, got %q for %s", e.Kind, e.Branch)
		}
		if e.HeadSHA == "" {
			t.Fatalf("expected non-empty head sha for %s", e.Branch)
		}
	}
}

// G8: re-collecting with unchanged SHAs is idempotent (replay-safe, INV-2).
func TestGitRefCollector_UnchangedBranches_Idempotent(t *testing.T) {
	c, store := newTestCollector(t, mockGitRunner{heads: map[string]string{
		"main":    "sha-main",
		"feature": "sha-feat",
	}})
	if _, err := c.Collect(context.Background(), "https://example.com/repo.git"); err != nil {
		t.Fatal(err)
	}
	// Second collect with identical SHAs -> no new events.
	n, err := c.Collect(context.Background(), "https://example.com/repo.git")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 new events on idempotent replay, got %d", n)
	}
	if got := store.List(); len(got) != 2 {
		t.Fatalf("expected store to keep 2 events, got %d", len(got))
	}
}

// G8: a changed head SHA on an existing branch is recorded as branch_updated,
// while unchanged branches stay silent.
func TestGitRefCollector_HeadChanged_BranchUpdated(t *testing.T) {
	c, store := newTestCollector(t, mockGitRunner{heads: map[string]string{
		"main":    "sha-main",
		"feature": "sha-feat",
	}})
	if _, err := c.Collect(context.Background(), "https://example.com/repo.git"); err != nil {
		t.Fatal(err)
	}
	// Change only "feature" head.
	c.runner = mockGitRunner{heads: map[string]string{
		"main":    "sha-main",
		"feature": "sha-feat-v2",
	}}
	n, err := c.Collect(context.Background(), "https://example.com/repo.git")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 updated event, got %d", n)
	}
	evts := store.List()
	if len(evts) != 3 {
		t.Fatalf("expected 3 stored events total, got %d", len(evts))
	}
	last := evts[len(evts)-1]
	if last.Kind != "branch_updated" || last.Branch != "feature" || last.HeadSHA != "sha-feat-v2" {
		t.Fatalf("unexpected last event: %+v", last)
	}
	// main remains branch_pushed from the first collect.
	for _, e := range evts {
		if e.Branch == "main" && e.Kind != "branch_pushed" {
			t.Fatalf("main should stay branch_pushed, got %q", e.Kind)
		}
	}
}

// G8: git IO failure is surfaced to the caller without corrupting the store.
func TestGitRefCollector_LsRemoteError_Propagated(t *testing.T) {
	c, store := newTestCollector(t, mockGitRunner{err: context.Canceled})
	_, err := c.Collect(context.Background(), "https://example.com/repo.git")
	if err == nil {
		t.Fatal("expected error from failed LsRemote")
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expected no events on failed collect, got %d", len(got))
	}
}

// G8: projection returns an oldest-first copy without mutating the store.
func TestProjectRepoTrajectory_ReturnsCopy(t *testing.T) {
	c, store := newTestCollector(t, mockGitRunner{heads: map[string]string{"main": "s1"}})
	if _, err := c.Collect(context.Background(), "https://example.com/repo.git"); err != nil {
		t.Fatal(err)
	}
	projected := ProjectRepoTrajectory(store.List())
	if len(projected) != 1 {
		t.Fatalf("expected 1 projected event, got %d", len(projected))
	}
	// Mutating the projection must not affect the store snapshot.
	projected[0].Kind = "mutated"
	if store.List()[0].Kind == "mutated" {
		t.Fatal("ProjectRepoTrajectory must return a defensive copy")
	}
}
