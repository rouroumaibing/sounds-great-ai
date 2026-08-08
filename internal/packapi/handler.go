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
	if err := saveBreedFile(h.breedsDir, &cfg); err != nil {
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
	deleteBreedFile(h.breedsDir, id)
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
		json.Unmarshal(v, &cfg.DisplayName)
	}
	if v, ok := updates["avatar"]; ok {
		json.Unmarshal(v, &cfg.Avatar)
	}
	if v, ok := updates["personality"]; ok {
		json.Unmarshal(v, &cfg.Personality)
	}
	if v, ok := updates["role_description"]; ok {
		json.Unmarshal(v, &cfg.RoleDescription)
	}
	if v, ok := updates["team_strengths"]; ok {
		json.Unmarshal(v, &cfg.TeamStrengths)
	}
	if v, ok := updates["mention_patterns"]; ok {
		json.Unmarshal(v, &cfg.MentionPatterns)
	}
	if v, ok := updates["roles"]; ok {
		json.Unmarshal(v, &cfg.Roles)
	}
	if v, ok := updates["variants"]; ok {
		json.Unmarshal(v, &cfg.Variants)
	}

	if err := h.pack.Validate(cfg); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if err := saveBreedFile(h.breedsDir, cfg); err != nil {
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
	path := filepath.Join(h.breedsDir, "cat-template.json")
	data, err := os.ReadFile(path)
	if err != nil {
		respondOK(w, []any{})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
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

// saveBreedFile 将 breed 配置持久化到 JSON 文件
func saveBreedFile(dir string, cfg *pack.BreedConfig) error {
	path := filepath.Join(dir, cfg.ID+".json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// deleteBreedFile 删除 breed JSON 文件
func deleteBreedFile(dir, id string) {
	os.Remove(filepath.Join(dir, id+".json"))
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
