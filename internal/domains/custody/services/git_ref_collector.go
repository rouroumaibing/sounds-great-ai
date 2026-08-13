package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"sounds-great-ai/internal/domains/custody/stores"
)

// GitRunner abstracts git IO so the collector is testable without a real repo.
type GitRunner interface {
	// LsRemote returns a map of branch name (without refs/heads/) -> head SHA,
	// by running `git ls-remote --heads <repoURL>`.
	LsRemote(ctx context.Context, repoURL string) (map[string]string, error)
}

// execGitRunner is the production GitRunner that shells out to `git`.
type execGitRunner struct{}

func (execGitRunner) LsRemote(ctx context.Context, repoURL string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", repoURL)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote failed: %w", err)
	}
	branches := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Output format: <sha>\trefs/heads/<branch>
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		sha, ref := parts[0], parts[1]
		const prefix = "refs/heads/"
		if !strings.HasPrefix(ref, prefix) {
			continue
		}
		branches[strings.TrimPrefix(ref, prefix)] = sha
	}
	return branches, nil
}

// GitRefCollector collects git branch events into the repo trajectory store.
// It is the SG homologue of clowder's GitRefSnapshotCollector, but limited to
// signals available without a GitHub API (branch presence + head SHA change).
type GitRefCollector struct {
	store  *stores.RepoTrajectoryStore
	runner GitRunner
}

// NewGitRefCollector builds a collector. A nil runner defaults to the
// exec-backed git runner.
func NewGitRefCollector(store *stores.RepoTrajectoryStore, runner GitRunner) *GitRefCollector {
	if runner == nil {
		runner = execGitRunner{}
	}
	return &GitRefCollector{store: store, runner: runner}
}

// Collect snapshots current branch heads and appends branch_pushed (new branch)
// or branch_updated (head SHA changed) events. Unchanged branches produce no
// event, so repeated collection is idempotent (INV-2 replay semantics).
func (c *GitRefCollector) Collect(ctx context.Context, repoURL string) (int, error) {
	if repoURL == "" {
		return 0, nil // disabled
	}
	heads, err := c.runner.LsRemote(ctx, repoURL)
	if err != nil {
		return 0, err
	}
	prev := lastHeadsByBranch(c.store.List())
	now := time.Now().Unix()
	count := 0
	for branch, sha := range heads {
		kind := "branch_pushed"
		if old, ok := prev[branch]; ok {
			if old == sha {
				continue // unchanged
			}
			kind = "branch_updated"
		}
		if err := c.store.Append(stores.RepoEvent{
			Kind:    kind,
			Branch:  branch,
			HeadSHA: sha,
			At:      now,
		}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func lastHeadsByBranch(evts []stores.RepoEvent) map[string]string {
	m := make(map[string]string, len(evts))
	for _, e := range evts {
		m[e.Branch] = e.HeadSHA
	}
	return m
}

// ProjectRepoTrajectory returns the projected code-repo timeline (oldest first).
// Kept as a pure function for symmetry with the custody projector and to allow
// future enrichment without changing the store.
func ProjectRepoTrajectory(evts []stores.RepoEvent) []stores.RepoEvent {
	out := make([]stores.RepoEvent, len(evts))
	copy(out, evts)
	return out
}
