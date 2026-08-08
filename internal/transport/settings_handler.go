package transport

import (
	"encoding/json"
	"net/http"
	"time"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/settings"
)

// validClientIDs is the set of allowed CLI client identifiers.
var validClientIDs = map[string]bool{
	"claude":   true,
	"codex":    true,
	"gemini":   true,
	"opencode": true,
	"kimi":     true,
}

// validateClientID returns false if clientId is non-empty and not in the allowed set.
func validateClientID(clientID string) bool {
	if clientID == "" {
		return true // optional field
	}
	return validClientIDs[clientID]
}

// SettingsHandler handles settings HTTP endpoints.
type SettingsHandler struct {
	store         settings.SettingsStore
	credStore     settings.CredentialStore
	eventBus      *config.ConfigEventBus
	breedBindings map[string][]string
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(store settings.SettingsStore) *SettingsHandler {
	return &SettingsHandler{store: store}
}

// NewSettingsHandlerWithCredentials creates a new SettingsHandler with a
// credential store and event bus for credential management and event emission.
func NewSettingsHandlerWithCredentials(store settings.SettingsStore, cred settings.CredentialStore, bus *config.ConfigEventBus) *SettingsHandler {
	return &SettingsHandler{store: store, credStore: cred, eventBus: bus}
}

// SetEventBus sets the event bus for emitting config change events.
func (h *SettingsHandler) SetEventBus(bus *config.ConfigEventBus) {
	h.eventBus = bus
}

// SetBreedBindings sets the account-to-breed bindings used for referential
// integrity checks on account deletion. The map key is an account ID and the
// value is the list of breed IDs bound to that account.
func (h *SettingsHandler) SetBreedBindings(bindings map[string][]string) {
	h.breedBindings = bindings
}

// Routes returns the HTTP routes for settings.
func (h *SettingsHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/settings/members", h.ListMembers)
	mux.HandleFunc("POST /api/settings/members", h.CreateMember)
	mux.HandleFunc("PATCH /api/settings/members/{id}", h.UpdateMember)
	mux.HandleFunc("DELETE /api/settings/members/{id}", h.DeleteMember)
	mux.HandleFunc("GET /api/settings/accounts", h.ListAccounts)
	mux.HandleFunc("POST /api/settings/accounts", h.CreateAccount)
	mux.HandleFunc("PATCH /api/settings/accounts/{id}", h.UpdateAccount)
	mux.HandleFunc("DELETE /api/settings/accounts/{id}", h.DeleteAccount)
	mux.HandleFunc("GET /api/settings/config", h.ListConfig)
	mux.HandleFunc("PATCH /api/settings/config", h.UpdateConfig)
	return mux
}

func (h *SettingsHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.store.ListMembers()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, members)
}

func (h *SettingsHandler) CreateMember(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if clientID, ok := raw["client_id"].(string); ok && !validateClientID(clientID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid client_id; allowed: claude, codex, gemini, opencode, kimi"})
		return
	}
	breedID, _ := raw["breed_id"].(string)
	displayName, _ := raw["display_name"].(string)
	role, _ := raw["role"].(string)
	enabled, _ := raw["enabled"].(bool)
	member, err := h.store.CreateMember(breedID, displayName, role, enabled)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Apply extended fields via update (skip basic fields already set).
	for _, k := range []string{"breed_id", "display_name", "role", "enabled"} {
		delete(raw, k)
	}
	if len(raw) > 0 {
		_ = h.store.UpdateMember(member.ID, raw)
	}
	respondJSON(w, http.StatusCreated, member)
}

func (h *SettingsHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if clientID, ok := updates["client_id"].(string); ok && !validateClientID(clientID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid client_id; allowed: claude, codex, gemini, opencode, kimi"})
		return
	}
	if err := h.store.UpdateMember(id, updates); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, nil)
}

func (h *SettingsHandler) DeleteMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteMember(id); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, nil)
}

func (h *SettingsHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.store.ListAccounts()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, accounts)
}

func (h *SettingsHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if clientID, ok := raw["client_id"].(string); ok && !validateClientID(clientID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid client_id; allowed: claude, codex, gemini, opencode, kimi"})
		return
	}
	provider, _ := raw["provider"].(string)
	apiKey, _ := raw["api_key"].(string)
	account, err := h.store.CreateAccount(provider, apiKey)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Apply extended fields via update (skip basic fields already set).
	for _, k := range []string{"provider", "api_key"} {
		delete(raw, k)
	}
	if len(raw) > 0 {
		_ = h.store.UpdateAccount(account.ID, raw)
	}
	respondJSON(w, http.StatusCreated, account)
}

func (h *SettingsHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if clientID, ok := updates["client_id"].(string); ok && !validateClientID(clientID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid client_id; allowed: claude, codex, gemini, opencode, kimi"})
		return
	}
	// Handle api_key via CredentialStore: empty string clears the credential,
	// a non-empty string sets it. The api_key field is removed from the updates
	// map so it is never passed to the metadata store.
	if apiKey, ok := updates["api_key"]; ok {
		if s, ok := apiKey.(string); ok && h.credStore != nil {
			if s == "" {
				_ = h.credStore.Delete(id)
			} else {
				_ = h.credStore.Set(id, s)
			}
		}
		delete(updates, "api_key")
	}
	if err := h.store.UpdateAccount(id, updates); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	h.emitEvent("account-config", "key", []string{id})
	respondJSON(w, http.StatusOK, nil)
}

func (h *SettingsHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Referential integrity: refuse to delete an account that has breeds bound
	// to it unless the caller passes force=true.
	if boundBreedIDs, ok := h.breedBindings[id]; ok && len(boundBreedIDs) > 0 {
		if r.URL.Query().Get("force") != "true" {
			respondJSON(w, http.StatusConflict, map[string]any{
				"error":           "account has bound breeds; pass force=true to override",
				"bound_breed_ids": boundBreedIDs,
			})
			return
		}
	}
	if err := h.store.DeleteAccount(id); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	// Best-effort credential cleanup.
	if h.credStore != nil {
		_ = h.credStore.Delete(id)
	}
	h.emitEvent("account-config", "key", []string{id})
	respondJSON(w, http.StatusOK, nil)
}

func (h *SettingsHandler) ListConfig(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.ListConfig()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, configs)
}

func (h *SettingsHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.store.UpdateConfig(body.Key, body.Value); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	h.emitEvent("system-config", "key", []string{body.Key})
	respondJSON(w, http.StatusOK, nil)
}

// emitEvent publishes a ConfigEvent to the event bus when one is configured.
func (h *SettingsHandler) emitEvent(source, scope string, keys []string) {
	if h.eventBus != nil {
		h.eventBus.Emit(config.ConfigEvent{
			Source:      source,
			Scope:       scope,
			ChangedKeys: keys,
			Timestamp:   time.Now(),
		})
	}
}
