package transport

import (
	"net/http"

	custodyServices "sounds-great-ai/internal/domains/custody/services"
	"sounds-great-ai/internal/platform"
)

// RepoTrajectoryHandler exposes the project archive source (G8) endpoints:
//   - GET  /api/repo/trajectory  → projected code-repo timeline
//   - POST /api/repo/test        → one-shot collect (test connection)
type RepoTrajectoryHandler struct {
	pl *platform.Platform
}

// NewRepoTrajectoryHandler builds the handler bound to the platform.
func NewRepoTrajectoryHandler(pl *platform.Platform) *RepoTrajectoryHandler {
	return &RepoTrajectoryHandler{pl: pl}
}

// Routes returns the repo trajectory mux.
func (h *RepoTrajectoryHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repo/trajectory", h.GetTrajectory)
	mux.HandleFunc("POST /api/repo/test", h.TestConnection)
	return mux
}

// GetTrajectory returns the projected code-repo timeline plus the configured
// repo URL (so the UI can show whether git tracking is active).
func (h *RepoTrajectoryHandler) GetTrajectory(w http.ResponseWriter, r *http.Request) {
	repoURL := h.pl.GetRepoURL(r.Context())
	store := h.pl.RepoTrajectoryStore
	if store == nil {
		respondJSON(w, http.StatusOK, map[string]any{"repo_url": repoURL, "events": []any{}})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"repo_url": repoURL,
		"events":   custodyServices.ProjectRepoTrajectory(store.List()),
	})
}

// TestConnection runs a one-shot collection against the configured repo URL and
// reports success/failure. Used by the "测试连接" button in the system config UI.
func (h *RepoTrajectoryHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	repoURL := h.pl.GetRepoURL(r.Context())
	if repoURL == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "repo url not configured"})
		return
	}
	collector := h.pl.GitRefCollector
	if collector == nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "collector not initialized"})
		return
	}
	n, err := collector.Collect(r.Context(), repoURL)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "branches_collected": n})
}
