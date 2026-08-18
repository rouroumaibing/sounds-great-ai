package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"sounds-great-ai/internal/memory"
)

func TestLanesHandlerLifecycle(t *testing.T) {
	reg := memory.NewLaneRegistry()
	disp := memory.NewDispositionRecorder()
	h := NewLanesHandler(reg, disp, nil, "operator", nil)

	// Seed a pending candidate directly (mimics P2 session-close supply).
	lane := reg.Lane(memory.LaneDecision)
	e := lane.Submit("decided to use Go for platform", "session:s1")

	// GET pending -> 1 entry
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/memory/lanes/pending", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pending status = %d", rec.Code)
	}
	var pending []*memory.LaneEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &pending); err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	// POST approve
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/memory/lanes/"+e.ID+"/approve", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d body=%s", rec.Code, rec.Body.String())
	}

	// GET truth?lane=decision -> 1 approved
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/memory/lanes/truth?lane=decision", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("truth status = %d", rec.Code)
	}
	var truth []*memory.LaneEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &truth); err != nil {
		t.Fatal(err)
	}
	if len(truth) != 1 {
		t.Fatalf("expected 1 truth, got %d", len(truth))
	}

	// pending now empty
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/memory/lanes/pending", nil)
	h.Routes().ServeHTTP(rec, req)
	var pending2 []*memory.LaneEntry
	_ = json.Unmarshal(rec.Body.Bytes(), &pending2)
	if len(pending2) != 0 {
		t.Fatalf("expected 0 pending after approve, got %d", len(pending2))
	}
}

func TestLanesHandlerReject(t *testing.T) {
	reg := memory.NewLaneRegistry()
	disp := memory.NewDispositionRecorder()
	h := NewLanesHandler(reg, disp, nil, "operator", nil)
	e := reg.Lane(memory.LaneTaste).Submit("prefer dark mode", "session:s1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/memory/lanes/"+e.ID+"/reject", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reject status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := reg.Lane(memory.LaneTaste).Get(e.ID)
	if got.Status != memory.StatusForgotten {
		t.Fatalf("expected forgotten, got %s", got.Status)
	}
}

func TestLanesHandlerModify(t *testing.T) {
	reg := memory.NewLaneRegistry()
	disp := memory.NewDispositionRecorder()
	h := NewLanesHandler(reg, disp, nil, "operator", nil)
	e := reg.Lane(memory.LaneProfile).Submit("I am a backend dev", "session:s1")

	body := strings.NewReader(`{"content":"I am a backend developer"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/memory/lanes/"+e.ID+"/modify", body)
	req.Header.Set("Content-Type", "application/json")
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("modify status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := reg.Lane(memory.LaneProfile).Get(e.ID)
	if got.Content != "I am a backend developer" {
		t.Fatalf("expected modified content, got %s", got.Content)
	}
	if got.Status != memory.StatusApproved {
		t.Fatalf("expected approved, got %s", got.Status)
	}
}

func TestLanesHandlerDeferUndo(t *testing.T) {
	reg := memory.NewLaneRegistry()
	disp := memory.NewDispositionRecorder()
	recall := memory.NewRecallStore(t.TempDir())
	h := NewLanesHandler(reg, disp, recall, "operator", nil)
	e := reg.Lane(memory.LaneDecision).Submit("decided to use Go for platform", "session:s1")

	// defer -> deferred, still not truth
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/memory/lanes/"+e.ID+"/defer", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("defer status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := reg.Lane(memory.LaneDecision).Get(e.ID)
	if got.Status != memory.StatusDeferred {
		t.Fatalf("expected deferred, got %s", got.Status)
	}

	// undo -> back to pending
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/memory/lanes/"+e.ID+"/undo", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("undo status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ = reg.Lane(memory.LaneDecision).Get(e.ID)
	if got.Status != memory.StatusPending {
		t.Fatalf("expected pending after undo, got %s", got.Status)
	}
}

func TestLanesHandlerSearch(t *testing.T) {
	reg := memory.NewLaneRegistryAt(filepath.Join(t.TempDir(), "lanes.json"))
	defer reg.Close()
	disp := memory.NewDispositionRecorder()
	h := NewLanesHandler(reg, disp, nil, "operator", nil)

	// Approve a candidate so it becomes searchable truth.
	e := reg.Lane(memory.LaneDecision).Submit("decided to use Go for the backend", "session:s1")
	reg.Lane(memory.LaneDecision).Approve(e.ID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/memory/lanes/search?q=Go", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("search status = %d body=%s", rec.Code, rec.Body.String())
	}
	var hits []*memory.LaneEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != e.ID {
		t.Fatalf("expected 1 hit for 'Go', got %v", hits)
	}

	// No query -> 400
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/memory/lanes/search", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing q, got %d", rec.Code)
	}
}

func TestLanesHandlerRecallEndpoints(t *testing.T) {
	reg := memory.NewLaneRegistry()
	disp := memory.NewDispositionRecorder()
	recall := memory.NewRecallStore(t.TempDir())
	h := NewLanesHandler(reg, disp, recall, "operator", nil)

	// Seed one recall event.
	recall.Record(&memory.RecallEvent{OperatorID: "operator", Kind: "push", Trigger: "seal", EntryIDs: []string{"e1"}, Count: 1})

	// GET recall/events -> 1
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/memory/lanes/recall/events?limit=10", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("recall events status = %d", rec.Code)
	}
	var events []*memory.RecallEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 recall event, got %d", len(events))
	}

	// GET recall/ledger -> contains 7d/14d/30d keys
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/memory/lanes/recall/ledger?windows=7,14,30", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("recall ledger status = %d", rec.Code)
	}
	var ledger map[string]memory.RecallWindowStat
	if err := json.Unmarshal(rec.Body.Bytes(), &ledger); err != nil {
		t.Fatal(err)
	}
	if ledger["7d"].Total != 1 || ledger["14d"].Total != 1 || ledger["30d"].Total != 1 {
		t.Fatalf("expected ledger total 1 for all windows, got %v", ledger)
	}
}
