package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"sounds-great-ai/internal/eval"
)

// mockEvalStore is an in-memory EvalStore used to verify the EvalHandler
// depends on the port (G10) and can be exercised without the filesystem.
type mockEvalStore struct {
	verdicts []eval.VerdictHandoffPacket
	byID     map[string]eval.VerdictHandoffPacket
}

func (m *mockEvalStore) ListVerdicts(domainID string) ([]eval.VerdictHandoffPacket, error) {
	if domainID == "" {
		return m.verdicts, nil
	}
	var out []eval.VerdictHandoffPacket
	for _, v := range m.verdicts {
		if v.DomainID == domainID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *mockEvalStore) GetVerdict(id string) (*eval.VerdictHandoffPacket, error) {
	if v, ok := m.byID[id]; ok {
		vv := v
		return &vv, nil
	}
	return nil, fmt.Errorf("verdict %q not found", id)
}

func newTestEvalHandler(store EvalStore) *EvalHandler {
	// runner/closure/scheduler are nil-safe for the store-only handlers we test.
	runner := eval.NewEvalRunner(nil, nil, []eval.EvalDomain{
		{DomainID: "d1", DisplayName: "routing"},
		{DomainID: "d2", DisplayName: "retrieval"},
	})
	return NewEvalHandler(runner, store, nil, nil)
}

func TestEvalHandler_ListEvals_UsesPortStore(t *testing.T) {
	store := &mockEvalStore{
		verdicts: []eval.VerdictHandoffPacket{
			{ID: "v1", DomainID: "d1", Verdict: "pass", Phenomenon: "routing ok"},
			{ID: "v2", DomainID: "d2", Verdict: "pass", Phenomenon: "retrieval ok"},
		},
	}
	h := newTestEvalHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/evals", nil)
	rec := httptest.NewRecorder()
	h.handleEvals(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var summaries []struct {
		Domain eval.EvalDomain `json:"domain"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&summaries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(summaries))
	}
	// d1 should carry its latest verdict.
	if summaries[0].Domain.DomainID != "d1" {
		t.Fatalf("expected first domain d1, got %s", summaries[0].Domain.DomainID)
	}
}

func TestEvalHandler_ListResults_FiltersByDomain(t *testing.T) {
	store := &mockEvalStore{
		verdicts: []eval.VerdictHandoffPacket{
			{ID: "v1", DomainID: "d1", Verdict: "pass"},
			{ID: "v2", DomainID: "d2", Verdict: "fail"},
		},
	}
	h := newTestEvalHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/evals/results?domainId=d1", nil)
	rec := httptest.NewRecorder()
	h.handleResults(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var got []eval.VerdictHandoffPacket
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].ID != "v1" {
		t.Fatalf("expected only v1 for d1, got %+v", got)
	}
}
