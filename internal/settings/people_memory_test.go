package settings

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const testOperator = "operator"

func newTestFileStore(t *testing.T) PeopleMemoryStore {
	t.Helper()
	return NewFilePeopleMemoryStore(t.TempDir())
}

// newTestRedisStore spins up an in-process miniredis and returns the
// operator-partitioned Redis-backed store.
func newTestRedisStore(t *testing.T) PeopleMemoryStore {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisPeopleMemoryStore(client)
}

func TestProposeApproveMaterializeAndSupersede(t *testing.T) {
	s := newTestFileStore(t)
	cand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{
			DisplayName:    "黄挺",
			PrivateAliases: []string{"ht", "黄老师"},
		},
		ClaimDrafts: []CandidateClaimDraft{
			{
				DraftID:         "d1",
				Payload:         PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "role", Value: "设计负责人"},
				NormalizedDraft: "role=设计负责人",
				SourceRole:      "owner_explicit",
				EvidenceExcerpt: "黄挺是设计负责人",
			},
		},
		SourceMessageRef: SourceRef{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1", Excerpt: "黄挺是设计负责人"},
	}
	stored, err := s.Propose(testOperator, cand)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if stored.State != CandPendingApproval {
		t.Fatalf("expected pending_approval, got %s", stored.State)
	}

	rec, err := s.ApproveDrafts(testOperator, stored.CandidateID, []string{"d1"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if rec.PersonID == "" {
		t.Fatal("expected a person id")
	}
	if len(rec.MaterializedClaimIDs) != 1 {
		t.Fatalf("expected 1 materialized claim, got %d", len(rec.MaterializedClaimIDs))
	}
	if len(rec.RemainingDraftIDs) != 0 {
		t.Fatalf("expected no remaining drafts, got %v", rec.RemainingDraftIDs)
	}

	// Recall the card.
	card, ok, err := s.RecallCard(testOperator, rec.PersonID)
	if err != nil || !ok {
		t.Fatalf("recall: ok=%v err=%v", ok, err)
	}
	if card.DisplayName != "黄挺" || len(card.Facts) != 1 {
		t.Fatalf("card mismatch: %+v", card)
	}

	// Supersede: approve a corrected claim with the same key.
	cand2 := &CaptureCandidate{
		TargetPersonID: rec.PersonID,
		ClaimDrafts: []CandidateClaimDraft{
			{
				DraftID:         "d2",
				Payload:         PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "role", Value: "首席设计师"},
				NormalizedDraft: "role=首席设计师",
				SourceRole:      "owner_explicit",
				EvidenceExcerpt: "其实黄挺是首席设计师",
			},
		},
		SourceMessageRef: SourceRef{SourceKind: "message_text", ThreadID: "t1", MessageID: "m2", Excerpt: "其实黄挺是首席设计师"},
	}
	stored2, _ := s.Propose(testOperator, cand2)
	rec2, err := s.ApproveDrafts(testOperator, stored2.CandidateID, []string{"d2"})
	if err != nil {
		t.Fatalf("approve2: %v", err)
	}
	if len(rec2.RestoredClaimIDs) != 1 {
		t.Fatalf("expected 1 superseded (restored) claim, got %v", rec2.RestoredClaimIDs)
	}
	claims, _ := s.ListClaims(testOperator, rec.PersonID)
	current := 0
	for _, c := range claims {
		if c.Status == ClaimStatusCurrent {
			current++
			if c.Payload.Value != "首席设计师" {
				t.Fatalf("current claim not updated: %+v", c.Payload)
			}
		}
	}
	if current != 1 {
		t.Fatalf("expected exactly 1 current claim after supersede, got %d", current)
	}
}

func TestNotNowRejectWithdraw(t *testing.T) {
	s := newTestFileStore(t)
	cand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "测试人物"},
		ClaimDrafts: []CandidateClaimDraft{
			{DraftID: "d1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "x", Value: "y"}, NormalizedDraft: "x=y", SourceRole: "owner_explicit"},
		},
	}
	stored, _ := s.Propose(testOperator, cand)

	if _, err := s.MarkNotNow(testOperator, stored.CandidateID); err != nil {
		t.Fatalf("not-now: %v", err)
	}
	pending, _ := s.ListPending(testOperator, 0)
	if len(pending) != 1 || pending[0].State != CandNotNow {
		t.Fatalf("not-now should remain pending: %+v", pending)
	}

	if _, err := s.WithdrawCandidate(testOperator, stored.CandidateID); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	pending, _ = s.ListPending(testOperator, 0)
	if len(pending) != 0 {
		t.Fatalf("withdrawn candidate should leave pending list: %+v", pending)
	}

	// Reject path on a fresh one.
	cand2 := &CaptureCandidate{PersonDraft: &PersonIdentityDraft{DisplayName: "另一人"}, ClaimDrafts: []CandidateClaimDraft{{DraftID: "d1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "a", Value: "b"}, NormalizedDraft: "a=b", SourceRole: "owner_explicit"}}}
	stored2, _ := s.Propose(testOperator, cand2)
	if _, err := s.RejectCandidate(testOperator, stored2.CandidateID); err != nil {
		t.Fatalf("reject: %v", err)
	}
	pending, _ = s.ListPending(testOperator, 0)
	if len(pending) != 0 {
		t.Fatalf("rejected should leave pending list")
	}
}

func TestUndoDecision(t *testing.T) {
	s := newTestFileStore(t)
	cand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "撤销测试"},
		ClaimDrafts: []CandidateClaimDraft{
			{DraftID: "d1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "k", Value: "v"}, NormalizedDraft: "k=v", SourceRole: "owner_explicit"},
		},
	}
	stored, _ := s.Propose(testOperator, cand)
	rec, _ := s.ApproveDrafts(testOperator, stored.CandidateID, []string{"d1"})

	if _, err := s.UndoDecision(testOperator, rec.DecisionID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	claims, _ := s.ListClaims(testOperator, rec.PersonID)
	if len(claims) != 0 {
		t.Fatalf("undo should delete materialized claims, got %d", len(claims))
	}
	pending, _ := s.ListPending(testOperator, 0)
	if len(pending) != 1 {
		t.Fatalf("undo should reopen candidate, got %d pending", len(pending))
	}
}

func TestAgentInferenceNotMaterialized(t *testing.T) {
	s := newTestFileStore(t)
	cand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "推断人物"},
		ClaimDrafts: []CandidateClaimDraft{
			{DraftID: "d1", Payload: PersonClaimPayload{Kind: ClaimKindAgentInference, Statement: "他可能喜欢咖啡"}, NormalizedDraft: "推测", SourceRole: "owner_explicit"},
		},
	}
	stored, _ := s.Propose(testOperator, cand)
	if _, err := s.ApproveDrafts(testOperator, stored.CandidateID, []string{"d1"}); err == nil {
		t.Fatal("agent_inference must NOT be materializable")
	}
}

func TestDeferredReceiptDualPath(t *testing.T) {
	s := newTestFileStore(t)
	receipt, err := s.DeferReceipt(testOperator, "bianmu", "黄挺是设计负责人", "", []SourceRef{{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1", Ref: "digest-abc"}})
	if err != nil {
		t.Fatalf("defer: %v", err)
	}
	// Content-free: no message body/excerpt stored.
	if receipt.Subject == "" || len(receipt.SourceCoords) != 1 {
		t.Fatalf("receipt malformed: %+v", receipt)
	}
	ready, _ := s.ListReadyDeferred(testOperator)
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready receipt, got %d", len(ready))
	}
	c, err := s.ClaimDeferredReceipt(testOperator, receipt.ReceiptID, "bianmu")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if c.DeferredReceiptID != receipt.ReceiptID {
		t.Fatalf("claimed candidate should reference receipt")
	}
	ready, _ = s.ListReadyDeferred(testOperator)
	if len(ready) != 0 {
		t.Fatalf("claimed receipt should leave ready queue")
	}
	// Withdraw after claim must fail closed.
	if err := s.WithdrawReceipt(testOperator, receipt.ReceiptID); err == nil {
		t.Fatal("withdraw after claim must fail closed")
	}
}

func TestHardForget(t *testing.T) {
	s := newTestFileStore(t)
	cand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "删除人物"},
		ClaimDrafts: []CandidateClaimDraft{
			{DraftID: "d1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "k", Value: "v"}, NormalizedDraft: "k=v", SourceRole: "owner_explicit"},
		},
	}
	stored, _ := s.Propose(testOperator, cand)
	rec, _ := s.ApproveDrafts(testOperator, stored.CandidateID, []string{"d1"})

	del, err := s.HardForget(testOperator, rec.PersonID)
	if err != nil {
		t.Fatalf("forget: %v", err)
	}
	if del.Verdict != "purged" || del.Counts["people"] != 1 || del.Counts["claims"] != 1 {
		t.Fatalf("forget receipt wrong: %+v", del)
	}
	if _, ok, _ := s.GetPerson(testOperator, rec.PersonID); ok {
		t.Fatal("person should be gone after hard-forget")
	}
}

func TestPersistenceAcrossReload(t *testing.T) {
	dir := t.TempDir()
	s := NewFilePeopleMemoryStore(dir)
	cand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "持久化人物"},
		ClaimDrafts: []CandidateClaimDraft{
			{DraftID: "d1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "k", Value: "v"}, NormalizedDraft: "k=v", SourceRole: "owner_explicit"},
		},
	}
	stored, _ := s.Propose(testOperator, cand)
	rec, _ := s.ApproveDrafts(testOperator, stored.CandidateID, []string{"d1"})

	// Reload from disk.
	s2 := NewFilePeopleMemoryStore(dir)
	people, _ := s2.ListPeople(testOperator)
	if len(people) != 1 || people[0].DisplayName != "持久化人物" {
		t.Fatalf("reload lost person: %+v", people)
	}
	card, ok, _ := s2.RecallCard(testOperator, rec.PersonID)
	if !ok || card.Facts[0].Text != "k" {
		t.Fatalf("reload lost card: %+v", card)
	}
}

// TestMultiOperatorIsolation verifies that an operator's people/candidates are
// not visible to another operator (KD-1 multi-operator partitioning).
func TestMultiOperatorIsolation(t *testing.T) {
	s := newTestFileStore(t)
	alice := "alice"
	bob := "bob"

	cand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "Alice 的同事"},
		ClaimDrafts: []CandidateClaimDraft{
			{DraftID: "d1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "x", Value: "y"}, NormalizedDraft: "x=y", SourceRole: "owner_explicit"},
		},
	}
	stored, _ := s.Propose(alice, cand)
	s.ApproveDrafts(alice, stored.CandidateID, []string{"d1"})

	// Bob's namespace must be empty.
	people, _ := s.ListPeople(bob)
	if len(people) != 0 {
		t.Fatalf("bob should see no people, got %d", len(people))
	}
	pending, _ := s.ListPending(bob, 0)
	if len(pending) != 0 {
		t.Fatalf("bob should see no pending, got %d", len(pending))
	}

	// Bob proposing must not collide with Alice's data.
	bobCand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "Bob 的朋友"},
		ClaimDrafts: []CandidateClaimDraft{
			{DraftID: "d1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "a", Value: "b"}, NormalizedDraft: "a=b", SourceRole: "owner_explicit"},
		},
	}
	bobStored, _ := s.Propose(bob, bobCand)
	s.ApproveDrafts(bob, bobStored.CandidateID, []string{"d1"})

	ap, _ := s.ListPeople(alice)
	bp, _ := s.ListPeople(bob)
	if len(ap) != 1 || len(bp) != 1 {
		t.Fatalf("operator partitioning broken: alice=%d bob=%d", len(ap), len(bp))
	}
	if ap[0].DisplayName == bp[0].DisplayName {
		t.Fatalf("operators leaked data across namespaces")
	}
}

// TestDailyClerkPromotesReadyReceipts verifies the homologous daily
// clerk promotes a ready deferred receipt into a rejectable candidate.
func TestDailyClerkPromotesReadyReceipts(t *testing.T) {
	s := newTestFileStore(t)
	receipt, err := s.DeferReceipt(testOperator, "bianmu", "值得记住但不是现在", "", []SourceRef{{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1", Ref: "digest-xyz"}})
	if err != nil {
		t.Fatalf("defer: %v", err)
	}
	ready, _ := s.ListReadyDeferred(testOperator)
	if len(ready) != 1 {
		t.Fatalf("precondition: expected 1 ready receipt, got %d", len(ready))
	}

	// Run the clerk once (degraded mode: no invoke func → promote to empty candidate).
	RunPeopleMemoryClerkOnce(context.Background(), s, PeopleMemoryClerkDeps{})

	// The receipt is no longer ready, and a candidate now exists.
	ready, _ = s.ListReadyDeferred(testOperator)
	if len(ready) != 0 {
		t.Fatalf("clerk should have promoted the receipt; ready=%d", len(ready))
	}
	pending, _ := s.ListPending(testOperator, 0)
	if len(pending) != 1 {
		t.Fatalf("clerk should have created 1 candidate, got %d", len(pending))
	}
	if pending[0].DeferredReceiptID != receipt.ReceiptID {
		t.Fatalf("promoted candidate should reference the receipt")
	}
}

// TestRedisDualPath exercises the Lua-guarded deferred-receipt lifecycle under
// miniredis (cjson-free scripts run identically to real Redis).
func TestRedisDualPath(t *testing.T) {
	s := newTestRedisStore(t)
	receipt, err := s.DeferReceipt(testOperator, "bianmu", "Redis 凭据主题", "", []SourceRef{{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1", Ref: "digest-r1"}})
	if err != nil {
		t.Fatalf("defer: %v", err)
	}
	ready, _ := s.ListReadyDeferred(testOperator)
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready receipt, got %d", len(ready))
	}

	// Withdraw before claim succeeds.
	if err := s.WithdrawReceipt(testOperator, receipt.ReceiptID); err != nil {
		t.Fatalf("withdraw before claim: %v", err)
	}
	ready, _ = s.ListReadyDeferred(testOperator)
	if len(ready) != 0 {
		t.Fatalf("withdrawn receipt should leave ready queue")
	}

	// Re-defer and claim.
	receipt2, _ := s.DeferReceipt(testOperator, "bianmu", "Redis 凭据主题 2", "", []SourceRef{{SourceKind: "message_text", ThreadID: "t1", MessageID: "m2", Ref: "digest-r2"}})
	c, err := s.ClaimDeferredReceipt(testOperator, receipt2.ReceiptID, "bianmu")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if c.DeferredReceiptID != receipt2.ReceiptID {
		t.Fatalf("claimed candidate should reference receipt")
	}
	// Forget after claim must fail closed.
	if err := s.ForgetReceipt(testOperator, receipt2.ReceiptID); err == nil {
		t.Fatal("forget after claim must fail closed")
	}
}

// TestRedisMultiOperatorIsolation verifies operator partitioning works in the
// Lua/Redis store too, and that ListOperators surfaces operators that hold only
// deferred receipts (so the clerk can promote them).
func TestRedisMultiOperatorIsolation(t *testing.T) {
	s := newTestRedisStore(t)
	alice := "alice"
	bob := "bob"

	// Alice only defers a receipt (no doc written yet).
	if _, err := s.DeferReceipt(alice, "bianmu", "Alice 的待记忆", "", []SourceRef{{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1", Ref: "d-a"}}); err != nil {
		t.Fatalf("alice defer: %v", err)
	}
	// Bob proposes + approves a person.
	bobCand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "Bob 的人"},
		ClaimDrafts: []CandidateClaimDraft{
			{DraftID: "d1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "a", Value: "b"}, NormalizedDraft: "a=b", SourceRole: "owner_explicit"},
		},
	}
	stored, _ := s.Propose(bob, bobCand)
	s.ApproveDrafts(bob, stored.CandidateID, []string{"d1"})

	// Bob's people must not appear under Alice.
	ap, _ := s.ListPeople(alice)
	bp, _ := s.ListPeople(bob)
	if len(ap) != 0 {
		t.Fatalf("alice should see no people, got %d", len(ap))
	}
	if len(bp) != 1 {
		t.Fatalf("bob should see 1 person, got %d", len(bp))
	}

	// ListOperators must surface Alice even though she has only a deferred
	// receipt (no doc). This is what lets the daily clerk promote her receipt.
	ops, err := s.ListOperators()
	if err != nil {
		t.Fatalf("list operators: %v", err)
	}
	hasAlice := false
	hasBob := false
	for _, o := range ops {
		if o == alice {
			hasAlice = true
		}
		if o == bob {
			hasBob = true
		}
	}
	if !hasAlice || !hasBob {
		t.Fatalf("ListOperators missing operator: alice=%v bob=%v (got %v)", hasAlice, hasBob, ops)
	}
	// And the clerk can promote Alice's receipt (degraded mode).
	RunPeopleMemoryClerkOnce(context.Background(), s, PeopleMemoryClerkDeps{})
	ready, _ := s.ListReadyDeferred(alice)
	if len(ready) != 0 {
		t.Fatalf("clerk should have promoted alice's receipt; ready=%d", len(ready))
	}
	pending, _ := s.ListPending(alice, 0)
	if len(pending) != 1 {
		t.Fatalf("clerk should have created 1 candidate for alice, got %d", len(pending))
	}
}

// TestProposeMultiDraftRoundTrip verifies that a single candidate carrying
// several claim drafts plus one relationship draft and one interaction draft
// survives a propose -> GetCandidate round trip and materializes fully on
// approval. This is the profile-update-actions contract: many drafts
// presented together in one proposal block, each independently approvable.
// Exercises both the file and Redis (miniredis) backends.
func TestProposeMultiDraftRoundTrip(t *testing.T) {
	stores := map[string]func(t *testing.T) PeopleMemoryStore{
		"file":  newTestFileStore,
		"redis": newTestRedisStore,
	}
	for name, makeStore := range stores {
		t.Run(name, func(t *testing.T) {
			s := makeStore(t)
			cand := &CaptureCandidate{
				PersonDraft: &PersonIdentityDraft{DisplayName: "林夕", PrivateAliases: []string{"阿夕"}},
				ClaimDrafts: []CandidateClaimDraft{
					{DraftID: "c1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "team", Value: "A"}, NormalizedDraft: "team=A", SourceRole: "owner_explicit"},
					{DraftID: "c2", Payload: PersonClaimPayload{Kind: ClaimKindUserAssessment, Statement: "沟通直接", Stance: "endorsed"}, NormalizedDraft: "沟通直接", SourceRole: "owner_explicit"},
				},
				RelationshipDraft: &CandidateRelationshipDraft{DraftID: "r1", Status: RelStatusCurrent, Decision: "pending"},
				InteractionDraft:  &CandidateInteractionDraft{DraftID: "i1", EventKind: "kickoff", Headline: "项目启动会", Decision: "pending"},
				SourceMessageRef:  SourceRef{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1"},
			}
			stored, err := s.Propose(testOperator, cand)
			if err != nil {
				t.Fatalf("propose: %v", err)
			}
			got, ok, err := s.GetCandidate(testOperator, stored.CandidateID)
			if err != nil {
				t.Fatalf("get candidate: %v", err)
			}
			if !ok {
				t.Fatal("candidate not found after propose")
			}
			if len(got.ClaimDrafts) != 2 {
				t.Fatalf("expected 2 claim drafts, got %d", len(got.ClaimDrafts))
			}
			if got.RelationshipDraft == nil || got.RelationshipDraft.DraftID != "r1" {
				t.Fatalf("relationship draft lost: %+v", got.RelationshipDraft)
			}
			if got.InteractionDraft == nil || got.InteractionDraft.DraftID != "i1" {
				t.Fatalf("interaction draft lost: %+v", got.InteractionDraft)
			}

			// Approving all drafts materializes a person, a relationship, and an event.
			rec, err := s.ApproveDrafts(testOperator, stored.CandidateID, []string{"c1", "c2", "r1", "i1"})
			if err != nil {
				t.Fatalf("approve: %v", err)
			}
			if rec.PersonID == "" {
				t.Fatal("expected person id")
			}
			if !rec.CreatedRelationship {
				t.Fatal("expected CreatedRelationship=true")
			}
			if len(rec.MaterializedEventIDs) != 1 {
				t.Fatalf("expected 1 materialized event, got %d", len(rec.MaterializedEventIDs))
			}
			if len(rec.RemainingDraftIDs) != 0 {
				t.Fatalf("expected no remaining drafts, got %v", rec.RemainingDraftIDs)
			}
			rels, _ := s.ListRelationships(testOperator, rec.PersonID)
			if len(rels) != 1 || rels[0].Status != RelStatusCurrent {
				t.Fatalf("relationship not materialized: %+v", rels)
			}
			events, _ := s.ListEvents(testOperator, rec.PersonID)
			if len(events) != 1 || events[0].Headline != "项目启动会" {
				t.Fatalf("event not materialized: %+v", events)
			}
		})
	}
}

// TestRejectDrafts exercises the homologous per-card reject: each draft
// decides independently; only its decision flag and the candidate state change
// (nothing is materialized). When all drafts are decided the candidate resolves.
func TestRejectDrafts(t *testing.T) {
	stores := map[string]func(t *testing.T) PeopleMemoryStore{
		"file":  newTestFileStore,
		"redis": newTestRedisStore,
	}
	for name, makeStore := range stores {
		t.Run(name, func(t *testing.T) {
			s := makeStore(t)
			cand := &CaptureCandidate{
				PersonDraft:      &PersonIdentityDraft{DisplayName: "林夕"},
				ClaimDrafts:      []CandidateClaimDraft{{DraftID: "c1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "team", Value: "A"}, NormalizedDraft: "team=A", SourceRole: "owner_explicit"}, {DraftID: "c2", Payload: PersonClaimPayload{Kind: ClaimKindUserAssessment, Statement: "沟通直接"}, NormalizedDraft: "沟通直接", SourceRole: "owner_explicit"}},
				RelationshipDraft: &CandidateRelationshipDraft{DraftID: "r1", Status: RelStatusCurrent, Decision: "pending"},
				InteractionDraft:  &CandidateInteractionDraft{DraftID: "i1", EventKind: "kickoff", Headline: "启动会", Decision: "pending"},
				SourceMessageRef:  SourceRef{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1"},
			}
			stored, err := s.Propose(testOperator, cand)
			if err != nil {
				t.Fatalf("propose: %v", err)
			}

			// Reject one claim + the relationship: c1 and r1 dropped, c2+i1 pending.
			rej, err := s.RejectDrafts(testOperator, stored.CandidateID, []string{"c1", "r1"})
			if err != nil {
				t.Fatalf("rejectDrafts: %v", err)
			}
			if rej.State != CandPartiallyMaterialized {
				t.Fatalf("expected partially_materialized, got %s", rej.State)
			}
			if !equalStringSet(rej.RemainingDraftIDs, []string{"c2", "i1"}) {
				t.Fatalf("expected remaining [c2 i1], got %v", rej.RemainingDraftIDs)
			}
			if s := draftDecision(rej, "c1"); s != "rejected" {
				t.Fatalf("c1 decision = %q, want rejected", s)
			}
			if s := draftDecision(rej, "r1"); s != "rejected" {
				t.Fatalf("r1 decision = %q, want rejected", s)
			}
			// Nothing materialized by a reject.
			if people, _ := s.ListPeople(testOperator); len(people) != 0 {
				t.Fatalf("reject must not materialize a person, got %d", len(people))
			}

			// Reject the rest → all decided, none approved → candidate rejected.
			rej2, err := s.RejectDrafts(testOperator, stored.CandidateID, []string{"c2", "i1"})
			if err != nil {
				t.Fatalf("rejectDrafts 2: %v", err)
			}
			if rej2.State != CandRejected {
				t.Fatalf("expected rejected (no approvals), got %s", rej2.State)
			}
			if len(rej2.RemainingDraftIDs) != 0 {
				t.Fatalf("expected 0 remaining, got %v", rej2.RemainingDraftIDs)
			}
		})
	}
}

// TestRejectDraftsMixed verifies that a candidate with some approved and some
// rejected drafts resolves to materialized (a person exists once anything is
// approved), not rejected.
func TestRejectDraftsMixed(t *testing.T) {
	s := newTestFileStore(t)
	cand := &CaptureCandidate{
		PersonDraft:      &PersonIdentityDraft{DisplayName: "周杰"},
		ClaimDrafts:      []CandidateClaimDraft{{DraftID: "c1", Payload: PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "role", Value: "B"}, NormalizedDraft: "role=B", SourceRole: "owner_explicit"}, {DraftID: "c2", Payload: PersonClaimPayload{Kind: ClaimKindUserAssessment, Statement: "随意"}, NormalizedDraft: "随意", SourceRole: "owner_explicit"}},
		RelationshipDraft: &CandidateRelationshipDraft{DraftID: "r1", Status: RelStatusCurrent, Decision: "pending"},
		InteractionDraft:  &CandidateInteractionDraft{DraftID: "i1", EventKind: "call", Headline: "电话", Decision: "pending"},
		SourceMessageRef:  SourceRef{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1"},
	}
	stored, err := s.Propose(testOperator, cand)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	// Approve one claim.
	if _, err := s.ApproveDrafts(testOperator, stored.CandidateID, []string{"c1"}); err != nil {
		t.Fatalf("approve c1: %v", err)
	}
	// Reject the remaining three.
	rej, err := s.RejectDrafts(testOperator, stored.CandidateID, []string{"c2", "r1", "i1"})
	if err != nil {
		t.Fatalf("reject remaining: %v", err)
	}
	if rej.State != CandMaterialized {
		t.Fatalf("expected materialized (c1 approved), got %s", rej.State)
	}
	if people, _ := s.ListPeople(testOperator); len(people) != 1 {
		t.Fatalf("expected 1 materialized person, got %d", len(people))
	}
}

func draftDecision(c *CaptureCandidate, id string) string {
	for _, d := range c.ClaimDrafts {
		if d.DraftID == id {
			return d.Decision
		}
	}
	if c.RelationshipDraft != nil && c.RelationshipDraft.DraftID == id {
		return c.RelationshipDraft.Decision
	}
	if c.InteractionDraft != nil && c.InteractionDraft.DraftID == id {
		return c.InteractionDraft.Decision
	}
	return ""
}

func equalStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	for _, x := range b {
		if !set[x] {
			return false
		}
	}
	return true
}
