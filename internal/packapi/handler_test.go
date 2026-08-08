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

func newValidBreed() pack.BreedConfig {
	return pack.BreedConfig{
		ID:               "user-breed",
		Name:             "user-breed",
		DisplayName:      "测试犬",
		DefaultVariantID: "v1",
		Variants: []pack.Variant{{
			ID:           "v1",
			ClientID:     "test",
			DefaultModel: "test-model",
		}},
		Source: pack.BreedSourceUser,
	}
}

func TestCreateBreedSuccess(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir)

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
		Variants:         []pack.Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	})

	h := NewHandler(p, dir)
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
		Variants:         []pack.Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	})

	h := NewHandler(p, dir)
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
		Variants:         []pack.Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           pack.BreedSourceSystem,
	})

	h := NewHandler(p, dir)
	req := httptest.NewRequest(http.MethodDelete, "/api/breeds/sys-breed", nil)
	req.SetPathValue("id", "sys-breed")
	w := httptest.NewRecorder()
	h.DeleteBreed(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCreateBreedRollbackOnPersistFailure(t *testing.T) {
	p := newTestPack(t)

	// Use a breedsDir that doesn't exist — saveBreedFile will fail
	h := NewHandler(p, "/nonexistent/path/that/does/not/exist")

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
	h := NewHandler(p, dir)

	// Omit source — should default to user
	cfg := pack.BreedConfig{
		ID:               "no-source-breed",
		Name:             "test",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
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
	h := NewHandler(p, dir)

	// Register a breed with the test capability
	p.Register(&pack.BreedConfig{
		ID:               "test-bark-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
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
	h := NewHandler(p, dir)

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
	h := NewHandler(p, dir)
	return h, dir
}

func TestGetTemplates(t *testing.T) {
	h, dir := setupTestPackHandler(t)

	// Write template file
	templates := `[{"id":"orchestrator","name":"Orchestrator","default_roles":["orchestrator"]}]`
	os.WriteFile(filepath.Join(dir, "cat-template.json"), []byte(templates), 0644)

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
	h, dir := setupTestPackHandler(t)

	// Create a user breed first
	cfg := pack.BreedConfig{
		ID:          "test-breed",
		Name:        "Test",
		DisplayName: "Test Breed",
		Source:      pack.BreedSourceUser,
		Variants:    []pack.Variant{{ID: "default", ClientID: "anthropic", DefaultModel: "claude-opus-4-6"}},
	}
	h.pack.Register(&cfg)
	saveBreedFile(dir, &cfg)

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
	h := NewHandler(p, dir)

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
	h := NewHandler(p, dir)

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
		Variants:         []pack.Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           pack.BreedSourceSystem,
	})

	h := NewHandler(p, dir)

	// Try to overwrite as user source
	cfg := pack.BreedConfig{
		ID:               "sys-breed",
		Name:             "overwritten",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
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
	h := NewHandler(p, dir)

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
			Variants:         []pack.Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
			Source:           pack.BreedSourceUser,
		})
	}

	h := NewHandler(p, dir)
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
	h := NewHandler(p, dir)

	req := httptest.NewRequest(http.MethodDelete, "/api/breeds/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.DeleteBreed(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestDeleteBreedRemovesFile(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)

	cfg := pack.BreedConfig{
		ID:               "file-breed",
		DefaultVariantID: "v1",
		Variants:         []pack.Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           pack.BreedSourceUser,
	}
	p.Register(&cfg)
	saveBreedFile(dir, &cfg)

	// Verify file exists
	filePath := filepath.Join(dir, "file-breed.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("breed file should exist before delete")
	}

	h := NewHandler(p, dir)
	req := httptest.NewRequest(http.MethodDelete, "/api/breeds/file-breed", nil)
	req.SetPathValue("id", "file-breed")
	w := httptest.NewRecorder()
	h.DeleteBreed(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("breed file should be deleted, stat err = %v", err)
	}
}

func TestBarkBreedInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := newTestPack(t)
	h := NewHandler(p, dir)

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
	h := NewHandler(p, dir)

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
		Variants:    []pack.Variant{{ID: "default", ClientID: "anthropic", DefaultModel: "claude-opus-4-6"}},
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
		Variants:    []pack.Variant{{ID: "default", ClientID: "anthropic", DefaultModel: "claude-opus-4-6"}},
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
	h, dir := setupTestPackHandler(t)

	cfg := pack.BreedConfig{
		ID:              "test-breed",
		Name:            "Test",
		DisplayName:     "Test Breed",
		Personality:     "calm",
		RoleDescription: "analyzer",
		Source:          pack.BreedSourceUser,
		Variants:        []pack.Variant{{ID: "default", ClientID: "anthropic", DefaultModel: "claude-opus-4-6"}},
	}
	h.pack.Register(&cfg)
	saveBreedFile(dir, &cfg)

	body, _ := json.Marshal(map[string]any{
		"display_name":     "Updated Name",
		"personality":      "energetic",
		"role_description": "reviewer",
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
	h := NewHandler(p, dir)

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
	h := NewHandler(p, dir)

	cfg := newValidBreed()
	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/breeds", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateBreed(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}
