package transport

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"sounds-great-ai/internal/skills"
)

// SkillsHandler 暴露 Skills Framework 的读写/同步/漂移治理 API（替代原 routes.go
// 中仅返回文件名的 SkillsHandler）。所有写路由在 routes.go 中经 auth 中间件包裹。
type SkillsHandler struct {
	mgr         *skills.SkillManager
	workspaceDir string
	homeDir     string
}

// NewSkillsHandler 构造 skills handler。mgr 为 nil 时所有写操作与列表降级为安全响应。
func NewSkillsHandler(mgr *skills.SkillManager, workspaceDir, homeDir string) *SkillsHandler {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	return &SkillsHandler{mgr: mgr, workspaceDir: workspaceDir, homeDir: homeDir}
}

// Routes 返回挂载在 /api/skills 子树下的子路由器。
func (h *SkillsHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/skills", h.list)
	mux.HandleFunc("GET /api/skills/{id}", h.detail)
	mux.HandleFunc("PATCH /api/skills/{id}", h.patch)
	mux.HandleFunc("POST /api/skills/sync", h.sync)
	mux.HandleFunc("POST /api/skills/drift/check", h.driftCheck)
	mux.HandleFunc("POST /api/skills/drift/resolve", h.driftResolve)
	mux.HandleFunc("GET /api/skills/security", h.securityList)
	mux.HandleFunc("POST /api/skills/security/{id}/approve", h.securityApprove)
	mux.HandleFunc("POST /api/skills/security/{id}/quarantine", h.securityQuarantine)
	mux.HandleFunc("POST /api/skills/security/{id}/revoke", h.securityRevoke)
	return mux
}

func (h *SkillsHandler) syncOpts() skills.SkillSyncOptions {
	return skills.SkillSyncOptions{WorkspaceDir: h.workspaceDir, HomeDir: h.homeDir}
}

// skillListItem 是 GET /api/skills 的单项结构。
type skillListItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Triggers    []string `json:"triggers"`
	Source      string   `json:"source"`
	Enabled     bool     `json:"enabled"`
	Scope       string   `json:"scope"`
	MountPoints []string `json:"mountPoints"`
	MountHealth string   `json:"mountHealth"`
	Security    string   `json:"security,omitempty"` // approved|pending|quarantined|revoked
}

// skillDetail 是 GET /api/skills/{id} 的详情结构。
type skillDetail struct {
	skillListItem
	Content string `json:"content"`
	Path    string `json:"path"`
}

func (h *SkillsHandler) toItem(s *skills.Skill) skillListItem {
	it := h.mgr.IntentOf(s.ID)
	enabled := it != nil && it.Enabled
	scope := ""
	// 序列化契约：triggers / mountPoints 永远是数组。nil slice 会被编成
	// null，前端 SkillsPanel 直接对这两个字段调 .map/.includes，一个刚清空
	// 配置（全部 intent 缺失）的工作区会把整个设置面板打进 ErrorBoundary。
	mps := []string{}
	if it != nil {
		scope = it.Scope
		if it.MountPoints != nil {
			mps = it.MountPoints
		}
	}
	triggers := s.AllTriggers()
	if triggers == nil {
		triggers = []string{}
	}
	return skillListItem{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Category:    s.Category,
		Triggers:    triggers,
		Source:      s.Source,
		Enabled:     enabled,
		Scope:       scope,
		MountPoints: mps,
		MountHealth: h.mountHealth(s, it),
		Security:    h.securityStatus(s.ID),
	}
}

// securityStatus 返回某 skill 的安全状态摘要（无状态视为 approved，与内部可信源默认一致）。
func (h *SkillsHandler) securityStatus(id string) string {
	st := h.mgr.SecurityState(id)
	if st == nil {
		return string(skills.SecurityApproved)
	}
	return string(st.Status)
}

// mountHealth 计算该 skill 的挂载健康摘要（对支持原生目录的 carrier 做磁盘检查）。
func (h *SkillsHandler) mountHealth(s *skills.Skill, it *skills.SkillIntent) string {
	if it == nil || !it.Enabled {
		return "disabled"
	}
	carriers := it.MountPoints
	if len(carriers) == 0 {
		carriers = skills.KnownCarriers
	}
	hasNative := false
	missing := false
	for _, c := range carriers {
		dir := skills.NativeSkillsDir(c, it.Scope, h.syncOpts())
		if dir == "" {
			continue // 逻辑挂载 carrier，无磁盘态
		}
		hasNative = true
		link := filepath.Join(dir, s.ID)
		info, err := os.Lstat(link)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			missing = true
		}
	}
	if hasNative {
		if missing {
			return "missing"
		}
		return "mounted"
	}
	return "logical"
}

func (h *SkillsHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.mgr == nil {
		json.NewEncoder(w).Encode([]skillListItem{})
		return
	}
	all := h.mgr.All()
	items := make([]skillListItem, 0, len(all))
	for _, s := range all {
		items = append(items, h.toItem(s))
	}
	json.NewEncoder(w).Encode(items)
}

func (h *SkillsHandler) detail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.PathValue("id")
	if h.mgr == nil {
		http.Error(w, "skills unavailable", http.StatusServiceUnavailable)
		return
	}
	s := h.mgr.Get(id)
	if s == nil {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}
	item := h.toItem(s)
	detail := skillDetail{skillListItem: item, Content: s.Body, Path: s.FilePath}
	json.NewEncoder(w).Encode(detail)
}

type skillPatchBody struct {
	Enabled     *bool    `json:"enabled"`
	Scope       *string  `json:"scope"`
	MountPoints []string `json:"mountPoints"`
}

func (h *SkillsHandler) patch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.PathValue("id")
	if h.mgr == nil {
		http.Error(w, "skills unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.mgr.Get(id) == nil {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}
	var body skillPatchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	scope := ""
	if it := h.mgr.IntentOf(id); it != nil {
		scope = it.Scope
	}
	if body.Scope != nil {
		scope = *body.Scope
	}
	if body.Enabled != nil {
		if err := h.mgr.SetEnabled(id, *body.Enabled, scope); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if body.MountPoints != nil {
		if err := h.mgr.SetMountPoints(id, body.MountPoints); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	// 意图变更后最佳努力调谐（物理挂载）。失败不阻断写操作，仅记录。
	if err := h.mgr.SyncSkillMounts(h.syncOpts()); err != nil {
		log.Printf("skill sync after patch failed: %v", err)
	}
	s := h.mgr.Get(id)
	json.NewEncoder(w).Encode(h.toItem(s))
}

func (h *SkillsHandler) sync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.mgr == nil {
		http.Error(w, "skills unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := h.mgr.SyncSkillMounts(h.syncOpts()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (h *SkillsHandler) driftCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.mgr == nil {
		http.Error(w, "skills unavailable", http.StatusServiceUnavailable)
		return
	}
	issues := h.mgr.DetectSkillDrift(h.syncOpts())
	if issues == nil {
		issues = []skills.DriftIssue{}
	}
	json.NewEncoder(w).Encode(map[string]any{"issues": issues, "count": len(issues)})
}

type driftResolveBody struct {
	Strategy string `json:"strategy"`
}

func (h *SkillsHandler) driftResolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.mgr == nil {
		http.Error(w, "skills unavailable", http.StatusServiceUnavailable)
		return
	}
	var body driftResolveBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	strategy := body.Strategy
	if strategy == "" {
		strategy = "keep-project"
	}
	issues, err := h.mgr.ResolveSkillDrift(h.syncOpts(), strategy)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error(), "issues": issues})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "issues": issues})
}

// securityList 返回全部技能安全状态（用于前端展示与人工放行）。
func (h *SkillsHandler) securityList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.mgr == nil {
		json.NewEncoder(w).Encode(map[string]any{})
		return
	}
	json.NewEncoder(w).Encode(h.mgr.AllSecurityStates())
}

// securityAction 是批准/隔离/撤销三类动作的共用骨架。
func (h *SkillsHandler) securityAction(w http.ResponseWriter, r *http.Request, fn func(id, by string) error) {
	w.Header().Set("Content-Type", "application/json")
	id := r.PathValue("id")
	if h.mgr == nil {
		http.Error(w, "skills unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.mgr.Get(id) == nil {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}
	by := "operator"
	var body struct {
		By string `json:"by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.By != "" {
		by = body.By
	}
	if err := fn(id, by); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(h.mgr.SecurityState(id))
}

func (h *SkillsHandler) securityApprove(w http.ResponseWriter, r *http.Request) {
	h.securityAction(w, r, h.mgr.ApproveSkill)
}

func (h *SkillsHandler) securityQuarantine(w http.ResponseWriter, r *http.Request) {
	h.securityAction(w, r, h.mgr.QuarantineSkill)
}

func (h *SkillsHandler) securityRevoke(w http.ResponseWriter, r *http.Request) {
	h.securityAction(w, r, h.mgr.RevokeSkill)
}
