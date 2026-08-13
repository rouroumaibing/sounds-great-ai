package packapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

// Handler HTTP API 处理器
type Handler struct {
	pack      *pack.Pack
	breedsDir string
	store     settings.SettingsStore
	eventBus  *config.ConfigEventBus
}

// NewHandler 创建一个新的 Handler。
// store 是运行时成员目录的真相源（.sounds-great-ai/dog-catalog.json）：
// 成员的增删改都落盘到该 store，而非仓库内的 dog-template.json（模板仅作种子）。
func NewHandler(p *pack.Pack, breedsDir string, store settings.SettingsStore) *Handler {
	return &Handler{pack: p, breedsDir: breedsDir, store: store}
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
	if err := h.validateBreed(&cfg, ""); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
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

// validateBreed 集中校验：别名（mention_patterns）全局唯一、client_id 白名单、
// account_ref 必须是已存在账号或内置 OAuth ref（决策点 D1/D5）。
func (h *Handler) validateBreed(cfg *pack.BreedConfig, excludeID string) error {
	conflictPattern, owner, ok := pack.CheckMentionPatternsUnique(h.pack.List(), cfg.MentionPatterns, excludeID)
	if !ok {
		return fmt.Errorf("alias %q is already used by member %q", conflictPattern, owner)
	}
	for _, v := range cfg.Variants {
		if !settings.ValidateClientID(v.ClientID) {
			return fmt.Errorf("invalid client_id %q; allowed: claude, codex, gemini, opencode, kimi", v.ClientID)
		}
		if err := settings.ValidateAccountRef(h.store, v.AccountRef); err != nil {
			return err
		}
	}
	return nil
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
	if v, ok := updates["color"]; ok {
		if err := json.Unmarshal(v, &cfg.Color); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid color: %w", err))
			return
		}
	}
	if v, ok := updates["nickname"]; ok {
		if err := json.Unmarshal(v, &cfg.Nickname); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid nickname: %w", err))
			return
		}
	}
	if v, ok := updates["caution"]; ok {
		if err := json.Unmarshal(v, &cfg.Caution); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid caution: %w", err))
			return
		}
	}
	if v, ok := updates["default_variant_id"]; ok {
		if err := json.Unmarshal(v, &cfg.DefaultVariantID); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid default_variant_id: %w", err))
			return
		}
	}
	if v, ok := updates["features"]; ok {
		var f pack.BreedFeatures
		if err := json.Unmarshal(v, &f); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid features: %w", err))
			return
		}
		cfg.Features = &f
	}
	if v, ok := updates["restrictions"]; ok {
		if err := json.Unmarshal(v, &cfg.Restrictions); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid restrictions: %w", err))
			return
		}
	}
	if v, ok := updates["relationship_key"]; ok {
		if err := json.Unmarshal(v, &cfg.RelationshipKey); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid relationship_key: %w", err))
			return
		}
	}
	if v, ok := updates["variants"]; ok {
		if err := json.Unmarshal(v, &cfg.Variants); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Errorf("invalid variants: %w", err))
			return
		}
	}

	if err := h.validateBreed(cfg, cfg.ID); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
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

// GetTemplates GET /api/breeds/templates — role templates (模板只读，种子用途)
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

// loadDogTemplate reads the consolidated template file (role_templates 等只读
// 种子数据). 模板不再被运行时写入（成员落 catalog）。
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

// persistBreed 把 breed 落盘到运行时 catalog（settings store）。
// 首次写入时若 roster 无对应条目，store 内部会自动补一个默认条目。
func (h *Handler) persistBreed(cfg *pack.BreedConfig) error {
	return h.store.CreateBreed(cfg)
}

// removeBreed 从运行时 catalog 删除 breed 及其 roster 条目。
// 若 breed 在 catalog 中不存在（例如它只是一个在内存 pack 中、尚未落盘到
// catalog 的模板种子犬），视作已删除（幂等），不返回错误——因为 pack 层面
// 的 Unregister 已经成功，运行时成员确实已被移除。
func (h *Handler) removeBreed(id string) error {
	if err := h.store.DeleteBreed(id); err != nil && !errors.Is(err, settings.ErrBreedNotFound) {
		return err
	}
	// roster 条目随 breed 删除（幂等：条目不存在也不报错）。
	return h.store.DeleteRosterEntry(id)
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
