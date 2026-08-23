package transport

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sounds-great-ai/internal/settings"

	"github.com/google/uuid"
)

// PanelsHandler serves the UI-panel configuration endpoints.
//
// Concierge / voice / connectors are real persisted config backed by
// settings.PanelConfigStore (<ConfigRoot>/panels/*.json, panels-roadmap P1+P2).
// The plugins/marketplace surface lives in PluginsHandler (P3; P4 stub).
//
// CORS is handled centrally by transport.CORSMiddleware wrapping the whole
// mux; nothing here sets permissive headers. Auth is applied at mount time
// via auth.Wrap (same as every other config surface).
type PanelsHandler struct {
	store *settings.PanelConfigStore
}

func NewPanelsHandler(store *settings.PanelConfigStore) *PanelsHandler {
	return &PanelsHandler{store: store}
}

func (h *PanelsHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config/concierge", h.handleConcierge)
	mux.HandleFunc("/api/config/voice", h.handleVoice)
	mux.HandleFunc("/api/config/connectors", h.handleConnectors)
	mux.HandleFunc("/api/config/connectors/", h.handleConnectorItem)
	return mux
}

// --- Concierge (P1) ---

func (h *PanelsHandler) handleConcierge(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := h.store.LoadConcierge()
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, cfg)
	case http.MethodPatch:
		cfg, err := h.store.LoadConcierge()
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		var body struct {
			Avatar               *string `json:"avatar"`
			Color                *string `json:"color"`
			Size                 *int    `json:"size"`
			Personality          *string `json:"personality"`
			Greeting             *string `json:"greeting"`
			DutyBreed            *string `json:"dutyBreed"`
			AutoSuggestThreshold *int    `json:"autoSuggestThreshold"`
			ProactivityLevel     *string `json:"proactivityLevel"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		if body.Avatar != nil {
			if len([]rune(*body.Avatar)) > 16 {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "avatar too long (max 16 runes)"})
				return
			}
			cfg.Avatar = *body.Avatar
		}
		if body.Color != nil {
			if !isHexColor(*body.Color) {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "color must be #RRGGBB"})
				return
			}
			cfg.Color = *body.Color
		}
		if body.Size != nil {
			if *body.Size < 16 || *body.Size > 256 {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "size must be 16-256"})
				return
			}
			cfg.Size = *body.Size
		}
		if body.Personality != nil {
			if len([]rune(*body.Personality)) > 2000 {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "personality too long (max 2000 runes)"})
				return
			}
			cfg.Personality = *body.Personality
		}
		if body.Greeting != nil {
			if len([]rune(*body.Greeting)) > 2000 {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "greeting too long (max 2000 runes)"})
				return
			}
			cfg.Greeting = *body.Greeting
		}
		if body.DutyBreed != nil {
			if len([]rune(*body.DutyBreed)) > 64 {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "dutyBreed too long (max 64 runes)"})
				return
			}
			cfg.DutyBreed = *body.DutyBreed
		}
		if body.AutoSuggestThreshold != nil {
			if *body.AutoSuggestThreshold < 0 || *body.AutoSuggestThreshold > 20 {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "autoSuggestThreshold must be 0-20"})
				return
			}
			cfg.AutoSuggestThreshold = *body.AutoSuggestThreshold
		}
		if body.ProactivityLevel != nil {
			switch *body.ProactivityLevel {
			case "low", "medium", "high":
				cfg.ProactivityLevel = *body.ProactivityLevel
			default:
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "proactivityLevel must be low|medium|high"})
				return
			}
		}
		if err := h.store.SaveConcierge(cfg); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, cfg)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Voice (P1) ---

func (h *PanelsHandler) handleVoice(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := h.store.LoadVoice()
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, cfg)
	case http.MethodPatch:
		cfg, err := h.store.LoadVoice()
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		var body struct {
			Enabled          *bool                         `json:"enabled"`
			TTSVoice         *string                       `json:"ttsVoice"`
			TTSLang          *string                       `json:"ttsLang"`
			TTSSpeed         *float64                      `json:"ttsSpeed"`
			TTSRefAudio      *string                       `json:"ttsRefAudio"`
			STTModel         *string                       `json:"sttModel"`
			STTLanguage      *string                       `json:"sttLanguage"`
			STTAutoTranscrib *bool                         `json:"sttAutoTranscribe"`
			Glossary         []settings.VoiceGlossaryEntry `json:"glossary"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		strPtr := func(p *string, max int, field string) (string, bool) {
			if p == nil {
				return "", true
			}
			if len([]rune(*p)) > max {
				return "", false
			}
			return *p, true
		}
		if body.Enabled != nil {
			cfg.Enabled = *body.Enabled
		}
		if v, ok := strPtr(body.TTSVoice, 64, "ttsVoice"); !ok {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "ttsVoice too long (max 64)"})
			return
		} else if body.TTSVoice != nil {
			cfg.TTSVoice = v
		}
		if v, ok := strPtr(body.TTSLang, 16, "ttsLang"); !ok {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "ttsLang too long (max 16)"})
			return
		} else if body.TTSLang != nil {
			cfg.TTSLang = v
		}
		if body.TTSSpeed != nil {
			if *body.TTSSpeed < 0.25 || *body.TTSSpeed > 4.0 {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "ttsSpeed must be 0.25-4.0"})
				return
			}
			cfg.TTSSpeed = *body.TTSSpeed
		}
		if v, ok := strPtr(body.TTSRefAudio, 512, "ttsRefAudio"); !ok {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "ttsRefAudio too long (max 512)"})
			return
		} else if body.TTSRefAudio != nil {
			cfg.TTSRefAudio = v
		}
		if v, ok := strPtr(body.STTModel, 64, "sttModel"); !ok {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "sttModel too long (max 64)"})
			return
		} else if body.STTModel != nil {
			cfg.STTModel = v
		}
		if v, ok := strPtr(body.STTLanguage, 16, "sttLanguage"); !ok {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "sttLanguage too long (max 16)"})
			return
		} else if body.STTLanguage != nil {
			cfg.STTLanguage = v
		}
		if body.STTAutoTranscrib != nil {
			cfg.STTAutoTranscrib = *body.STTAutoTranscrib
		}
		if body.Glossary != nil {
			if len(body.Glossary) > 200 {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "glossary too large (max 200 entries)"})
				return
			}
			for _, g := range body.Glossary {
				if len([]rune(g.Source)) > 200 || len([]rune(g.Target)) > 200 {
					respondJSON(w, http.StatusBadRequest, map[string]string{"error": "glossary entry too long (max 200 runes)"})
					return
				}
			}
			cfg.Glossary = body.Glossary
		}
		if err := h.store.SaveVoice(cfg); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, cfg)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- IM Connectors (P2) ---

// connectorView is the masked read model: the raw auth key never leaves the
// server (key_set/key_preview mirror the accounts credentials discipline).
type connectorView struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Endpoint       string `json:"endpoint"`
	Enabled        bool   `json:"enabled"`
	LastCheck      string `json:"last_check,omitempty"`
	AuthKeySet     bool   `json:"auth_key_set"`
	AuthKeyPreview string `json:"auth_key_preview,omitempty"`
}

func maskKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 6 {
		return "***"
	}
	return k[:3] + "…" + k[len(k)-2:]
}

func toView(c settings.Connector) connectorView {
	return connectorView{
		ID:             c.ID,
		Name:           c.Name,
		Type:           c.Type,
		Endpoint:       c.Endpoint,
		Enabled:        c.Enabled,
		LastCheck:      c.LastCheck,
		AuthKeySet:     c.AuthKey != "",
		AuthKeyPreview: maskKey(c.AuthKey),
	}
}

func (h *PanelsHandler) handleConnectors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := h.store.ListConnectors()
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		views := make([]connectorView, 0, len(list))
		for _, c := range list {
			views = append(views, toView(c))
		}
		respondJSON(w, http.StatusOK, views)
	case http.MethodPost:
		var body struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Endpoint string `json:"endpoint"`
			AuthKey  string `json:"auth_key"`
			Enabled  *bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		c, err := h.upsertConnector(settings.Connector{}, body.Name, body.Type, body.Endpoint, body.AuthKey, body.Enabled)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.ID = uuid.NewString()
		list, _ := h.store.ListConnectors()
		if err := h.store.SaveConnectors(append(list, c)); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusCreated, toView(c))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PanelsHandler) handleConnectorItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/config/connectors/")
	id = strings.TrimSuffix(id, "/test")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	list, err := h.store.ListConnectors()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	idx := -1
	for i, c := range list {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "connector not found"})
		return
	}

	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/test") {
		result := probeConnector(list[idx])
		list[idx].LastCheck = result["status"].(string) + " (" + time.Now().UTC().Format(time.RFC3339) + ")"
		if err := h.store.SaveConnectors(list); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, result)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Name     *string `json:"name"`
			Type     *string `json:"type"`
			Endpoint *string `json:"endpoint"`
			AuthKey  *string `json:"auth_key"` // empty string clears; nil keeps
			Enabled  *bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		cur := list[idx]
		name, typ, endpoint, key := cur.Name, cur.Type, cur.Endpoint, cur.AuthKey
		if body.Name != nil {
			name = *body.Name
		}
		if body.Type != nil {
			typ = *body.Type
		}
		if body.Endpoint != nil {
			endpoint = *body.Endpoint
		}
		if body.AuthKey != nil {
			key = *body.AuthKey
		}
		updated, err := h.upsertConnector(cur, name, typ, endpoint, key, body.Enabled)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		list[idx] = updated
		if err := h.store.SaveConnectors(list); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, toView(updated))
	case http.MethodDelete:
		list = append(list[:idx], list[idx+1:]...)
		if err := h.store.SaveConnectors(list); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// upsertConnector validates the mutable fields and returns the merged
// connector. enabled==nil keeps the current value.
func (h *PanelsHandler) upsertConnector(cur settings.Connector, name, typ, endpoint, key string, enabled *bool) (settings.Connector, error) {
	name = strings.TrimSpace(name)
	if n := len([]rune(name)); n < 1 || n > 64 {
		return cur, errString("name must be 1-64 runes")
	}
	switch typ {
	case "slack", "telegram", "webhook", "generic":
	default:
		return cur, errString("type must be slack|telegram|webhook|generic")
	}
	if len(endpoint) > 512 {
		return cur, errString("endpoint too long (max 512)")
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return cur, errString("endpoint must be a valid http(s) URL")
	}
	if len([]rune(key)) > 512 {
		return cur, errString("auth_key too long (max 512)")
	}
	cur.Name = name
	cur.Type = typ
	cur.Endpoint = endpoint
	cur.AuthKey = key
	if enabled != nil {
		cur.Enabled = *enabled
	}
	return cur, nil
}

type errString string

func (e errString) Error() string { return string(e) }

// probeConnector performs a lightweight GET against the connector endpoint
// (with the configured bearer key when present) and reports
// reachability + latency. Operator-only surface, same trust level as
// POST /api/repo/test.
func probeConnector(c settings.Connector) map[string]any {
	req, err := http.NewRequest(http.MethodGet, c.Endpoint, nil)
	if err != nil {
		return map[string]any{"ok": false, "latency_ms": 0, "status": "invalid endpoint", "error": err.Error()}
	}
	if c.AuthKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthKey)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return map[string]any{"ok": false, "latency_ms": elapsed, "status": "unreachable", "error": err.Error()}
	}
	defer resp.Body.Close()
	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	status := "reachable"
	if !ok {
		status = "http " + resp.Status
	}
	return map[string]any{"ok": ok, "latency_ms": elapsed, "status": status}
}

func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
