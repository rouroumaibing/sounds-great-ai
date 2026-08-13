package packapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

// testCapability 测试用 mock capability
type testCapability struct {
	name    string
	version string
}

func (c *testCapability) Name() string    { return c.name }
func (c *testCapability) Version() string { return c.version }
func (c *testCapability) Init(ctx context.Context) error { return nil }
func (c *testCapability) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	return &pack.TaskOutput{Approved: true}, nil
}
func (c *testCapability) Health() error { return nil }
func (c *testCapability) Close() error  { return nil }

func newTestPack(t *testing.T) *pack.Pack {
	t.Helper()
	p := pack.New("test")
	if err := p.RegisterCapability(&testCapability{name: "cap1", version: "v1"}); err != nil {
		t.Fatalf("RegisterCapability: %v", err)
	}
	return p
}

// newTestStore returns a fresh, isolated file-backed SettingsStore rooted in a
// temp dir. Each call gets its own accounts.json + dog-catalog.json so tests
// never collide on disk.
func newTestStore(t *testing.T) settings.SettingsStore {
	t.Helper()
	dir := t.TempDir()
	return settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)
}

// failingCreateStore wraps a real SettingsStore but makes CreateBreed always
// fail, so we can exercise the rollback path in CreateBreed (persist failure
// must not leave the breed registered in the in-memory pack).
type failingCreateStore struct {
	settings.SettingsStore
}

func (f *failingCreateStore) CreateBreed(*pack.BreedConfig) error {
	return fmt.Errorf("simulated persist failure")
}

func newValidBreed() pack.BreedConfig {
	return pack.BreedConfig{
		ID:               "user-breed",
		Name:             "user-breed",
		DisplayName:      "测试犬",
		DefaultVariantID: "v1",
		Variants: []pack.Variant{{
			ID:           "v1",
			ClientID:     "claude",
			DefaultModel: "test-model",
		}},
		Source: pack.BreedSourceUser,
	}
}

func TestCreateBreedSuccess(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	cfg := newValidBreed()
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	var result pack.BreedConfig
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.ID != cfg.ID {
		t.Errorf("ID = %q, want %q", result.ID, cfg.ID)
	}
}

func TestListBreeds(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)

	// Register a breed
	p.Register(&pack.BreedConfig{
		ID:               "breed1",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	})

	h := NewHandler(p, dir, newTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/api/breeds", nil)
	w := httptest.NewRecorder()
	h.ListBreeds(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var breeds []*pack.BreedConfig
	json.Unmarshal(w.Body.Bytes(), &breeds)
	if len(breeds) != 1 {
		t.Errorf("breeds len = %d, want 1", len(breeds))
	}
}

func TestDeleteUserBreedSuccess(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)

	p.Register(&pack.BreedConfig{
		ID:               "user-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	})

	h := NewHandler(p, dir, newTestStore(t))
	req := httptest.NewRequest(http.MethodDelete, "/api/breeds/user-breed", nil)
	req.SetPathValue("id", "user-breed")
	w := httptest.NewRecorder()
	h.DeleteBreed(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if len(p.List()) != 0 {
		t.Error("breed should be deleted")
	}
}

func TestDeleteSystemBreedForbidden(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)

	p.Register(&pack.BreedConfig{
		ID:               "sys-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
		Source:           pack.BreedSourceSystem,
	})

	h := NewHandler(p, dir, newTestStore(t))
	req := httptest.NewRequest(http.MethodDelete, "/api/breeds/sys-breed", nil)
	req.SetPathValue("id", "sys-breed")
	w := httptest.NewRecorder()
	h.DeleteBreed(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCreateBreedRollbackOnPersistFailure(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)

	// Persist always fails → CreateBreed must roll back the in-memory registration.
	h := NewHandler(p, dir, &failingCreateStore{newTestStore(t)})

	cfg := newValidBreed()
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}

	// Verify breed was rolled back (not in pack)
	for _, b := range p.List() {
		if b.ID == "user-breed" {
			t.Error("breed should have been rolled back after persist failure")
		}
	}
}

func TestCreateBreedDefaultSourceUser(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	// Omit source — should default to user
	cfg := pack.BreedConfig{
		ID:               "no-source-breed",
		Name:             "test",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
	}
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	var result pack.BreedConfig
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Source != pack.BreedSourceUser {
		t.Errorf("Source = %q, want %q", result.Source, pack.BreedSourceUser)
	}
}

func TestBarkBreedSuccess(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	// Register a breed with the test capability
	p.Register(&pack.BreedConfig{
		ID:               "test-bark-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
		Source:           pack.BreedSourceSystem,
	})

	body, _ := json.Marshal(map[string]string{"query": "test query"})
	req := httptest.NewRequest(http.MethodPost, "/api/breeds/test-bark-breed/bark", bytes.NewReader(body))
	req.SetPathValue("id", "test-bark-breed")
	w := httptest.NewRecorder()
	h.BarkBreed(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestBarkBreedNotFound(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	body, _ := json.Marshal(map[string]string{"query": "test"})
	req := httptest.NewRequest(http.MethodPost, "/api/breeds/nonexistent/bark", bytes.NewReader(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.BarkBreed(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func setupTestPackHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	dir := t.TempDir()
	p := pack.New("test")
	h := NewHandler(p, dir, newTestStore(t))
	return h, dir
}

func TestGetTemplates(t *testing.T) {
	h, dir := setupTestPackHandler(t)

	// Write template file (consolidated format)
	templates := `{"version":2,"role_templates":[{"id":"orchestrator","name":"Orchestrator"}],"breeds":[]}`
	os.WriteFile(filepath.Join(dir, "dog-template.json"), []byte(templates), 0644)

	mux := h.Routes()
	req := httptest.NewRequest("GET", "/api/breeds/templates", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var result []map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if len(result) != 1 || result[0]["id"] != "orchestrator" {
		t.Fatalf("got %v, want 1 template with id=orchestrator", result)
	}
}

func TestGetTemplates_Fallback(t *testing.T) {
	h, _ := setupTestPackHandler(t)
	// No template file → fallback to empty array
	mux := h.Routes()
	req := httptest.NewRequest("GET", "/api/breeds/templates", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result []any
	json.NewDecoder(rec.Body).Decode(&result)
	if len(result) != 0 {
		t.Fatalf("got %v, want empty array", result)
	}
}

func TestUpdateBreed(t *testing.T) {
	h, _ := setupTestPackHandler(t)

	// Create a user breed first
	cfg := pack.BreedConfig{
		ID:          "test-breed",
		Name:        "Test",
		DisplayName: "Test Breed",
		Source:      pack.BreedSourceUser,
		Variants:    []pack.Variant{{ID: "default", ClientID: "claude", DefaultModel: "claude-opus-4-6"}},
	}
	h.pack.Register(&cfg)
	h.persistBreed(&cfg)

	// PATCH
	body, _ := json.Marshal(map[string]any{"display_name": "Updated Breed"})
	req := httptest.NewRequest("PATCH", "/api/breeds/test-breed", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux := h.Routes()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var result pack.BreedConfig
	json.NewDecoder(rec.Body).Decode(&result)
	if result.DisplayName != "Updated Breed" {
		t.Fatalf("DisplayName = %q, want %q", result.DisplayName, "Updated Breed")
	}
}

func TestGetBreedStatus(t *testing.T) {
	h, _ := setupTestPackHandler(t)
	cfg := pack.BreedConfig{ID: "test-breed", Name: "Test", DisplayName: "Test", Source: pack.BreedSourceUser, Variants: []pack.Variant{{ID: "default"}}}
	h.pack.Register(&cfg)

	mux := h.Routes()
	req := httptest.NewRequest("GET", "/api/breeds/test-breed/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["status"] != "idle" {
		t.Fatalf("status = %v, want idle", result["status"])
	}
}

// ---------------------------------------------------------------------------
// Edge case tests
// ---------------------------------------------------------------------------

func TestCreateBreedInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateBreedEmptyBody(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(nil))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateBreedOverwriteSystemBreed(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)

	// Register a system breed first
	p.Register(&pack.BreedConfig{
		ID:               "sys-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
		Source:           pack.BreedSourceSystem,
	})

	h := NewHandler(p, dir, newTestStore(t))

	// Try to overwrite as user source
	cfg := pack.BreedConfig{
		ID:               "sys-breed",
		Name:             "overwritten",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	}
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (Validate should reject)", w.Code, http.StatusBadRequest)
	}
}

func TestListBreedsEmpty(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	req := httptest.NewRequest(http.MethodGet, "/api/breeds", nil)
	w := httptest.NewRecorder()
	h.ListBreeds(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var breeds []*pack.BreedConfig
	json.Unmarshal(w.Body.Bytes(), &breeds)
	if len(breeds) != 0 {
		t.Errorf("breeds len = %d, want 0", len(breeds))
	}
}

func TestListBreedsMultiple(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)

	for i := 0; i < 3; i++ {
		p.Register(&pack.BreedConfig{
			ID:               fmt.Sprintf("breed-%d", i),
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
			Source:           pack.BreedSourceUser,
		})
	}

	h := NewHandler(p, dir, newTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/api/breeds", nil)
	w := httptest.NewRecorder()
	h.ListBreeds(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var breeds []*pack.BreedConfig
	json.Unmarshal(w.Body.Bytes(), &breeds)
	if len(breeds) != 3 {
		t.Errorf("breeds len = %d, want 3", len(breeds))
	}
}

func TestDeleteBreedNotFound(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	req := httptest.NewRequest(http.MethodDelete, "/api/breeds/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.DeleteBreed(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestDeleteBreedRemovesFromCatalog(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	store := newTestStore(t)

	cfg := pack.BreedConfig{
		ID:               "file-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	}
	p.Register(&cfg)
	h := NewHandler(p, dir, store)
	if err := h.persistBreed(&cfg); err != nil {
		t.Fatalf("persistBreed: %v", err)
	}

	// Before delete: the runtime catalog must hold the breed plus a roster entry.
	breeds, err := store.ListBreeds()
	if err != nil {
		t.Fatalf("ListBreeds: %v", err)
	}
	if len(breeds) != 1 || breeds[0].ID != "file-breed" {
		t.Fatalf("expected file-breed in catalog before delete, got %v", breeds)
	}
	roster, err := store.GetRoster()
	if err != nil {
		t.Fatalf("GetRoster: %v", err)
	}
	if _, ok := roster["file-breed"]; !ok {
		t.Fatalf("expected roster entry for file-breed before delete")
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/breeds/file-breed", nil)
	req.SetPathValue("id", "file-breed")
	w := httptest.NewRecorder()
	h.DeleteBreed(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	breeds, _ = store.ListBreeds()
	for _, b := range breeds {
		if b.ID == "file-breed" {
			t.Errorf("breed file-breed should be removed from catalog, got %v", breeds)
		}
	}
	roster, _ = store.GetRoster()
	if _, ok := roster["file-breed"]; ok {
		t.Errorf("roster entry for file-breed should be removed")
	}
}

// TestCreateBreedDuplicateAlias verifies that two breeds cannot share a
// mention pattern (alias) — the handler must return 400 on conflict.
func TestCreateBreedDuplicateAlias(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)

	// An existing breed owns the alias "@alpha".
	p.Register(&pack.BreedConfig{
		ID:               "existing",
		DefaultVariantID: "v1",
		MentionPatterns:  []string{"@alpha"},
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	})

	h := NewHandler(p, dir, newTestStore(t))

	cfg := pack.BreedConfig{
		ID:               "new-breed",
		DefaultVariantID: "v1",
		MentionPatterns:  []string{"@ALPHA"}, // case-insensitive collision
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	}
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (duplicate alias), body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// TestCreateBreedInvalidClientID verifies the client_id whitelist guard (D5).
func TestCreateBreedInvalidClientID(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	cfg := pack.BreedConfig{
		ID:               "bad-client-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "badclient", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	}
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (invalid client_id), body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// TestCreateBreedAccountRefNotFound verifies a non-existent account_ref is
// rejected. Empty refs and built-in OAuth refs (claude/codex/...) are allowed.
func TestCreateBreedAccountRefNotFound(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	cfg := pack.BreedConfig{
		ID:               "missing-account-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "claude", AccountRef: "does-not-exist", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	}
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (account_ref not found), body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// TestCreateBreedEmptyClientIDAllowed verifies that an empty client_id
// (generic api_key account, per decision D5) is accepted.
func TestCreateBreedEmptyClientIDAllowed(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	cfg := pack.BreedConfig{
		ID:               "generic-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	}
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (empty client_id allowed), body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestBarkBreedInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	req := httptest.NewRequest(http.MethodPost, "/api/breeds/some/bark", bytes.NewReader([]byte("not json")))
	req.SetPathValue("id", "some")
	w := httptest.NewRecorder()
	h.BarkBreed(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBarkBreedEmptyBody(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	req := httptest.NewRequest(http.MethodPost, "/api/breeds/some/bark", bytes.NewReader(nil))
	req.SetPathValue("id", "some")
	w := httptest.NewRecorder()
	h.BarkBreed(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateBreedNotFound(t *testing.T) {
	h, _ := setupTestPackHandler(t)

	body, _ := json.Marshal(map[string]any{"display_name": "Updated"})
	req := httptest.NewRequest("PATCH", "/api/breeds/nonexistent", bytes.NewReader(body))
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()
	h.UpdateBreed(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdateBreedSystemBreedForbidden(t *testing.T) {
	h, _ := setupTestPackHandler(t)

	cfg := pack.BreedConfig{
		ID:          "sys-breed",
		Name:        "System",
		DisplayName: "System Breed",
		Source:      pack.BreedSourceSystem,
		Variants:    []pack.Variant{{ID: "default", ClientID: "claude", DefaultModel: "claude-opus-4-6"}},
	}
	h.pack.Register(&cfg)

	body, _ := json.Marshal(map[string]any{"display_name": "Hacked"})
	req := httptest.NewRequest("PATCH", "/api/breeds/sys-breed", bytes.NewReader(body))
	req.SetPathValue("id", "sys-breed")
	rec := httptest.NewRecorder()
	h.UpdateBreed(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestUpdateBreedInvalidJSON(t *testing.T) {
	h, _ := setupTestPackHandler(t)

	cfg := pack.BreedConfig{
		ID:          "test-breed",
		Name:        "Test",
		DisplayName: "Test Breed",
		Source:      pack.BreedSourceUser,
		Variants:    []pack.Variant{{ID: "default", ClientID: "claude", DefaultModel: "claude-opus-4-6"}},
	}
	h.pack.Register(&cfg)

	req := httptest.NewRequest("PATCH", "/api/breeds/test-breed", bytes.NewReader([]byte("not json")))
	req.SetPathValue("id", "test-breed")
	rec := httptest.NewRecorder()
	h.UpdateBreed(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateBreedMultipleFields(t *testing.T) {
	h, _ := setupTestPackHandler(t)

	cfg := pack.BreedConfig{
		ID:              "test-breed",
		Name:            "Test",
		DisplayName:     "Test Breed",
		Personality:     "calm",
		RoleDescription: "analyzer",
		Source:          pack.BreedSourceUser,
		Variants:        []pack.Variant{{ID: "default", ClientID: "claude", DefaultModel: "claude-opus-4-6"}},
	}
	h.pack.Register(&cfg)
	h.persistBreed(&cfg)

	body, _ := json.Marshal(map[string]any{
		"display_name":     "Updated Name",
		"personality":      "energetic",
		"role_description": "reviewer",
		"color":            map[string]any{"primary": "#ff0000", "secondary": "#00ff00"},
		"nickname":         "tester",
		"caution":          "do not edit prod",
		"restrictions":     []string{"no-code"},
	})
	req := httptest.NewRequest("PATCH", "/api/breeds/test-breed", bytes.NewReader(body))
	req.SetPathValue("id", "test-breed")
	rec := httptest.NewRecorder()
	h.UpdateBreed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var result pack.BreedConfig
	json.NewDecoder(rec.Body).Decode(&result)
	if result.DisplayName != "Updated Name" {
		t.Errorf("DisplayName = %q, want %q", result.DisplayName, "Updated Name")
	}
	if result.Personality != "energetic" {
		t.Errorf("Personality = %q, want %q", result.Personality, "energetic")
	}
	if result.RoleDescription != "reviewer" {
		t.Errorf("RoleDescription = %q, want %q", result.RoleDescription, "reviewer")
	}
	if result.Color.Primary != "#ff0000" || result.Color.Secondary != "#00ff00" {
		t.Errorf("Color = %+v, want primary #ff0000 / secondary #00ff00", result.Color)
	}
	if result.Nickname != "tester" {
		t.Errorf("Nickname = %q, want %q", result.Nickname, "tester")
	}
	if result.Caution != "do not edit prod" {
		t.Errorf("Caution = %q, want %q", result.Caution, "do not edit prod")
	}
	if len(result.Restrictions) != 1 || result.Restrictions[0] != "no-code" {
		t.Errorf("Restrictions = %v, want [no-code]", result.Restrictions)
	}
}

func TestGetBreedStatusNotFound(t *testing.T) {
	h, _ := setupTestPackHandler(t)

	req := httptest.NewRequest("GET", "/api/breeds/nonexistent/status", nil)
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()
	h.GetBreedStatus(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestErrorResponseFormat(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	var errResp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if errResp["error"] == "" {
		t.Error("expected non-empty error message in response")
	}
}

func TestCreateBreedContentTypeJSON(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir, newTestStore(t))

	cfg := newValidBreed()
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}
