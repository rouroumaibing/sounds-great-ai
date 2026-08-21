package transport

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"sounds-great-ai/internal/settings"

	"sounds-great-ai/pkg/pack"
)

// ConfigHandler handles advanced config management HTTP endpoints.
type ConfigHandler struct {
	breedLoader   *pack.Loader
	breedsDir     string
	settingsStore settings.SettingsStore
	envPath       string
	leader        *pack.Leader
}

func NewConfigHandler(loader *pack.Loader, breedsDir string, store settings.SettingsStore, envPath string) *ConfigHandler {
	return &ConfigHandler{breedLoader: loader, breedsDir: breedsDir, settingsStore: store, envPath: envPath}
}

// SetLeader wires the platform's LeaderConfig pointer for GET/PATCH /api/config/leader.
func (h *ConfigHandler) SetLeader(l *pack.Leader) {
	h.leader = l
}

func (h *ConfigHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config/default-breed", h.GetDefaultBreed)
	mux.HandleFunc("PUT /api/config/default-breed", h.SetDefaultBreed)
	mux.HandleFunc("GET /api/config/breed-order", h.GetBreedOrder)
	mux.HandleFunc("PUT /api/config/breed-order", h.SetBreedOrder)
	mux.HandleFunc("GET /api/config/repo", h.GetRepo)
	mux.HandleFunc("PUT /api/config/repo", h.SetRepo)
	mux.HandleFunc("GET /api/config/env-summary", h.GetEnvSummary)
	mux.HandleFunc("PATCH /api/config/env", h.UpdateEnv)
	mux.HandleFunc("GET /api/config/leader", h.GetLeader)
	mux.HandleFunc("PATCH /api/config/leader", h.UpdateLeader)
	return mux
}

func (h *ConfigHandler) GetDefaultBreed(w http.ResponseWriter, r *http.Request) {
	breedID := os.Getenv("DEFAULT_BREED_ID")
	isOverride := breedID != ""
	if !isOverride {
		configs, _ := h.settingsStore.ListConfig()
		for _, c := range configs {
			if c.Key == "default_breed" {
				breedID = c.Value
				break
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{"breed_id": breedID, "is_override": isOverride})
}

// repoConfigKey is the SystemConfig key holding the optional code-repo URL that
// powers the project archive source (G8). Empty = git-ref tracking disabled.
const repoConfigKey = "repo_url"

// GetRepo returns the configured code-repo URL (empty string when unset).
func (h *ConfigHandler) GetRepo(w http.ResponseWriter, r *http.Request) {
	repoURL := ""
	if configs, err := h.settingsStore.ListConfig(); err == nil {
		for _, c := range configs {
			if c.Key == repoConfigKey {
				repoURL = c.Value
				break
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{"repo_url": repoURL})
}

// SetRepo upserts the code-repo URL. An empty string clears/disables it.
func (h *ConfigHandler) SetRepo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RepoURL string `json:"repo_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if body.RepoURL != "" && !isValidRepoURL(body.RepoURL) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid repo url (must be http(s)://, git@, or an absolute path)",
		})
		return
	}
	if err := h.settingsStore.UpsertConfig(repoConfigKey, body.RepoURL); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"repo_url": body.RepoURL})
}

// isValidRepoURL accepts http(s)://, git@ (scp-like) remotes, or absolute paths.
func isValidRepoURL(u string) bool {
	return strings.HasPrefix(u, "http://") ||
		strings.HasPrefix(u, "https://") ||
		strings.HasPrefix(u, "git@") ||
		strings.HasPrefix(u, "/")
}

func (h *ConfigHandler) SetDefaultBreed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BreedID string `json:"breed_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if body.BreedID != "" {
		known, _ := h.knownBreedIDs()
		if !known[body.BreedID] {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "breed not found"})
			return
		}
	}
	// Persist to the settings store. A deploy-time DEFAULT_BREED_ID env override
	// still wins on read (see GetDefaultBreed).
	if err := h.settingsStore.UpdateConfig("default_breed", body.BreedID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"breed_id": body.BreedID})
}

// knownBreedIDs returns the set of valid breed IDs as the union of the template
// seeds and the runtime catalog (merged registry). New
// members created at runtime must be selectable as the default breed, so the
// catalog breeds are included in the validation set.
func (h *ConfigHandler) knownBreedIDs() (map[string]bool, error) {
	known := make(map[string]bool)
	if tmpl, err := h.breedLoader.LoadFromFile(filepath.Join(h.breedsDir, "dog-template.json")); err == nil {
		for id := range tmpl {
			known[id] = true
		}
	}
	if catalog, err := h.settingsStore.ListBreeds(); err == nil {
		for _, b := range catalog {
			if b != nil && b.ID != "" {
				known[b.ID] = true
			}
		}
	}
	return known, nil
}

func (h *ConfigHandler) GetBreedOrder(w http.ResponseWriter, r *http.Request) {
	// The catalog breeds[] order is the sort truth.
	breeds, _ := h.settingsStore.ListBreeds()
	if len(breeds) > 0 {
		order := make([]string, 0, len(breeds))
		for _, b := range breeds {
			if b != nil && b.ID != "" {
				order = append(order, b.ID)
			}
		}
		respondJSON(w, http.StatusOK, map[string][]string{"order": order})
		return
	}
	// Fallback to template order before any catalog exists.
	tmpl, _ := h.breedLoader.LoadFromFile(filepath.Join(h.breedsDir, "dog-template.json"))
	order := make([]string, 0, len(tmpl))
	for id := range tmpl {
		order = append(order, id)
	}
	respondJSON(w, http.StatusOK, map[string][]string{"order": order})
}

func (h *ConfigHandler) SetBreedOrder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Order []string `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	known, _ := h.knownBreedIDs()
	var missing []string
	for _, id := range body.Order {
		if !known[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown breed IDs", "missing": missing})
		return
	}
	// Reorder the persisted catalog breeds[] (sort truth).
	if err := h.settingsStore.ReorderBreeds(body.Order); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string][]string{"order": body.Order})
}

func (h *ConfigHandler) GetEnvSummary(w http.ResponseWriter, r *http.Request) {
	categories := []map[string]any{
		{"name": "model", "variables": []map[string]any{
			{"key": "MODEL_TYPE", "value": os.Getenv("MODEL_TYPE"), "sensitive": false},
			{"key": "MODEL_API_KEY", "value": maskEnvValue(os.Getenv("MODEL_API_KEY")), "sensitive": true},
			{"key": "MODEL_BASE_URL", "value": os.Getenv("MODEL_BASE_URL"), "sensitive": false},
			{"key": "MODEL_NAME", "value": os.Getenv("MODEL_NAME"), "sensitive": false},
		}},
		{"name": "runtime", "variables": []map[string]any{
			{"key": "MAX_CONCURRENT_BARKS", "value": os.Getenv("MAX_CONCURRENT_BARKS"), "sensitive": false},
		}},
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
		"data_dirs": map[string]string{
			"breeds": h.breedsDir,
			"env":    h.envPath,
		},
	})
}

func (h *ConfigHandler) UpdateEnv(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Updates []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"updates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	for _, u := range body.Updates {
		if isSensitiveEnv(u.Key) && !isLoopback(r) {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "sensitive env vars require loopback access"})
			return
		}
	}

	envMap := readEnvFile(h.envPath)
	for _, u := range body.Updates {
		envMap[u.Key] = u.Value
		os.Setenv(u.Key, u.Value)
	}
	writeEnvFile(h.envPath, envMap)

	keys := make([]string, 0, len(body.Updates))
	for _, u := range body.Updates {
		keys = append(keys, u.Key)
	}
	respondJSON(w, http.StatusOK, map[string][]string{"updated": keys})
}

func (h *ConfigHandler) GetLeader(w http.ResponseWriter, r *http.Request) {
	// Persisted leader lives in dog-catalog.json (via the settings store).
	l, err := h.settingsStore.GetLeader()
	if err != nil || l == nil {
		defaultCfg := pack.DefaultLeaderConfig()
		respondJSON(w, http.StatusOK, defaultCfg)
		return
	}
	respondJSON(w, http.StatusOK, l)
}

func (h *ConfigHandler) UpdateLeader(w http.ResponseWriter, r *http.Request) {
	var body pack.Leader
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := body.Validate(); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// Persist to dog-catalog.json.
	if err := h.settingsStore.UpdateLeader(&body); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Keep the platform's in-memory leader in sync for runtime use.
	if h.leader != nil {
		*h.leader = body
	}
	respondJSON(w, http.StatusOK, &body)
}

func isSensitiveEnv(key string) bool {
	upper := strings.ToUpper(key)
	return strings.Contains(upper, "KEY") || strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PASSWORD")
}

func isLoopback(r *http.Request) bool {
	host := r.RemoteAddr
	return strings.HasPrefix(host, "127.0.0.1:") || strings.HasPrefix(host, "[::1]:")
}

func maskEnvValue(val string) string {
	if val == "" {
		return ""
	}
	if len(val) <= 4 {
		return "****"
	}
	return val[:2] + "****" + val[len(val)-2:]
}

func readEnvFile(path string) map[string]string {
	result := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			result[line[:idx]] = line[idx+1:]
		}
	}
	return result
}

func writeEnvFile(path string, envMap map[string]string) {
	var lines []string
	for k, v := range envMap {
		lines = append(lines, k+"="+v)
	}
	data := strings.Join(lines, "\n")
	tmp := path + ".tmp"
	os.WriteFile(tmp, []byte(data), 0644)
	os.Rename(tmp, path)
}
