package packapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"sounds-great-ai/pkg/pack"
)

// Handler HTTP API 处理器
type Handler struct {
	pack      *pack.Pack
	breedsDir string
}

// NewHandler 创建一个新的 Handler
func NewHandler(p *pack.Pack, breedsDir string) *Handler {
	return &Handler{pack: p, breedsDir: breedsDir}
}

// Routes 返回 HTTP 路由
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/breeds", h.CreateBreed)
	mux.HandleFunc("GET /api/breeds", h.ListBreeds)
	mux.HandleFunc("DELETE /api/breeds/{id}", h.DeleteBreed)
	mux.HandleFunc("POST /api/breeds/{id}/bark", h.BarkBreed)
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
