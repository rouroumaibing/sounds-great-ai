package transport

import (
	"encoding/json"
	"fmt"
	"net/http"

	"sounds-great-ai/internal/mcp"
	"sounds-great-ai/internal/mcp/governance"
)

// MCPHandler exposes operator-facing CRUD + tool-introspection for the
// persisted MCP server registry. Operators install/configure the MCP servers
// that CLI agents receive via --mcp-config, and the panel discloses each
// server's live tools.
type MCPHandler struct {
	store *mcp.FileStore
	probe *mcp.ProbeCache
}

func NewMCPHandler(store *mcp.FileStore, probe *mcp.ProbeCache) *MCPHandler {
	if probe == nil {
		probe = mcp.NewProbeCache(0, 0)
	}
	return &MCPHandler{store: store, probe: probe}
}

// mcpServerView is the API representation of a server. Env values are masked
// so secrets are never leaked in responses; the operator sends raw env only on
// create/update.
type mcpServerView struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name,omitempty"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	// Remote (HTTP/SSE) transport fields.
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	// CallbackURL is the HTTP fallback surfaced to the agent when the primary
	// transport is unreachable.
	CallbackURL      string `json:"callback_url,omitempty"`
	FallbackAvailable bool   `json:"fallback_available"`
	Enabled          bool   `json:"enabled"`
	Builtin          bool   `json:"builtin"`
	Breeds           []string `json:"breeds,omitempty"`
	Tools            []string `json:"tools"`
	Status           string   `json:"status"`
	Error            string   `json:"error,omitempty"`
}

func maskEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for k := range env {
		out[k] = "***"
	}
	return out
}

func maskHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k := range headers {
		out[k] = "***"
	}
	return out
}

func (h *MCPHandler) toView(cfg mcp.MCPServerConfig) mcpServerView {
	tools, status, errMsg := h.probe.Get(cfg.Name, &cfg, false)
	return mcpServerView{
		Name:             cfg.Name,
		DisplayName:      cfg.DisplayName,
		Command:          cfg.Command,
		Args:             cfg.Args,
		Env:              maskEnv(cfg.Env),
		URL:              cfg.URL,
		Headers:          maskHeaders(cfg.Headers),
		CallbackURL:      cfg.CallbackURL,
		FallbackAvailable: cfg.CallbackURL != "" || cfg.Name == "platform",
		Enabled:          cfg.Enabled,
		Builtin:          cfg.Builtin,
		Breeds:           cfg.Breeds,
		Tools:            tools,
		Status:           status,
		Error:            errMsg,
	}
}

// mcpFallbackTool describes one fallback (HTTP callback) tool mapping.
type mcpFallbackTool struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Sample string `json:"sample"`
}

// mcpFallbackView is returned by GET /api/mcp/servers/{name}/fallback. It tells
// the agent how to reach the same capability over plain HTTP when the MCP
// transport (stdio or remote) is unavailable, made explicit and auditable.
type mcpFallbackView struct {
	Name        string             `json:"name"`
	CallbackURL string             `json:"callback_url,omitempty"`
	Tools       []mcpFallbackTool  `json:"tools,omitempty"`
	Note        string             `json:"note,omitempty"`
}

// Routes returns an http.Handler subtree mounted at /api/mcp/servers.
func (h *MCPHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/mcp/servers", h.list)
	mux.HandleFunc("POST /api/mcp/servers", h.add)
	mux.HandleFunc("GET /api/mcp/servers/{name}/fallback", h.fallback)
	mux.HandleFunc("PUT /api/mcp/servers/{name}", h.update)
	mux.HandleFunc("DELETE /api/mcp/servers/{name}", h.remove)
	return mux
}

// fallback returns the HTTP callback surface for a server: when the MCP
// transport is down, the agent can call the capability directly over HTTP. For
// the builtin "platform" server this is generated from the governed catalog
// (each tool maps 1:1 to an SG REST endpoint); for other servers it surfaces
// the configured CallbackURL.
func (h *MCPHandler) fallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.PathValue("name")
	cfg, ok := h.store.Get(name)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "mcp server " + name + " not found"})
		return
	}
	view := mcpFallbackView{Name: name, CallbackURL: cfg.CallbackURL}
	if name == "platform" {
		// Generate the REST fallback mapping from the governed catalog so the
		// agent has ready-to-use instructions without the MCP transport.
		tools := make([]mcpFallbackTool, 0, len(governance.Catalog()))
		for _, t := range governance.Catalog() {
			tools = append(tools, mcpFallbackTool{
				Name:   t.Name,
				Method: t.Method,
				Path:   t.Path,
				Sample: buildFallbackSample(t, cfg.CallbackURL),
			})
		}
		view.Tools = tools
		if view.CallbackURL == "" {
			view.CallbackURL = "http://localhost:8080"
		}
	} else if cfg.CallbackURL == "" {
		view.Note = "no HTTP callback fallback configured for this server"
	}
	respondJSON(w, http.StatusOK, view)
}

// buildFallbackSample renders a ready-to-use curl command for a governed tool
// against the server's callback URL. Auth is shown as a placeholder so the
// sample is copy-pasteable without leaking the real token.
func buildFallbackSample(t governance.ToolDefinition, base string) string {
	if base == "" {
		base = "http://localhost:8080"
	}
	url := base + t.Path
	auth := ""
	if t.ReadOnly {
		// Read endpoints still need auth in production; kept explicit.
		auth = " -H 'Authorization: Bearer <SG_API_TOKEN>'"
	}
	return fmt.Sprintf("curl -X %s '%s'%s", t.Method, url, auth)
}

func (h *MCPHandler) list(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	force := r.URL.Query().Get("refresh") == "1"
	servers := h.store.List()
	views := make([]mcpServerView, 0, len(servers))
	for _, s := range servers {
		v := h.toView(s)
		if force {
			tools, status, errMsg := h.probe.Get(s.Name, &s, true)
			v.Tools, v.Status, v.Error = tools, status, errMsg
		}
		views = append(views, v)
	}
	if views == nil {
		views = []mcpServerView{}
	}
	respondJSON(w, http.StatusOK, views)
}

func (h *MCPHandler) add(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body mcp.MCPServerConfig
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	body.Enabled = true // new servers are enabled by default
	if err := h.store.Add(body); err != nil {
		code := http.StatusBadRequest
		if isConflict(err) {
			code = http.StatusConflict
		}
		respondJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	cfg, _ := h.store.Get(body.Name)
	respondJSON(w, http.StatusCreated, h.toView(cfg))
}

func (h *MCPHandler) update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.PathValue("name")
	var patch mcp.MCPServerConfig
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	// A bare {enabled:bool} payload is the toggle path.
	if patch.Command == "" && patch.Args == nil && patch.Env == nil && patch.Breeds == nil && patch.DisplayName == "" && patch.URL == "" && patch.Headers == nil && patch.CallbackURL == "" && len(patch.Name) == 0 {
		if err := h.store.SetEnabled(name, patch.Enabled); err != nil {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
	} else {
		updated, err := h.store.Update(name, patch)
		if err != nil {
			code := http.StatusBadRequest
			if isNotFound(err) {
				code = http.StatusNotFound
			}
			respondJSON(w, code, map[string]string{"error": err.Error()})
			return
		}
		cfg := *updated
		respondJSON(w, http.StatusOK, h.toView(cfg))
		return
	}
	cfg, ok := h.store.Get(name)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "mcp server " + name + " not found"})
		return
	}
	respondJSON(w, http.StatusOK, h.toView(cfg))
}

func (h *MCPHandler) remove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.PathValue("name")
	if err := h.store.Remove(name); err != nil {
		code := http.StatusBadRequest
		if isNotFound(err) {
			code = http.StatusNotFound
		}
		respondJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isConflict(err error) bool {
	return containsSubstr(err.Error(), "already exists")
}

func isNotFound(err error) bool {
	return containsSubstr(err.Error(), "not found")
}

func containsSubstr(s, sub string) bool {
	return len(sub) > 0 && (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
