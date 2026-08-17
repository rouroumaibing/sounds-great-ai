package sop

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// QCState is the persisted state machine for the QC loop (clowder F253:
// qc.idle → … → qc.archived). SG's QCLoop is otherwise a stateless one-shot;
// this record adds stale-detection and idempotency so a re-run on a moved HEAD
// is recognised and the prior verdict is invalidated rather than reused.
//
// This closes the "stateless QC" gap identified in the SG↔clowder comparison:
// clowder tracks reviewedSha / idempotencyKey / staleFlag per change; SG now
// tracks the same three signals against a persisted file.
type QCState struct {
	Phase          string    `json:"phase"`
	ReviewedSha    string    `json:"reviewed_sha"`
	IdempotencyKey string    `json:"idempotency_key"`
	StaleFlag      bool      `json:"stale_flag"`
	LastRun        time.Time `json:"last_run"`
}

// DefaultQCStatePath returns the on-disk location for the QC state file when no
// ConfigRoot override is supplied. cmd/qc may override this with the same
// three-tier ConfigRoot resolution used for qc-metrics.jsonl.
func DefaultQCStatePath(workDir string) string {
	return filepath.Join(workDir, ".qc-state.json")
}

// LoadQCState reads the QC state file, returning a fresh idle state when the
// file is absent or unreadable (non-fatal — the loop still runs).
func LoadQCState(path string) QCState {
	st := QCState{Phase: "qc.idle"}
	b, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	_ = json.Unmarshal(b, &st)
	if st.Phase == "" {
		st.Phase = "qc.idle"
	}
	return st
}

// SaveQCState writes the QC state file atomically (tmp + rename).
func SaveQCState(path string, st QCState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// headSHA returns the current git HEAD sha for workDir, or "" if workDir is not
// a git repository (e.g. a temp dir in tests). A non-empty sha enables
// stale-detection and state persistence; an empty sha keeps the loop stateless
// and side-effect free.
func headSHA(workDir string) string {
	dir := workDir
	if dir == "" {
		dir = "."
	}
	out, err := exec.Command("git", "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ComputeStale reports whether a prior QC state is invalidated by a moved HEAD.
// It mirrors clowder's staleFlag: when the reviewed SHA no longer matches the
// current HEAD, the prior verdict must be re-derived rather than reused.
func ComputeStale(state QCState, sha string) bool {
	return sha != "" && state.ReviewedSha != "" && state.ReviewedSha != sha
}

// QCStatePath resolves the QC state file following the three-tier ConfigRoot
// resolution (env SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT -> <workdir>/.sounds-great-ai
// -> <home>/.sounds-great-ai). Shared by cmd/qc and the server auto-runner so
// both read/write the same persisted state.
func QCStatePath(workDir string) string {
	if v := os.Getenv("SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT"); v != "" {
		return filepath.Join(expandHome(v), "qc-state.json")
	}
	local := filepath.Join(workDir, ".sounds-great-ai", "qc-state.json")
	if _, err := os.Stat(filepath.Join(workDir, ".sounds-great-ai")); err == nil {
		return local
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".sounds-great-ai", "qc-state.json")
	}
	return local
}

// QCMetricsPath resolves the eval:qc metrics file with the same three-tier
// ConfigRoot resolution as QCStatePath.
func QCMetricsPath(workDir string) string {
	if v := os.Getenv("SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT"); v != "" {
		return filepath.Join(expandHome(v), "qc-metrics.jsonl")
	}
	local := filepath.Join(workDir, ".sounds-great-ai", "qc-metrics.jsonl")
	if _, err := os.Stat(filepath.Join(workDir, ".sounds-great-ai")); err == nil {
		return local
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".sounds-great-ai", "qc-metrics.jsonl")
	}
	return local
}

// expandHome expands a leading ~ or ~/ in a path to the user's home directory.
func expandHome(p string) string {
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
	}
	if len(p) > 2 && p[:2] == "~/" {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}
