package transport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

// SettingsHandler handles settings HTTP endpoints.
type SettingsHandler struct {
	store     settings.SettingsStore
	credStore settings.CredentialStore
	eventBus  *config.ConfigEventBus
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

// Routes returns the HTTP routes for settings.
func (h *SettingsHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/settings/accounts", h.ListAccounts)
	mux.HandleFunc("POST /api/settings/accounts", h.CreateAccount)
	mux.HandleFunc("PATCH /api/settings/accounts/{id}", h.UpdateAccount)
	mux.HandleFunc("DELETE /api/settings/accounts/{id}", h.DeleteAccount)
	mux.HandleFunc("GET /api/settings/roster", h.ListRoster)
	mux.HandleFunc("GET /api/settings/roster/{id}", h.GetRosterEntry)
	mux.HandleFunc("PATCH /api/settings/roster/{id}", h.UpdateRosterEntry)
	mux.HandleFunc("GET /api/settings/review-policy", h.GetReviewPolicy)
	mux.HandleFunc("PUT /api/settings/review-policy", h.SetReviewPolicy)
	mux.HandleFunc("GET /api/settings/config", h.ListConfig)
	mux.HandleFunc("PATCH /api/settings/config", h.UpdateConfig)
	return mux
}

func (h *SettingsHandler) ListRoster(w http.ResponseWriter, r *http.Request) {
	roster, err := h.store.GetRoster()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, roster)
}

func (h *SettingsHandler) GetRosterEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	roster, err := h.store.GetRoster()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	entry, ok := roster[id]
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "roster entry not found"})
		return
	}
	respondJSON(w, http.StatusOK, entry)
}

func (h *SettingsHandler) UpdateRosterEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	// Start from the existing entry so unspecified fields are preserved (partial update).
	entry := pack.RosterEntry{}
	if existing, err := h.store.GetRoster(); err == nil {
		if e, ok := existing[id]; ok {
			entry = e
		}
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.store.UpdateRosterEntry(id, entry); err != nil {
		if errors.Is(err, settings.ErrBreedNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "breed not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, nil)
}

func (h *SettingsHandler) GetReviewPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := h.store.GetReviewPolicy()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, policy)
}

func (h *SettingsHandler) SetReviewPolicy(w http.ResponseWriter, r *http.Request) {
	var body pack.ReviewPolicy
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.store.UpdateReviewPolicy(&body); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
	if clientID, ok := raw["client_id"].(string); ok && !settings.ValidateClientID(clientID) {
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
	// Persist the credential up-front so that creating an account with an
	// api_key results in a usable account (the metadata store only stores a
	// masked preview). If the credential cannot be persisted, roll back the
	// account to avoid an orphaned entry.
	if apiKey != "" && h.credStore != nil {
		if err := h.credStore.Set(account.ID, apiKey); err != nil {
			_ = h.store.DeleteAccount(account.ID) // best-effort rollback
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to persist credential: " + err.Error(),
			})
			return
		}
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
	if clientID, ok := updates["client_id"].(string); ok && !settings.ValidateClientID(clientID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid client_id; allowed: claude, codex, gemini, opencode, kimi"})
		return
	}
	// Handle api_key via CredentialStore: empty string clears the credential,
	// a non-empty string sets it. The api_key field is removed from the updates
	// map so it is never passed to the metadata store.
	if apiKey, ok := updates["api_key"]; ok {
		if s, ok := apiKey.(string); ok && h.credStore != nil {
			var cerr error
			if s == "" {
				cerr = h.credStore.Delete(id)
			} else {
				cerr = h.credStore.Set(id, s)
			}
			if cerr != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "failed to update credential: " + cerr.Error(),
				})
				return
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
	// Referential integrity: refuse to delete an account
	// that is referenced by one or more members via account_ref, unless the
	// caller passes force=true.
	if r.URL.Query().Get("force") != "true" {
		breeds, err := h.store.ListBreeds()
		if err == nil {
			var bound []string
			for _, b := range breeds {
				if b == nil {
					continue
				}
				for _, v := range b.Variants {
					if v.AccountRef == id {
						bound = append(bound, b.ID)
						break
					}
				}
			}
			if len(bound) > 0 {
				respondJSON(w, http.StatusConflict, map[string]any{
					"error":           "account has bound breeds; pass force=true to override",
					"bound_member_ids": bound,
				})
				return
			}
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
