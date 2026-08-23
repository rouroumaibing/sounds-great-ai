package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"sounds-great-ai/internal/marketplace"
	"sounds-great-ai/internal/plugins"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/skills"
	"sounds-great-ai/pkg/pack"
)

// PluginsHandler serves the plugin lifecycle (panels-roadmap P3) and the
// marketplace surface (P4).
//
// Install  = unpack + register (disabled); skills land in the pending
//
//	security review via the plugin source dir.
//
// Enable   = gated on every shipped skill being approved, then registers
//
//	breeds through the settings-store validation channel
//	(source="plugin"). Disable reverses both.
//
// Uninstall= removes sources, deletes plugin breeds, drops the payload.
//
// Marketplace (P4) = index browsing (cached) + verified install: downloads
// the archive, checks sha256 + an ed25519 signature from the trusted
// publisher keys (SG_MARKETPLACE_PUBKEYS; no keys ⇒ installs fail closed),
// then feeds the verified bytes to the P3 installer.
type PluginsHandler struct {
	svc    *plugins.Service
	skills *skills.SkillManager   // nil = skills integration unavailable
	store  settings.SettingsStore // nil = breed registration unavailable
	mkt    *marketplace.Client    // nil = marketplace disabled
}

func NewPluginsHandler(svc *plugins.Service, sk *skills.SkillManager, store settings.SettingsStore, mkt *marketplace.Client) *PluginsHandler {
	if mkt == nil {
		mkt = marketplace.NewClient("")
	}
	return &PluginsHandler{svc: svc, skills: sk, store: store, mkt: mkt}
}

func (h *PluginsHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/plugins", h.handleCollection)
	mux.HandleFunc("/api/plugins/install", h.install)
	mux.HandleFunc("/api/plugins/", h.handleItem)
	mux.HandleFunc("/api/marketplace", h.handleMarketplace)
	mux.HandleFunc("/api/marketplace/install", h.marketplaceInstall)
	return mux
}

func (h *PluginsHandler) handleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		views, err := h.svc.List()
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, views)
	case http.MethodPost: // install (multipart)
		h.install(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PluginsHandler) install(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart/form-data with a 'package' file required"})
		return
	}
	// Cap the upload at the same budget as the unzip cap.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	file, _, err := r.FormFile("package")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "missing 'package' file"})
		return
	}
	defer file.Close()

	view, err := h.svc.Install(file.(plugins.SeekableReader))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Feed shipped skills into the pending-security pipeline right away so
	// the operator can review before enabling. Registration is disabled by
	// design until approved.
	notes := []string{}
	if h.skills != nil && len(view.SkillIDs) > 0 {
		h.skills.AddSource(h.svc.SkillsDir(view.ID), "plugin")
		if err := h.skills.Scan(); err != nil {
			notes = append(notes, "skills scan failed: "+err.Error())
		} else {
			notes = append(notes, "skills registered as pending security review")
		}
	}
	if len(view.BreedIDs) > 0 {
		notes = append(notes, "breeds register on first enable (after skills approval)")
	}
	respondJSON(w, http.StatusCreated, map[string]any{"plugin": view, "notes": notes})
}

func (h *PluginsHandler) handleItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/plugins/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		h.set_enabled(w, r, id)
	case http.MethodDelete:
		h.uninstall(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PluginsHandler) set_enabled(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be {\"enabled\": bool}"})
		return
	}
	view, err := h.svc.Get(id)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	if *body.Enabled {
		if err := h.enable(view); err != nil {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
	} else {
		h.disable(view)
	}
	updated, _ := h.svc.Get(id)
	respondJSON(w, http.StatusOK, updated)
}

// enable enforces the approval gate and registers plugin breeds.
func (h *PluginsHandler) enable(view plugins.View) error {
	if h.skills != nil {
		dir := h.svc.SkillsDir(view.ID)
		h.skills.AddSource(dir, "plugin")
		if err := h.skills.Scan(); err != nil {
			return err
		}
		// Gate: every skill shipped by this plugin must be approved. A skill
		// still pending/quarantined blocks the whole plugin (fail closed).
		var blocked []string
		for _, sk := range h.skills.All() {
			if sk.Source != "plugin" || !strings.HasPrefix(sk.FilePath, dir+string(filepath.Separator)) {
				continue
			}
			if st := h.skills.Security().StateOf(sk.ID); st == nil || string(st.Status) != "approved" {
				blocked = append(blocked, sk.ID+" ("+statusOr(st)+")")
			}
		}
		if len(blocked) > 0 {
			sort.Strings(blocked)
			return &errApprovalGate{blocked}
		}
	}
	// Register breeds through the existing validation channel. A breed id
	// already in the catalog is updated in place (re-enable path).
	if h.store != nil {
		raws, _, errs := h.svc.BreedConfigs(view.ID)
		for _, e := range errs {
			return &errBreedLoad{e}
		}
		for _, raw := range raws {
			var cfg pack.BreedConfig
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return &errBreedLoad{err.Error()}
			}
			cfg.Source = pack.BreedSourcePlugin
			if cfg.Enabled {
				if err := h.store.UpdateBreed(cfg.ID, &cfg); err == nil {
					continue
				}
			}
			if err := h.store.CreateBreed(&cfg); err != nil {
				return err
			}
		}
	}
	return h.svc.SetEnabled(view.ID, true)
}

// disable unmounts the skills source and turns its breeds off (definitions
// stay — a re-enable flips them back on).
func (h *PluginsHandler) disable(view plugins.View) {
	if h.skills != nil {
		h.skills.RemoveSource(h.svc.SkillsDir(view.ID))
		_ = h.skills.Scan()
	}
	if h.store != nil {
		raws, _, _ := h.svc.BreedConfigs(view.ID)
		for _, raw := range raws {
			var probe struct {
				ID      string `json:"id"`
				Enabled bool   `json:"enabled"`
			}
			if json.Unmarshal(raw, &probe) != nil || probe.ID == "" {
				continue
			}
			var cfg pack.BreedConfig
			if json.Unmarshal(raw, &cfg) != nil {
				continue
			}
			cfg.Enabled = false
			_ = h.store.UpdateBreed(probe.ID, &cfg)
		}
	}
	_ = h.svc.SetEnabled(view.ID, false)
}

func (h *PluginsHandler) uninstall(w http.ResponseWriter, r *http.Request, id string) {
	view, err := h.svc.Get(id)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if view.Enabled {
		h.disable(view)
	}
	// Plugin breeds are removed with the package (they only exist because of it).
	if h.store != nil {
		_, ids, _ := h.svc.BreedConfigs(id)
		for _, bid := range ids {
			if bid == "" {
				continue
			}
			_ = h.store.DeleteBreed(bid)
		}
	}
	if err := h.svc.Uninstall(id); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// marketplaceItem is the enriched read model: index entry + installed flag.
type marketplaceItem struct {
	marketplace.Item
	Installed bool `json:"installed"`
}

func (h *PluginsHandler) handleMarketplace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, staleNote, err := h.mkt.List(r.URL.Query().Get("query"))
	if err != nil {
		// Disabled (no index URL) or unreachable with no cache: honest state.
		respondJSON(w, http.StatusServiceUnavailable, map[string]any{
			"plugins": []marketplaceItem{},
			"note":    err.Error(),
		})
		return
	}
	installed := map[string]bool{}
	if views, err := h.svc.List(); err == nil {
		for _, v := range views {
			installed[v.ID] = true
		}
	}
	out := make([]marketplaceItem, 0, len(items))
	for _, it := range items {
		out = append(out, marketplaceItem{Item: it, Installed: installed[it.ID]})
	}
	body := map[string]any{"plugins": out}
	if staleNote != "" {
		body["note"] = "serving cached index: " + staleNote
	}
	respondJSON(w, http.StatusOK, body)
}

// marketplaceInstall: POST {id} → find in index → download → verify
// (sha256 + ed25519 vs trusted keys) → P3 install.
func (h *PluginsHandler) marketplaceInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be {\"id\": \"plugin-id\"}"})
		return
	}
	item, ok, err := h.mkt.Find(body.ID)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found in marketplace index"})
		return
	}

	data, err := h.mkt.Download(item)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	// Signature gate BEFORE the archive touches the installer's zip parser:
	// unverified bytes never get interpreted as an archive.
	keys := marketplace.TrustedKeys()
	if err := marketplace.Verify(data, item, keys); err != nil {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	// Verified bytes → same P3 install path (manifest checks, zip-slip
	// defenses, disabled-by-default registration).
	view, err := h.svc.Install(bytes.NewReader(data))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if h.skills != nil && len(view.SkillIDs) > 0 {
		h.skills.AddSource(h.svc.SkillsDir(view.ID), "plugin")
		_ = h.skills.Scan()
	}
	respondJSON(w, http.StatusCreated, map[string]any{"plugin": view})
}

// typed errors so the wire layer can render the gate明细 correctly
type errApprovalGate struct{ blocked []string }

func (e *errApprovalGate) Error() string {
	return "skills not approved: " + strings.Join(e.blocked, ", ")
}

type errBreedLoad struct{ detail string }

func (e *errBreedLoad) Error() string { return "breed payload invalid: " + e.detail }

func statusOr(st *skills.SkillSecurityState) string {
	if st == nil {
		return "pending"
	}
	return string(st.Status)
}
