package packapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/pkg/pack"
)

// Handler HTTP API 处理器
type Handler struct {
	pack      *pack.Pack
	breedsDir string
	eventBus  *config.ConfigEventBus
}

// NewHandler 创建一个新的 Handler
func NewHandler(p *pack.Pack, breedsDir string) *Handler {
	return &Handler{pack: p, breedsDir: breedsDir}
}

// SetEventBus 注入配置事件总线（可选）
func (h *Handler) SetEventBus(bus *config.ConfigEventBus) { h.eventBus = bus }

// Routes 返回 HTTP 路由
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/breeds", h.CreateBreed)
	mux.HandleFunc("GET /api/breeds", h.ListBreeds)
	mux.HandleFunc("DELETE /api/breeds/{id}", h.DeleteBreed)
	mux.HandleFunc("POST /api/breeds/{id}/bark", h.BarkBreed)
	mux.HandleFunc("PATCH /api/breeds/{id}", h.UpdateBreed)
	mux.HandleFunc("GET /api/breeds/templates", h.GetTemplates)
	mux.HandleFunc("GET /api/breeds/{id}/status", h.GetBreedStatus)
	return mux
}

// CreateBreed POST /api/breeds — 创建/更新 breed
func (h *Handler) CreateBreed(w http.ResponseWriter, r *http.Request) {
	var cfg pack.BreedConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if cfg.Source == "" {
		cfg.Source = pack.BreedSourceUser
	}
	if err := h.pack.Validate(&cfg); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.pack.Register(&cfg); err != nil {
		respondError(w, http.StatusForbidden, err)
		return
	}
	if err := h.persistBreed(&cfg); err != nil {
		h.pack.Unregister(cfg.ID) // rollback
		respondError(w, http.StatusInternalServerError, err)
		return
	}
	respondOK(w, cfg)
}

// ListBreeds GET /api/breeds — 列出所有 breed
func (h *Handler) ListBreeds(w http.ResponseWriter, r *http.Request) {
	breeds := h.pack.List()
	respondOK(w, breeds)
}

// DeleteBreed DELETE /api/breeds/{id} — 删除 breed
func (h *Handler) DeleteBreed(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.pack.Unregister(id); err != nil {
		respondError(w, http.StatusForbidden, err)
		return
	}
	if err := h.removeBreed(id); err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}
	respondOK(w, nil)
}

// BarkBreed POST /api/breeds/{id}/bark — 执行指定 breed 的 Bark
func (h *Handler) BarkBreed(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input pack.TaskInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	out, err := h.pack.Bark(r.Context(), id, &input)
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondOK(w, out)
}

// UpdateBreed PATCH /api/breeds/{id} — partial update breed config
func (h *Handler) UpdateBreed(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg := h.pack.GetBreed(id)
	if cfg == nil {
		respondError(w, http.StatusNotFound, fmt.Errorf("breed %q not found", id))
		return
	}
	if cfg.Source == pack.BreedSourceSystem {
		respondError(w, http.StatusForbidden, fmt.Errorf("system breeds cannot be modified"))
		return
	}

	var updates map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	if v, ok := updates["display_name"]; ok {
		if err := json.Unmarshal(v, &cfg.DisplayName); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid display_name: %w", err))
			return
		}
	}
	if v, ok := updates["avatar"]; ok {
		if err := json.Unmarshal(v, &cfg.Avatar); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid avatar: %w", err))
			return
		}
	}
	if v, ok := updates["personality"]; ok {
		if err := json.Unmarshal(v, &cfg.Personality); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid personality: %w", err))
			return
		}
	}
	if v, ok := updates["role_description"]; ok {
		if err := json.Unmarshal(v, &cfg.RoleDescription); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid role_description: %w", err))
			return
		}
	}
	if v, ok := updates["team_strengths"]; ok {
		if err := json.Unmarshal(v, &cfg.TeamStrengths); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid team_strengths: %w", err))
			return
		}
	}
	if v, ok := updates["mention_patterns"]; ok {
		if err := json.Unmarshal(v, &cfg.MentionPatterns); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid mention_patterns: %w", err))
			return
		}
	}
	if v, ok := updates["roles"]; ok {
		if err := json.Unmarshal(v, &cfg.Roles); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid roles: %w", err))
			return
		}
	}
	if v, ok := updates["variants"]; ok {
		if err := json.Unmarshal(v, &cfg.Variants); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid variants: %w", err))
			return
		}
	}

	if err := h.pack.Validate(cfg); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.persistBreed(cfg); err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}
	if h.eventBus != nil {
		h.eventBus.Emit(config.ConfigEvent{Source: "breed-config", Scope: "domain", ChangedKeys: []string{id}})
	}
	respondOK(w, cfg)
}

// GetTemplates GET /api/breeds/templates — role templates
func (h *Handler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	tmpl, err := h.loadDogTemplate()
	if err != nil {
		respondOK(w, []any{})
		return
	}
	respondOK(w, tmpl.RoleTemplates)
}

// GetBreedStatus GET /api/breeds/{id}/status — runtime status
func (h *Handler) GetBreedStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.pack.GetBreed(id) == nil {
		respondError(w, http.StatusNotFound, fmt.Errorf("breed %q not found", id))
		return
	}
	respondOK(w, map[string]any{
		"id":             id,
		"status":         "idle",
		"current_task":   "",
		"last_active_at": "",
	})
}

// breedsFilePath returns the single consolidated template file path.
func (h *Handler) breedsFilePath() string {
	return filepath.Join(h.breedsDir, "dog-template.json")
}

// loadDogTemplate reads the consolidated template file. If the file is missing,
// an empty template (version 2) is returned so callers can still upsert.
func (h *Handler) loadDogTemplate() (*pack.DogTemplateFile, error) {
	path := h.breedsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &pack.DogTemplateFile{Version: 2}, nil
		}
		return nil, err
	}
	var tmpl pack.DogTemplateFile
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, err
	}
	if tmpl.Breeds == nil {
		tmpl.Breeds = []pack.BreedConfig{}
	}
	return &tmpl, nil
}

// saveDogTemplate writes the consolidated template file.
func (h *Handler) saveDogTemplate(tmpl *pack.DogTemplateFile) error {
	path := h.breedsFilePath()
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// persistBreed upserts a breed into the consolidated template file.
func (h *Handler) persistBreed(cfg *pack.BreedConfig) error {
	tmpl, err := h.loadDogTemplate()
	if err != nil {
		return err
	}
	replaced := false
	for i := range tmpl.Breeds {
		if tmpl.Breeds[i].ID == cfg.ID {
			tmpl.Breeds[i] = *cfg
			replaced = true
			break
		}
	}
	if !replaced {
		tmpl.Breeds = append(tmpl.Breeds, *cfg)
	}
	return h.saveDogTemplate(tmpl)
}

// removeBreed deletes a breed from the consolidated template file.
func (h *Handler) removeBreed(id string) error {
	tmpl, err := h.loadDogTemplate()
	if err != nil {
		return err
	}
	kept := tmpl.Breeds[:0]
	for _, b := range tmpl.Breeds {
		if b.ID != id {
			kept = append(kept, b)
		}
	}
	tmpl.Breeds = kept
	return h.saveDogTemplate(tmpl)
}

func respondOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
