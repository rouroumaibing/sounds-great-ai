package transport

import (
	"encoding/json"
	"net/http"
	"strings"
)

// PanelsHandler serves config/data for UI panels that don't have dedicated
// business-logic backends yet (concierge, voice, plugins, marketplace, IM).
// All endpoints return empty/default data — the frontend shows empty states.
type PanelsHandler struct{}

func NewPanelsHandler() *PanelsHandler { return &PanelsHandler{} }

func (h *PanelsHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config/concierge", h.handleConcierge)
	mux.HandleFunc("/api/config/voice", h.handleVoice)
	mux.HandleFunc("/api/config/connectors", h.handleConnectors)
	mux.HandleFunc("/api/plugins", h.handlePlugins)
	mux.HandleFunc("/api/plugins/", h.handlePluginItem)
	mux.HandleFunc("/api/marketplace", h.handleMarketplace)
	return mux
}

func cors(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

// --- Concierge ---

func (h *PanelsHandler) handleConcierge(w http.ResponseWriter, r *http.Request) {
	if cors(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, map[string]any{
			"avatar":                "🐕",
			"color":                 "#4A90D9",
			"size":                  56,
			"personality":           "",
			"greeting":              "",
			"dutyBreed":             "",
			"autoSuggestThreshold":  3,
			"proactivityLevel":      "medium",
		})
	case http.MethodPatch:
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		respondJSON(w, http.StatusOK, body)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Voice ---

func (h *PanelsHandler) handleVoice(w http.ResponseWriter, r *http.Request) {
	if cors(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, map[string]any{
			"enabled":          false,
			"ttsVoice":         "alloy",
			"ttsLang":          "zh-CN",
			"ttsSpeed":         1.0,
			"ttsRefAudio":      "",
			"sttModel":         "whisper-1",
			"sttLanguage":      "zh",
			"sttAutoTranscribe": true,
			"glossary":         []map[string]string{},
		})
	case http.MethodPatch:
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		respondJSON(w, http.StatusOK, body)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- IM Connectors ---

func (h *PanelsHandler) handleConnectors(w http.ResponseWriter, r *http.Request) {
	if cors(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, []map[string]any{})
	case http.MethodPatch:
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		respondJSON(w, http.StatusOK, body)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Plugins ---

func (h *PanelsHandler) handlePlugins(w http.ResponseWriter, r *http.Request) {
	if cors(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	respondJSON(w, http.StatusOK, []map[string]any{})
}

func (h *PanelsHandler) handlePluginItem(w http.ResponseWriter, r *http.Request) {
	if cors(w, r) {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/plugins/")
	if id == "" || r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)
	respondJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
}

// --- Marketplace ---

func (h *PanelsHandler) handleMarketplace(w http.ResponseWriter, r *http.Request) {
	if cors(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	respondJSON(w, http.StatusOK, []map[string]any{})
}
