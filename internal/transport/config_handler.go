package transport

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/settings"
)

// ConfigHandler handles advanced config management HTTP endpoints.
type ConfigHandler struct {
	breedLoader   *config.Loader
	breedsDir     string
	settingsStore settings.SettingsStore
	envPath       string
	leader        *config.LeaderConfig
}

func NewConfigHandler(loader *config.Loader, breedsDir string, store settings.SettingsStore, envPath string) *ConfigHandler {
	return &ConfigHandler{breedLoader: loader, breedsDir: breedsDir, settingsStore: store, envPath: envPath}
}

// SetLeader wires the platform's LeaderConfig pointer for GET/PATCH /api/config/leader.
func (h *ConfigHandler) SetLeader(l *config.LeaderConfig) {
	h.leader = l
}

func (h *ConfigHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config/default-breed", h.GetDefaultBreed)
	mux.HandleFunc("PUT /api/config/default-breed", h.SetDefaultBreed)
	mux.HandleFunc("GET /api/config/breed-order", h.GetBreedOrder)
	mux.HandleFunc("PUT /api/config/breed-order", h.SetBreedOrder)
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

func (h *ConfigHandler) SetDefaultBreed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BreedID string `json:"breed_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if body.BreedID != "" {
		breeds, _ := h.breedLoader.LoadFromFile(filepath.Join(h.breedsDir, "dog-template.json"))
		if _, ok := breeds[body.BreedID]; !ok {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "breed not found"})
			return
		}
	}
	os.Setenv("DEFAULT_BREED_ID", body.BreedID)
	respondJSON(w, http.StatusOK, map[string]string{"breed_id": body.BreedID})
}

func (h *ConfigHandler) GetBreedOrder(w http.ResponseWriter, r *http.Request) {
	configs, _ := h.settingsStore.ListConfig()
	for _, c := range configs {
		if c.Key == "breed_order" && c.Value != "" {
			var order []string
			if json.Unmarshal([]byte(c.Value), &order) == nil {
				respondJSON(w, http.StatusOK, map[string][]string{"order": order})
				return
			}
		}
	}
	breeds, _ := h.breedLoader.LoadFromFile(filepath.Join(h.breedsDir, "dog-template.json"))
	order := make([]string, 0, len(breeds))
	for id := range breeds {
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
	breeds, _ := h.breedLoader.LoadFromFile(filepath.Join(h.breedsDir, "dog-template.json"))
	var missing []string
	for _, id := range body.Order {
		if _, ok := breeds[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown breed IDs", "missing": missing})
		return
	}
	orderJSON, _ := json.Marshal(body.Order)
	h.settingsStore.UpdateConfig("breed_order", string(orderJSON))
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
	if h.leader == nil {
		defaultCfg := config.DefaultLeaderConfig()
		respondJSON(w, http.StatusOK, defaultCfg)
		return
	}
	respondJSON(w, http.StatusOK, h.leader)
}

func (h *ConfigHandler) UpdateLeader(w http.ResponseWriter, r *http.Request) {
	var body config.LeaderConfig
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := body.Validate(); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if h.leader == nil {
		h.leader = &body
	} else {
		h.leader.Name = body.Name
		h.leader.Aliases = body.Aliases
		h.leader.MentionPatterns = body.MentionPatterns
		h.leader.TimeZone = body.TimeZone
		h.leader.Avatar = body.Avatar
		h.leader.ColorPrimary = body.ColorPrimary
		h.leader.ColorSecondary = body.ColorSecondary
	}
	respondJSON(w, http.StatusOK, h.leader)
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
