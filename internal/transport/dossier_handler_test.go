package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sounds-great-ai/internal/dossier"
	"sounds-great-ai/pkg/pack"
)

func newDossierTestServer(t *testing.T) (*httptest.Server, *dossier.Service, *dossier.Loader) {
	t.Helper()
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "docs", "team"), 0o755); err != nil {
		t.Fatal(err)
	}
	dossierContent := "### 边牧 · @边牧 · `dog:bianmu`\n\n```yaml\n# structured-profile: dog:bianmu\nentityId: \"bianmu\"\nl0RosterSummary: \"任务拆解\"\nl0RoutingNote: \"不替队友做主\"\n```\n"
	if err := os.WriteFile(filepath.Join(workspace, dossier.DossierRelativePath), []byte(dossierContent), 0o644); err != nil {
		t.Fatal(err)
	}

	observations, _ := dossier.NewObservationStoreAt("")
	proposals, _ := dossier.NewProposalStoreAt("")
	loader := dossier.NewLoader()
	opportunities := dossier.NewInMemoryOpportunityStore()
	svc := dossier.NewService(proposals, observations, opportunities, dossier.NewCheckpoint(opportunities, nil), loader, workspace)

	breeds := map[string]*pack.BreedConfig{
		"bianmu": {
			ID: "bianmu", DisplayName: "边牧", DogID: "bianmu", DefaultVariantID: "v1",
			Variants: []pack.Variant{{ID: "v1", DogID: "bianmu", ClientID: "claude", DefaultModel: "claude-opus-4-6"}},
		},
	}
	handler := NewDossierHandler(svc, loader, func() map[string]*pack.BreedConfig { return breeds })
	server := httptest.NewServer(handler.Routes())
	t.Cleanup(server.Close)
	return server, svc, loader
}

func doJSON(t *testing.T, method, url string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

func TestDossierOverviewJoin(t *testing.T) {
	server, _, _ := newDossierTestServer(t)
	resp, body := doJSON(t, "GET", server.URL+"/api/dossier", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	meta := body["meta"].(map[string]any)
	if meta["totalDogs"].(float64) != 1 {
		t.Errorf("totalDogs = %v", meta["totalDogs"])
	}
	if meta["dossierCoverage"].(float64) != 1.0 {
		t.Errorf("coverage = %v, want 1.0", meta["dossierCoverage"])
	}
	groups := body["modelGroups"].([]any)
	g := groups[0].(map[string]any)
	if g["model"] != "claude-opus-4-6" {
		t.Errorf("model group = %v", g["model"])
	}
	dog := g["dogs"].([]any)[0].(map[string]any)
	if dog["dogId"] != "bianmu" || dog["dossier"] == nil {
		t.Errorf("dog card = %v", dog)
	}
}

func TestObservationEndpoints(t *testing.T) {
	server, _, _ := newDossierTestServer(t)

	resp, _ := doJSON(t, "POST", server.URL+"/api/dossier/observations", map[string]string{"dogId": "bianmu", "content": "拆解稳"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, "POST", server.URL+"/api/dossier/observations", map[string]string{"dogId": "", "content": "x"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing dogId must 400, got %d", resp.StatusCode)
	}
	resp, body := doJSON(t, "GET", server.URL+"/api/dossier/observations?dogId=bianmu", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", resp.StatusCode)
	}
	obs := body["observations"].([]any)
	if len(obs) != 1 {
		t.Fatalf("observations = %v", obs)
	}
}

func TestDistillationLifecycleHTTP(t *testing.T) {
	server, svc, _ := newDossierTestServer(t)

	// Seed an opportunity the way the SOP checkpoint would.
	svc.Checkpoint.OnReviewComplete(dossier.ReviewCompleteContext{
		ThreadID: "t1", ReviewerDogID: "xigou", AuthorDogID: "bianmu", CommitSHA: "sha1",
	})

	// Scoped listing: a dog only sees its own.
	resp, body := doJSON(t, "GET", server.URL+"/api/dossier/distillation-opportunities", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("opportunities status %d", resp.StatusCode)
	}
	if len(body["opportunities"].([]any)) != 1 {
		t.Fatalf("operator must see all opportunities: %v", body)
	}

	hash, err := svc.CurrentBaseHash()
	if err != nil {
		t.Fatal(err)
	}

	createPayload := map[string]any{
		"sourceEvent":    "review-complete",
		"sourceId":       "review-complete:t1:sha1:xigou",
		"targetDogId":    "bianmu",
		"targetFields":   []string{"l0RosterSummary"},
		"beforeSnapshot": "l0RosterSummary: \"任务拆解\"",
		"afterDraft":     "l0RosterSummary: \"任务拆解（校准版）\"",
		"rationale":      "review 证实",
		"evidenceRefs":   []map[string]string{{"type": "review", "id": "t1"}},
		"baseHash":       hash,
		"actor":          "xigou",
	}

	resp, body = doJSON(t, "POST", server.URL+"/api/dossier/distillations", createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status %d: %v", resp.StatusCode, body)
	}
	proposal := body["proposal"].(map[string]any)
	proposalID := proposal["proposalId"].(string)

	// Idempotent re-create returns 200 with the same proposal.
	resp, body = doJSON(t, "POST", server.URL+"/api/dossier/distillations", createPayload)
	if resp.StatusCode != http.StatusOK || body["proposal"].(map[string]any)["proposalId"] != proposalID {
		t.Errorf("idempotent create: status %d", resp.StatusCode)
	}

	// Fail-closed: no evidence.
	bad := map[string]any{"sourceEvent": "review-complete", "sourceId": "x", "targetDogId": "b",
		"targetFields": []string{"f"}, "beforeSnapshot": "a", "afterDraft": "b",
		"rationale": "r", "baseHash": hash}
	resp, _ = doJSON(t, "POST", server.URL+"/api/dossier/distillations", bad)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty evidence must 400, got %d", resp.StatusCode)
	}

	// Self-approval refused.
	resp, _ = doJSON(t, "POST", server.URL+"/api/dossier/distillations/"+proposalID+"/approve", map[string]string{"actor": "xigou"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("self-approve must 403, got %d", resp.StatusCode)
	}

	// Operator approves.
	resp, body = doJSON(t, "POST", server.URL+"/api/dossier/distillations/"+proposalID+"/approve", map[string]string{"actor": "operator"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve status %d: %v", resp.StatusCode, body)
	}

	// Non-target apply refused.
	resp, _ = doJSON(t, "POST", server.URL+"/api/dossier/distillations/"+proposalID+"/execute-apply", map[string]string{"actor": "jinmao"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-target apply must 403, got %d", resp.StatusCode)
	}

	// Pending list empty after approval.
	resp, body = doJSON(t, "GET", server.URL+"/api/dossier/distillations", nil)
	if len(body["proposals"].([]any)) != 0 {
		t.Errorf("pending should be empty post-approval: %v", body)
	}

	// Convert the opportunity (closes the loop).
	oppID := ""
	_, oppBody := doJSON(t, "GET", server.URL+"/api/dossier/distillation-opportunities", nil)
	for _, o := range oppBody["opportunities"].([]any) {
		oppID = o.(map[string]any)["opportunityId"].(string)
	}
	if oppID != "" {
		resp, _ = doJSON(t, "POST", server.URL+"/api/dossier/distillation-opportunities/"+oppID+"/convert", map[string]string{"proposalId": proposalID})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("convert status %d", resp.StatusCode)
		}
	}

	_ = strings.TrimSpace // keep strings import if assertions above change
}
