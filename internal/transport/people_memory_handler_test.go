package transport

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sounds-great-ai/internal/domains/threads/ports"
	"sounds-great-ai/internal/settings"
)

func newTestPeopleServer(t *testing.T, authorizer settings.SourceAuthorizer) (*PeopleMemoryHandler, *httptest.Server) {
	t.Helper()
	store := settings.NewFilePeopleMemoryStore(t.TempDir())
	h := NewPeopleMemoryHandler(store, "operator", authorizer, settings.NewPeopleMemoryEventHub())
	return h, httptest.NewServer(h.Routes())
}

// TestPeopleMemoryHTTPFlow exercises the full propose → approve → recall →
// hard-forget flow over HTTP. With AllowAllAuthorizer the source refs are
// accepted without a threadstore.
func TestPeopleMemoryHTTPFlow(t *testing.T) {
	_, srv := newTestPeopleServer(t, settings.AllowAllAuthorizer{})
	defer srv.Close()

	// 1. Propose a candidate with a new person + one claim draft.
	proposeBody := `{
		"person_draft": {"display_name": "黄挺", "private_aliases": ["ht"]},
		"claim_drafts": [{"draft_id":"d1","payload":{"kind":"reported_fact","predicate":"role","value":"设计负责人"},"normalized_draft":"role=设计负责人","source_role":"owner_explicit","evidence_excerpt":"黄挺是设计负责人"}],
		"source_message_ref": {"source_kind":"message_text","thread_id":"t1","message_id":"m1","excerpt":"黄挺是设计负责人"}
	}`
	resp, err := http.Post(srv.URL+"/api/people-memory/propose", "application/json", bytes.NewBufferString(proposeBody))
	if err != nil {
		t.Fatalf("post propose: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("propose status %d", resp.StatusCode)
	}
	var cand settings.CaptureCandidate
	_ = json.NewDecoder(resp.Body).Decode(&cand)
	resp.Body.Close()
	if cand.CandidateID == "" {
		t.Fatal("no candidate id returned")
	}

	// 2. Approve the draft.
	approveBody := `{"draft_ids":["d1"]}`
	resp2, err := http.Post(srv.URL+"/api/people-memory/candidates/"+cand.CandidateID+"/approve", "application/json", bytes.NewBufferString(approveBody))
	if err != nil {
		t.Fatalf("post approve: %v", err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("approve status %d", resp2.StatusCode)
	}
	var rec settings.PersonMemoryDecisionReceipt
	_ = json.NewDecoder(resp2.Body).Decode(&rec)
	resp2.Body.Close()
	if len(rec.MaterializedClaimIDs) != 1 {
		t.Fatalf("expected 1 materialized claim, got %v", rec.MaterializedClaimIDs)
	}

	// 3. Recall the card.
	resp3, err := http.Get(srv.URL + "/api/people-memory/person/" + rec.PersonID + "/card")
	if err != nil {
		t.Fatalf("get card: %v", err)
	}
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("card status %d", resp3.StatusCode)
	}
	var card settings.RelationshipCard
	_ = json.NewDecoder(resp3.Body).Decode(&card)
	resp3.Body.Close()
	if card.DisplayName != "黄挺" || len(card.Facts) != 1 {
		t.Fatalf("card mismatch: %+v", card)
	}
	if card.Storable || card.Indexable {
		t.Fatal("recall card must be non-storable / non-indexable")
	}

	// 4. List people shows the new person.
	resp4, _ := http.Get(srv.URL + "/api/people-memory")
	var people []map[string]any
	_ = json.NewDecoder(resp4.Body).Decode(&people)
	resp4.Body.Close()
	if len(people) != 1 {
		t.Fatalf("expected 1 person, got %d", len(people))
	}

	// 5. Hard-forget the person.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/people-memory/person/"+rec.PersonID+"/forget", nil)
	resp5, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("forget: %v", err)
	}
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("forget status %d", resp5.StatusCode)
	}
	var del settings.PersonMemoryDeletionReceipt
	_ = json.NewDecoder(resp5.Body).Decode(&del)
	resp5.Body.Close()
	if del.Verdict != "purged" {
		t.Fatalf("expected purged, got %s", del.Verdict)
	}

	// 6. After forget, recall returns 404.
	resp6, _ := http.Get(srv.URL + "/api/people-memory/person/" + rec.PersonID + "/card")
	if resp6.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after forget, got %d", resp6.StatusCode)
	}
	resp6.Body.Close()
}

// TestPeopleMemoryOperatorIsolationHTTP verifies the X-Operator-Id header scopes
// data per operator over HTTP (KD-1 multi-operator).
func TestPeopleMemoryOperatorIsolationHTTP(t *testing.T) {
	_, srv := newTestPeopleServer(t, settings.AllowAllAuthorizer{})
	defer srv.Close()

	proposeBody := `{
		"person_draft": {"display_name": "Alice 的人", "private_aliases": ["a"]},
		"claim_drafts": [{"draft_id":"d1","payload":{"kind":"reported_fact","predicate":"x","value":"y"},"normalized_draft":"x=y","source_role":"owner_explicit","evidence_excerpt":"x=y"}],
		"source_message_ref": {"source_kind":"message_text","thread_id":"t1","message_id":"m1"}
	}`

	// Propose as alice.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/people-memory/propose", bytes.NewBufferString(proposeBody))
	req.Header.Set("X-Operator-Id", "alice")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("propose status %d", resp.StatusCode)
	}
	var aliceCand settings.CaptureCandidate
	_ = json.NewDecoder(resp.Body).Decode(&aliceCand)
	resp.Body.Close()
	if aliceCand.CandidateID == "" {
		t.Fatal("no candidate id returned")
	}
	// Approve it so alice has a materialized person.
	appr, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/people-memory/candidates/"+aliceCand.CandidateID+"/approve", bytes.NewBufferString(`{"draft_ids":["d1"]}`))
	appr.Header.Set("X-Operator-Id", "alice")
	apprResp, err := http.DefaultClient.Do(appr)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if apprResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status %d", apprResp.StatusCode)
	}
	apprResp.Body.Close()

	// Bob sees nothing.
	reqBob, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/people-memory", nil)
	reqBob.Header.Set("X-Operator-Id", "bob")
	respBob, _ := http.DefaultClient.Do(reqBob)
	var bobPeople []map[string]any
	_ = json.NewDecoder(respBob.Body).Decode(&bobPeople)
	respBob.Body.Close()
	if len(bobPeople) != 0 {
		t.Fatalf("bob should see 0 people, got %d", len(bobPeople))
	}

	// Alice sees 1.
	reqAlice, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/people-memory", nil)
	reqAlice.Header.Set("X-Operator-Id", "alice")
	respAlice, _ := http.DefaultClient.Do(reqAlice)
	var alicePeople []map[string]any
	_ = json.NewDecoder(respAlice.Body).Decode(&alicePeople)
	respAlice.Body.Close()
	if len(alicePeople) != 1 {
		t.Fatalf("alice should see 1 person, got %d", len(alicePeople))
	}
}

// ---- fakes for cross-thread source authorization ----

type fakeThreadStore struct {
	threads []*ports.Thread
}

func (f *fakeThreadStore) CreateThread(title string) (*ports.Thread, error) { return &ports.Thread{}, nil }
func (f *fakeThreadStore) ListThreads() ([]*ports.Thread, error)           { return f.threads, nil }
func (f *fakeThreadStore) DeleteThread(id string) error                    { return nil }
func (f *fakeThreadStore) UpdateTitle(id string, title string) error       { return nil }
func (f *fakeThreadStore) AddEvent(threadID string, event json.RawMessage) error { return nil }
func (f *fakeThreadStore) GetEvents(threadID string) ([]json.RawMessage, error) { return nil, nil }
func (f *fakeThreadStore) CreateSession(threadID, breedID string) (*ports.SessionRecord, error) {
	return &ports.SessionRecord{}, nil
}
func (f *fakeThreadStore) ListSessions(threadID string) ([]*ports.SessionRecord, error) {
	return nil, nil
}
func (f *fakeThreadStore) UnsealSession(sessionID string) error { return nil }

type fakeMessageStore struct {
	msgs map[string][]*ports.Message
}

func (f *fakeMessageStore) Append(msg *ports.Message) error { return nil }
func (f *fakeMessageStore) GetByThread(threadID string, limit int) ([]*ports.Message, error) {
	return f.msgs[threadID], nil
}
func (f *fakeMessageStore) GetByThreadBefore(threadID string, before time.Time, beforeID string, limit int) ([]*ports.Message, error) {
	return nil, nil
}

// TestPeopleMemoryCrossThreadForbidden verifies the fail-closed cross-thread
// source authorizer: a source ref pointing at an unknown thread (or message)
// is rejected with 403, while accessible sources are allowed.
func TestPeopleMemoryCrossThreadForbidden(t *testing.T) {
	fakeThreads := &fakeThreadStore{threads: []*ports.Thread{{ID: "t1"}}}
	fakeMsgs := &fakeMessageStore{msgs: map[string][]*ports.Message{
		"t1": {{ID: "m1", ThreadID: "t1"}},
	}}
	auth := NewThreadstoreAuthorizer(fakeThreads, fakeMsgs)
	_, srv := newTestPeopleServer(t, auth)
	defer srv.Close()

	// Unknown thread -> 403 (fail-closed).
	body403 := `{"person_draft":{"display_name":"X"},"claim_drafts":[{"draft_id":"d1","payload":{"kind":"reported_fact","predicate":"a","value":"b"},"normalized_draft":"a=b","source_role":"owner_explicit","evidence_excerpt":"a=b"}],"source_message_ref":{"source_kind":"message_text","thread_id":"t2","message_id":"mX"}}`
	resp403, _ := http.Post(srv.URL+"/api/people-memory/propose", "application/json", bytes.NewBufferString(body403))
	if resp403.StatusCode != http.StatusForbidden {
		t.Fatalf("unknown thread should be 403, got %d", resp403.StatusCode)
	}
	resp403.Body.Close()

	// Known thread+message -> 201.
	body201 := `{"person_draft":{"display_name":"Y"},"claim_drafts":[{"draft_id":"d1","payload":{"kind":"reported_fact","predicate":"a","value":"b"},"normalized_draft":"a=b","source_role":"owner_explicit","evidence_excerpt":"a=b"}],"source_message_ref":{"source_kind":"message_text","thread_id":"t1","message_id":"m1"}}`
	resp201, _ := http.Post(srv.URL+"/api/people-memory/propose", "application/json", bytes.NewBufferString(body201))
	if resp201.StatusCode != http.StatusCreated {
		t.Fatalf("known thread+message should be 201, got %d", resp201.StatusCode)
	}
	resp201.Body.Close()

	// No thread/message (operator-authored) -> 201.
	bodyManual := `{"person_draft":{"display_name":"Z"},"claim_drafts":[{"draft_id":"d1","payload":{"kind":"reported_fact","predicate":"a","value":"b"},"normalized_draft":"a=b","source_role":"owner_explicit","evidence_excerpt":"a=b"}],"source_message_ref":{"source_kind":"operator"}}`
	respManual, _ := http.Post(srv.URL+"/api/people-memory/propose", "application/json", bytes.NewBufferString(bodyManual))
	if respManual.StatusCode != http.StatusCreated {
		t.Fatalf("operator source should be 201, got %d", respManual.StatusCode)
	}
	respManual.Body.Close()
}

// TestPeopleMemorySourceFieldReverify exercises the homologous
// per-field re-verification in ThreadstoreAuthorizer: a captured SourceRef must
// honestly correspond to the real message (excerpt contained in body; ref/digest
// matches the derived digest). Mismatches fail closed with 403.
func TestPeopleMemorySourceFieldReverify(t *testing.T) {
	realMsg := &ports.Message{
		ID:        "m1",
		ThreadID: "t1",
		Role:      "user",
		Content:   "黄挺是设计负责人，负责品牌视觉。",
		Timestamp: time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC),
	}
	fakeThreads := &fakeThreadStore{threads: []*ports.Thread{{ID: "t1"}}}
	fakeMsgs := &fakeMessageStore{msgs: map[string][]*ports.Message{"t1": {realMsg}}}
	auth := NewThreadstoreAuthorizer(fakeThreads, fakeMsgs)
	_, srv := newTestPeopleServer(t, auth)
	defer srv.Close()

	goodDigest := messageDigest(realMsg)

	propose := func(refBody string) int {
		body := `{"person_draft":{"display_name":"X"},"claim_drafts":[{"draft_id":"d1","payload":{"kind":"reported_fact","predicate":"a","value":"b"},"normalized_draft":"a=b","source_role":"owner_explicit","evidence_excerpt":"a=b"}],"source_message_ref":` + refBody + `}`
		resp, err := http.Post(srv.URL+"/api/people-memory/propose", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("post propose: %v", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	// Excerpt contained in body -> 201.
	if code := propose(`{"source_kind":"message_text","thread_id":"t1","message_id":"m1","excerpt":"黄挺是设计负责人"}`); code != http.StatusCreated {
		t.Fatalf("matching excerpt should be 201, got %d", code)
	}

	// Excerpt NOT contained in body (forged attribution) -> 403 fail-closed.
	if code := propose(`{"source_kind":"message_text","thread_id":"t1","message_id":"m1","excerpt":"他其实是工程师"}`); code != http.StatusForbidden {
		t.Fatalf("mismatched excerpt should be 403, got %d", code)
	}

	// Correct digest -> 201.
	if code := propose(`{"source_kind":"message_text","thread_id":"t1","message_id":"m1","ref":"` + goodDigest + `"}`); code != http.StatusCreated {
		t.Fatalf("matching digest should be 201, got %d", code)
	}

	// Wrong digest (forged ref) -> 403 fail-closed.
	if code := propose(`{"source_kind":"message_text","thread_id":"t1","message_id":"m1","ref":"deadbeef"}`); code != http.StatusForbidden {
		t.Fatalf("mismatched digest should be 403, got %d", code)
	}

	// No excerpt/ref -> 201 (backward compatible).
	if code := propose(`{"source_kind":"message_text","thread_id":"t1","message_id":"m1"}`); code != http.StatusCreated {
		t.Fatalf("bare source should be 201, got %d", code)
	}
}

// TestPeopleMemorySSE verifies the SSE endpoint streams PeopleMemoryEvents to a
// subscriber and filters them by operator scope.
func TestPeopleMemorySSE(t *testing.T) {
	dir := t.TempDir()
	hub := settings.NewPeopleMemoryEventHub()
	store := settings.NewBroadcastingPeopleMemoryStore(settings.NewFilePeopleMemoryStore(dir), hub)
	h := NewPeopleMemoryHandler(store, "operator", settings.AllowAllAuthorizer{}, hub)
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/people-memory/events?operator=op1")
	if err != nil {
		t.Fatalf("connect sse: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("unexpected content-type: %q", ct)
	}

	got := make(chan settings.PeopleMemoryEvent, 1)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var ev settings.PeopleMemoryEvent
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err == nil {
				got <- ev
				return
			}
		}
	}()

	// A mutation for the subscribed operator must reach the stream.
	if _, err := store.Propose("op1", &settings.CaptureCandidate{
		PersonDraft: &settings.PersonIdentityDraft{DisplayName: "Carol"},
		ClaimDrafts: []settings.CandidateClaimDraft{{DraftID: "d1", Decision: "pending"}},
	}); err != nil {
		t.Fatalf("propose op1: %v", err)
	}
	select {
	case ev := <-got:
		if ev.OperatorID != "op1" || ev.Type != "proposed" {
			t.Fatalf("unexpected sse event: %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no SSE event received for subscribed operator")
	}

	// A different operator's mutation must NOT be delivered to this stream.
	if _, err := store.Propose("op2", &settings.CaptureCandidate{
		PersonDraft: &settings.PersonIdentityDraft{DisplayName: "Dave"},
		ClaimDrafts: []settings.CandidateClaimDraft{{DraftID: "d2", Decision: "pending"}},
	}); err != nil {
		t.Fatalf("propose op2: %v", err)
	}
	select {
	case ev := <-got:
		t.Fatalf("operator-scoped filter leaked: %+v", ev)
	case <-time.After(400 * time.Millisecond):
		// expected: no event for op2 on an op1 subscription
	}
}
