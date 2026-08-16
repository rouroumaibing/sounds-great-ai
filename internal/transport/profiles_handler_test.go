package transport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

func newProfilesServer(t *testing.T) *httptest.Server {
	t.Helper()
	profiles := settings.NewProfileRepository(t.TempDir(), "operator")
	cont := settings.NewContinuityStore(t.TempDir())
	ev := memory.NewEvidenceStore()
	h := NewProfilesHandler(profiles, cont, ev, nil, "", nil)
	return httptest.NewServer(h.Routes())
}

func doReq(t *testing.T, method, url, body string) (int, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func TestProfilesHandler_FullLifecycle(t *testing.T) {
	srv := newProfilesServer(t)
	defer srv.Close()
	base := srv.URL + "/api/profiles"

	// 1. PUT an active capsule.
	code, _ := doReq(t, http.MethodPut, base+"/fam", `{"body":"operator likes terse","source_ref":"operator:manual"}`)
	if code != http.StatusOK {
		t.Fatalf("PUT active: status %d", code)
	}

	// 2. GET shows it with no pending proposal.
	code, data := doReq(t, http.MethodGet, base+"/fam", "")
	if code != http.StatusOK {
		t.Fatalf("GET: status %d", code)
	}
	var got struct {
		Body           string `json:"body"`
		PendingProposal bool   `json:"pending_proposal"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal GET: %v", err)
	}
	if got.Body != "operator likes terse" || got.PendingProposal {
		t.Errorf("GET mismatch: %+v", got)
	}

	// 3. Propose a candidate.
	code, _ = doReq(t, http.MethodPost, base+"/fam/propose", `{"body":"operator likes diagrams too"}`)
	if code != http.StatusCreated {
		t.Fatalf("propose: status %d", code)
	}

	// 4. GET now reports a pending proposal.
	_, data = doReq(t, http.MethodGet, base+"/fam", "")
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.PendingProposal {
		t.Error("expected pending_proposal=true after propose")
	}

	// 5. Approve promotes the proposal.
	code, data = doReq(t, http.MethodPost, base+"/fam/proposal/approve", "")
	if code != http.StatusOK {
		t.Fatalf("approve: status %d", code)
	}
	var approved struct {
		Body          string `json:"body"`
		EvalApprovals int    `json:"eval_approvals"`
	}
	if err := json.Unmarshal(data, &approved); err != nil {
		t.Fatalf("unmarshal approve: %v", err)
	}
	if approved.Body != "operator likes diagrams too" || approved.EvalApprovals != 1 {
		t.Errorf("approve mismatch: %+v", approved)
	}

	// 6. After approval, no pending proposal remains. Use a fresh struct so an
	// omitted (false) pending_proposal does not inherit a prior unmarshal value.
	var after struct {
		Body           string `json:"body"`
		PendingProposal bool   `json:"pending_proposal"`
	}
	_, data = doReq(t, http.MethodGet, base+"/fam", "")
	if err := json.Unmarshal(data, &after); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if after.PendingProposal {
		t.Error("pending proposal should be cleared after approval")
	}

	// 7. Approving again with nothing pending is a conflict.
	code, _ = doReq(t, http.MethodPost, base+"/fam/proposal/approve", "")
	if code != http.StatusConflict {
		t.Errorf("repeat approve status = %d, want 409", code)
	}
}

func TestProfilesHandler_BodyCap(t *testing.T) {
	srv := newProfilesServer(t)
	defer srv.Close()
	tooLong := strings.Repeat("x", settings.MaxCapsuleBodyLen+1)
	code, data := doReq(t, http.MethodPut, srv.URL+"/api/profiles/fam", `{"body":"`+tooLong+`"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for over-long body, got %d", code)
	}
	if !strings.Contains(string(data), settings.ErrCapsuleTooLong.Error()) {
		t.Errorf("error body should mention cap: %s", data)
	}
}

func TestProfilesHandler_Distill(t *testing.T) {
	srv := newProfilesServer(t)
	defer srv.Close()
	// Seed an evidence record tagged with the relationship key.
	profiles := settings.NewProfileRepository(t.TempDir(), "operator")
	cont := settings.NewContinuityStore(t.TempDir())
	ev := memory.NewEvidenceStore()
	if _, err := ev.AddEvidence("t1", "correction", "operator corrected pacing", "operator said slow down on fam", []string{"fam"}); err != nil {
		t.Fatalf("AddEvidence: %v", err)
	}
	h := NewProfilesHandler(profiles, cont, ev, nil, "", nil)
	dsrv := httptest.NewServer(h.Routes())
	defer dsrv.Close()

	code, data := doReq(t, http.MethodPost, dsrv.URL+"/api/profiles/fam/distill", "")
	if code != http.StatusOK {
		t.Fatalf("distill: status %d", code)
	}
	var out struct {
		EvidenceCount int `json:"evidence_count"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal distill: %v", err)
	}
	if out.EvidenceCount != 1 {
		t.Errorf("evidence_count = %d, want 1", out.EvidenceCount)
	}
}

// TestProfilesHandler_DistillAgentRequiresSessionOrClient locks in the
// homologous behavior: there is NO hardcoded default distiller dog.
// The distiller must be derived from the current session (?session_id) or an
// explicit ?client_id; otherwise the endpoint refuses (400).
func TestProfilesHandler_DistillAgentRequiresSessionOrClient(t *testing.T) {
	srv := newProfilesServer(t)
	defer srv.Close()

	// Neither session_id nor client_id -> refuse (no default dog).
	code, _ := doReq(t, http.MethodPost, srv.URL+"/api/profiles/fam/distill/agent", "")
	if code != http.StatusBadRequest {
		t.Fatalf("distill/agent without session_id/client_id: status %d, want 400", code)
	}

	// Unknown session_id -> refuse (session has no associated dog).
	code, _ = doReq(t, http.MethodPost, srv.URL+"/api/profiles/fam/distill/agent?session_id=ghost", "")
	if code != http.StatusBadRequest {
		t.Fatalf("distill/agent with unknown session_id: status %d, want 400", code)
	}
}

// TestAutoDistillSession verifies the on-session-seal autonomous distill: with
// a breed relationship key and matching evidence, a pending capsule proposal is
// written (never auto-applied), and a second seal does not pile up a draft.
func TestAutoDistillSession(t *testing.T) {
	profiles := settings.NewProfileRepository(t.TempDir(), "operator")
	cont := settings.NewContinuityStore(t.TempDir())
	ev := memory.NewEvidenceStore()
	pl := &platform.Platform{Breeds: map[string]*pack.BreedConfig{
		"bianmu": {RelationshipKey: "operator:bianmu"},
	}}
	h := NewProfilesHandler(profiles, cont, ev, nil, "", pl)

	// No evidence -> no proposal written.
	h.AutoDistillSession(context.Background(), "s1", "bianmu")
	if pending, _ := profiles.HasProposal("operator:bianmu"); pending {
		t.Fatal("expected no proposal without evidence")
	}

	// Evidence matching the relationship key -> a pending proposal is written.
	if _, err := ev.AddEvidence("t1", "note", "operator:bianmu prefers terse replies", "content about operator:bianmu", []string{"pref"}); err != nil {
		t.Fatalf("add evidence: %v", err)
	}
	h.AutoDistillSession(context.Background(), "s2", "bianmu")
	c, ok, err := profiles.ReadProposal("operator:bianmu")
	if err != nil || !ok {
		t.Fatalf("expected pending proposal, ok=%v err=%v", ok, err)
	}
	if !strings.Contains(c.Body, "自动蒸馏草稿") {
		t.Fatalf("proposal body unexpected: %q", c.Body)
	}

	// A second seal with a pending proposal must not pile up another draft.
	h.AutoDistillSession(context.Background(), "s3", "bianmu")
	if pending, _ := profiles.HasProposal("operator:bianmu"); !pending {
		t.Fatal("expected proposal still pending after repeat seal")
	}
}

// TestAutoDistillSessionDisabled verifies the env gate and nil-platform no-op.
func TestAutoDistillSessionDisabled(t *testing.T) {
	profiles := settings.NewProfileRepository(t.TempDir(), "operator")
	cont := settings.NewContinuityStore(t.TempDir())
	ev := memory.NewEvidenceStore()
	pl := &platform.Platform{Breeds: map[string]*pack.BreedConfig{
		"bianmu": {RelationshipKey: "operator:bianmu"},
	}}

	// Env disabled: SG_AUTO_DISTILL_ON_SEAL=false suppresses the distill.
	t.Setenv("SG_AUTO_DISTILL_ON_SEAL", "false")
	if _, err := ev.AddEvidence("t1", "note", "operator:bianmu x", "y", nil); err != nil {
		t.Fatalf("add evidence: %v", err)
	}
	h := NewProfilesHandler(profiles, cont, ev, nil, "", pl)
	h.AutoDistillSession(context.Background(), "s1", "bianmu")
	if pending, _ := profiles.HasProposal("operator:bianmu"); pending {
		t.Fatal("expected no proposal when env-disabled")
	}

	// Nil platform: degrades to a no-op.
	t.Setenv("SG_AUTO_DISTILL_ON_SEAL", "true")
	h2 := NewProfilesHandler(profiles, cont, ev, nil, "", nil)
	h2.AutoDistillSession(context.Background(), "s2", "bianmu")
	if pending, _ := profiles.HasProposal("operator:bianmu"); pending {
		t.Fatal("expected no proposal when platform is nil")
	}
}
