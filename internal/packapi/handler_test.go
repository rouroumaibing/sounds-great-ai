package packapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		ID:           "user-breed",
		Name:         "user-breed",
		DisplayName:  "测试犬",
		Capabilities: []pack.CapabilityBinding{{Name: "cap1", Version: "v1"}},
		Workflow: pack.WorkflowConfig{
			Steps: []pack.WorkflowStep{{ID: "s1", CapabilityRef: "cap1:v1"}},
		},
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
		ID:           "breed1",
		Capabilities: []pack.CapabilityBinding{{Name: "cap1", Version: "v1"}},
		Workflow:     pack.WorkflowConfig{Steps: []pack.WorkflowStep{{ID: "s1", CapabilityRef: "cap1:v1"}}},
		Source:       pack.BreedSourceUser,
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
		ID:           "user-breed",
		Capabilities: []pack.CapabilityBinding{{Name: "cap1", Version: "v1"}},
		Workflow:     pack.WorkflowConfig{Steps: []pack.WorkflowStep{{ID: "s1", CapabilityRef: "cap1:v1"}}},
		Source:       pack.BreedSourceUser,
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
		ID:           "sys-breed",
		Capabilities: []pack.CapabilityBinding{{Name: "cap1", Version: "v1"}},
		Workflow:     pack.WorkflowConfig{Steps: []pack.WorkflowStep{{ID: "s1", CapabilityRef: "cap1:v1"}}},
		Source:       pack.BreedSourceSystem,
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
		ID:           "no-source-breed",
		Name:         "test",
		Capabilities: []pack.CapabilityBinding{{Name: "cap1", Version: "v1"}},
		Workflow:     pack.WorkflowConfig{Steps: []pack.WorkflowStep{{ID: "s1", CapabilityRef: "cap1:v1"}}},
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
		ID:           "test-bark-breed",
		Capabilities: []pack.CapabilityBinding{{Name: "cap1", Version: "v1"}},
		Workflow:     pack.WorkflowConfig{Steps: []pack.WorkflowStep{{ID: "s1", CapabilityRef: "cap1:v1"}}},
		Source:       pack.BreedSourceSystem,
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
