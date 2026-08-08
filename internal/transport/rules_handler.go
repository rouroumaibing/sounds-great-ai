package transport

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/hooks"
)

// RulesHandler serves rules data and prompt injection manifest.
type RulesHandler struct {
	hookRegistry *hooks.Registry
	breedLoader  *config.Loader
	breedsDir    string
	agentsPath   string
}

func NewRulesHandler(hookReg *hooks.Registry, loader *config.Loader, breedsDir, agentsPath string) *RulesHandler {
	return &RulesHandler{hookRegistry: hookReg, breedLoader: loader, breedsDir: breedsDir, agentsPath: agentsPath}
}

func (h *RulesHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/rules", h.GetRules)
	mux.HandleFunc("GET /api/prompt-injection/manifest", h.GetHookManifest)
	mux.HandleFunc("GET /api/prompt-injection/preview", h.CompilePreview)
	return mux
}

func (h *RulesHandler) GetRules(w http.ResponseWriter, r *http.Request) {
	ironLaws := []map[string]string{
		{"id": "1", "title": "数据存储保护区", "desc": "不删除 internal/memory/、internal/ragstore/ 或任何持久化存储的数据。测试用临时实例。"},
		{"id": "2", "title": "进程自保", "desc": "不杀父进程，不修改启动配置导致无法重启。"},
		{"id": "3", "title": "配置不可变", "desc": "不在运行时修改 internal/config/ 下的配置文件。配置变更需要人类介入。"},
		{"id": "4", "title": "网络边界", "desc": "不访问不属于本服务的 localhost 端口。"},
		{"id": "5", "title": "愿景不可违", "desc": "不违反 docs/VISION.md §4 的不可逆决策。如果要改，先更新 VISION.md。"},
	}

	redFlags := []map[string]string{
		{"pattern": "在 internal/ 层调 LLM 做推理", "violation": "§3 三层原则", "fix": "推理交给 CLI adapter"},
		{"pattern": "硬编码 workflow DAG", "violation": "§4.2 不可逆决策", "fix": "用动态路由"},
		{"pattern": "新建 A2A HTTP server/client", "violation": "§4.1 不可逆决策", "fix": "用 CLI adapter (stdin/stdout pipe)"},
		{"pattern": "在 platform 层做 agent reasoning", "violation": "§4.1 不可逆决策", "fix": "reasoning 是 CLI 的事"},
	}

	breedRestrictions := h.loadBreedRestrictions()
	modelGuides := h.loadModelGuides()
	agentsContent := h.readAgentsFile()

	respondJSON(w, http.StatusOK, map[string]any{
		"iron_laws":         ironLaws,
		"red_flags":         redFlags,
		"breed_restrictions": breedRestrictions,
		"model_guides":      modelGuides,
		"agents_content":    agentsContent,
	})
}

func (h *RulesHandler) GetHookManifest(w http.ResponseWriter, r *http.Request) {
	if h.hookRegistry == nil {
		respondJSON(w, http.StatusOK, map[string]any{"hooks": []any{}, "stages": []string{}})
		return
	}

	allHooks := h.hookRegistry.All()
	hookList := make([]map[string]any, 0, len(allHooks))
	stages := make([]string, 0)
	seenStage := make(map[string]bool)

	for _, hk := range allHooks {
		m := hk.Manifest
		hookList = append(hookList, map[string]any{
			"id":           m.ID,
			"name":         m.Name,
			"stage":        m.Stage,
			"order":        m.Order,
			"version":      m.Version,
			"enabled":      m.Enabled,
			"disableable":  m.Disableable,
			"resolver":     m.Resolver,
			"template":     m.Template,
			"inputs":       m.Inputs,
			"safety_tier":  m.SafetyTier,
			"governance":   m.GovernanceTier,
			"has_template": hk.Template != "",
		})
		if !seenStage[m.Stage] {
			stages = append(stages, m.Stage)
			seenStage[m.Stage] = true
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"hooks":  hookList,
		"stages": stages,
	})
}

func (h *RulesHandler) loadBreedRestrictions() []map[string]string {
	restrictions := []map[string]string{
		{"breed": "边牧 (bianmu)", "can": "任务分解、路由决策、结果合成", "cannot": "直接写业务代码、做 RAG 检索"},
		{"breed": "灵缇 (xigou)", "can": "代码搜索、分析、重构建议", "cannot": "改架构、改路由"},
		{"breed": "金毛 (jinmao)", "can": "RAG 检索、上下文组装", "cannot": "改代码逻辑、做 review"},
		{"breed": "德牧 (demu)", "can": "日志追踪、错误诊断", "cannot": "写新功能、改架构"},
		{"breed": "藏獒 (zangao)", "can": "输出格式化、渲染", "cannot": "改业务逻辑、做路由决策"},
		{"breed": "中华田园犬", "can": "命令拦截、路径校验、敏感过滤", "cannot": "写功能代码、做推理"},
	}
	return restrictions
}

func (h *RulesHandler) loadModelGuides() []map[string]string {
	return []map[string]string{
		{"adapter": "claude", "guide": "Claude CLI adapter — 使用 claude-opus-4-6 / claude-sonnet-4-20250514，支持 MCP、session chain"},
		{"adapter": "codex", "guide": "Codex CLI adapter — 使用 o3 模型，擅长代码搜索与安全审查"},
		{"adapter": "gemini", "guide": "Gemini CLI adapter — 使用 gemini-2.5-pro，擅长 RAG 检索与知识合成"},
		{"adapter": "opencode", "guide": "opencode CLI adapter — 通用 CLI 包装，支持多种模型"},
	}
}

func (h *RulesHandler) readAgentsFile() string {
	path := h.agentsPath
	if path == "" {
		path = "AGENTS.md"
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	// Truncate to reasonable size for API response
	if len(content) > 20000 {
		content = content[:20000] + "\n... (truncated)"
	}
	return content
}

// compilePreview generates a preview of the compiled prompt for a given breed.
func (h *RulesHandler) CompilePreview(w http.ResponseWriter, r *http.Request) {
	breedID := r.URL.Query().Get("breed")
	if breedID == "" {
		breedID = "bianmu"
	}

	var sections []string
	agentsContent := h.readAgentsFile()
	if agentsContent != "" {
		// Extract iron laws section
		if idx := strings.Index(agentsContent, "## Iron Laws"); idx >= 0 {
			end := strings.Index(agentsContent[idx:], "## ")
			if end > 0 {
				sections = append(sections, agentsContent[idx:idx+end])
			} else {
				sections = append(sections, agentsContent[idx:])
			}
		}
	}

	// Add hook templates if registry available
	if h.hookRegistry != nil {
		allHooks := h.hookRegistry.All()
		for _, hk := range allHooks {
			if hk.Manifest.Enabled && hk.Template != "" {
				sections = append(sections, fmt.Sprintf("### Hook: %s (%s)\n%s", hk.Manifest.ID, hk.Manifest.Stage, hk.Template))
			}
		}
	}

	compiled := strings.Join(sections, "\n\n---\n\n")
	respondJSON(w, http.StatusOK, map[string]any{
		"breed_id": breedID,
		"compiled": compiled,
		"sections": len(sections),
	})
}
