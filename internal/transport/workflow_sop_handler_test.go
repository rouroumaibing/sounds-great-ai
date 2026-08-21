package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sounds-great-ai/internal/sop"
)

func newWorkflowSOPHandler(t *testing.T) *WorkflowSOPHandler {
	t.Helper()
	return NewWorkflowSOPHandler(sop.NewWorkflowSOP(nil)) // memory mode
}

func TestWorkflowSOPHandler_GetMissingBoard(t *testing.T) {
	h := newWorkflowSOPHandler(t)
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/backlog/FT-TEST-001/workflow-sop", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

func TestWorkflowSOPHandler_UpdateAndGet(t *testing.T) {
	h := newWorkflowSOPHandler(t)
	mux := h.Routes()

	// Create board at kickoff.
	create := `{"feature_id":"FT-TEST-001","stage":"kickoff","baton_holder":"bianmu","resume_capsule":"goal: ship; done: kickoff; focus: impl"}`
	req := httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(create))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Transition to impl with CAS on kickoff.
	update := `{"stage":"impl","baton_holder":"xigou","expected_stage":"kickoff"}`
	req = httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(update))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Read back.
	req = httptest.NewRequest("GET", "/api/backlog/FT-TEST-001/workflow-sop", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", rec.Code)
	}
	var state sop.WorkflowState
	if err := json.NewDecoder(rec.Body).Decode(&state); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if state.Stage != "impl" || state.BatonHolder != "xigou" {
		t.Errorf("state = %+v, want stage impl / baton xigou", state)
	}
	if state.FeatureID != "FT-TEST-001" {
		t.Errorf("feature_id = %q, want FT-TEST-001", state.FeatureID)
	}
}

func TestWorkflowSOPHandler_FeatureMismatch(t *testing.T) {
	h := newWorkflowSOPHandler(t)
	mux := h.Routes()

	body := `{"feature_id":"FT-OTHER","stage":"kickoff"}`
	req := httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "feature_mismatch") {
		t.Errorf("expected feature_mismatch code, got %s", rec.Body.String())
	}
}

func TestWorkflowSOPHandler_CASConflict(t *testing.T) {
	h := newWorkflowSOPHandler(t)
	mux := h.Routes()

	create := `{"feature_id":"FT-TEST-001","stage":"kickoff"}`
	req := httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(create))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Stale expected_stage -> concurrent modification.
	update := `{"stage":"impl","expected_stage":"kickoff"}`
	req = httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(update))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first update status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(update))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale CAS status = %d, want 409; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "concurrent_modification") {
		t.Errorf("expected concurrent_modification code, got %s", rec.Body.String())
	}
}

func TestWorkflowSOPHandler_InvalidTransition(t *testing.T) {
	h := newWorkflowSOPHandler(t)
	mux := h.Routes()

	create := `{"feature_id":"FT-TEST-001","stage":"kickoff"}`
	req := httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(create))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// kickoff -> merge is not a valid transition.
	update := `{"stage":"merge"}`
	req = httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(update))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid transition status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

func TestWorkflowSOPHandler_NewBoardMustStartKickoff(t *testing.T) {
	h := newWorkflowSOPHandler(t)
	mux := h.Routes()

	body := `{"feature_id":"FT-TEST-002","stage":"impl"}`
	req := httptest.NewRequest("PUT", "/api/backlog/FT-TEST-002/workflow-sop", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

func TestWorkflowSOPHandler_AttestCheck(t *testing.T) {
	h := newWorkflowSOPHandler(t)
	mux := h.Routes()

	create := `{"feature_id":"FT-TEST-001","stage":"kickoff"}`
	req := httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(create))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	attest := `{"checks":[{"name":"quality_gate_passed","status":"attested","at":"2026-08-21T00:00:00Z"}]}`
	req = httptest.NewRequest("PUT", "/api/backlog/FT-TEST-001/workflow-sop", bytes.NewBufferString(attest))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("attest status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("GET", "/api/backlog/FT-TEST-001/workflow-sop", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var state sop.WorkflowState
	if err := json.NewDecoder(rec.Body).Decode(&state); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(state.Checks) != 1 || state.Checks[0].Name != "quality_gate_passed" || state.Checks[0].Status != sop.CheckAttested {
		t.Errorf("checks = %+v, want 1 attested quality_gate_passed", state.Checks)
	}
}
