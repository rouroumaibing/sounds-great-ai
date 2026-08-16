package settings

import (
	"math"
	"strings"
	"testing"
)

// TestRecallDrillBudgetDiscipline exercises the on-demand drill across both the
// file- and Redis-backed stores: it verifies all three item kinds (claim /
// relationship / event) project their verbatim backing, that the per-person
// per-turn budget (3) is enforced, and that missing/wrong-window items fail
// closed with not_available.
func TestRecallDrillBudgetDiscipline(t *testing.T) {
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
					{
						DraftID:         "c1",
						Payload:         PersonClaimPayload{Kind: ClaimKindReportedFact, Predicate: "team", Value: "A"},
						NormalizedDraft: "team=A",
						SourceRole:      "owner_explicit",
						EvidenceExcerpt: "林夕在 A 团队",
					},
				},
				RelationshipDraft: &CandidateRelationshipDraft{DraftID: "r1", Status: RelStatusCurrent, Decision: "pending"},
				InteractionDraft:  &CandidateInteractionDraft{DraftID: "i1", EventKind: "kickoff", Headline: "项目启动会", Decision: "pending"},
				SourceMessageRef:  SourceRef{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1", Excerpt: "林夕在 A 团队"},
			}
			stored, err := s.Propose(testOperator, cand)
			if err != nil {
				t.Fatalf("propose: %v", err)
			}
			rec, err := s.ApproveDrafts(testOperator, stored.CandidateID, []string{"c1", "r1", "i1"})
			if err != nil {
				t.Fatalf("approve: %v", err)
			}
			pid := rec.PersonID

			claims, _ := s.ListClaims(testOperator, pid)
			if len(claims) != 1 {
				t.Fatalf("expected 1 claim, got %d", len(claims))
			}
			claimID := claims[0].ClaimID
			recordedAt := claims[0].RecordedAt

			rels, _ := s.ListRelationships(testOperator, pid)
			relID := rels[0].RelationshipID
			events, _ := s.ListEvents(testOperator, pid)
			eventID := events[0].EventID

			wide := PeopleMemoryDrillInput{
				TurnID:         "turn-1",
				PersonID:       pid,
				TimeWindowFrom: 1,
				TimeWindowTo:   math.MaxInt64,
			}

			// claim drill -> ok with verbatim text + source ref
			res, err := s.RecallDrill(testOperator, PeopleMemoryDrillInput{
				TurnID: wide.TurnID, PersonID: pid, ItemKind: DrillItemClaim, ItemID: claimID,
				TimeWindowFrom: wide.TimeWindowFrom, TimeWindowTo: wide.TimeWindowTo,
			})
			if err != nil {
				t.Fatalf("drill claim: %v", err)
			}
			if res.Status != "ok" || res.Text == "" || res.EstimatedTokens <= 0 {
				t.Fatalf("claim drill: %+v", res)
			}
			if res.SourceRef.MessageID != "m1" {
				t.Fatalf("claim drill source ref mismatch: %+v", res.SourceRef)
			}

			// relationship drill -> ok
			res, err = s.RecallDrill(testOperator, PeopleMemoryDrillInput{
				TurnID: wide.TurnID, PersonID: pid, ItemKind: DrillItemRelationship, ItemID: relID,
				TimeWindowFrom: wide.TimeWindowFrom, TimeWindowTo: wide.TimeWindowTo,
			})
			if err != nil {
				t.Fatalf("drill relationship: %v", err)
			}
			if res.Status != "ok" {
				t.Fatalf("relationship drill: %+v", res)
			}

			// event drill -> ok (3rd call for this person this turn)
			res, err = s.RecallDrill(testOperator, PeopleMemoryDrillInput{
				TurnID: wide.TurnID, PersonID: pid, ItemKind: DrillItemEvent, ItemID: eventID,
				TimeWindowFrom: wide.TimeWindowFrom, TimeWindowTo: wide.TimeWindowTo,
			})
			if err != nil {
				t.Fatalf("drill event: %v", err)
			}
			if res.Status != "ok" {
				t.Fatalf("event drill: %+v", res)
			}

			// 4th call this turn: per-person-per-turn cap (3) exceeded
			res, err = s.RecallDrill(testOperator, PeopleMemoryDrillInput{
				TurnID: wide.TurnID, PersonID: pid, ItemKind: DrillItemClaim, ItemID: claimID,
				TimeWindowFrom: wide.TimeWindowFrom, TimeWindowTo: wide.TimeWindowTo,
			})
			if err != nil {
				t.Fatalf("drill 4th: %v", err)
			}
			if res.Status != "budget_exceeded" {
				t.Fatalf("expected budget_exceeded, got %+v", res)
			}

			// not_available: bogus item id
			res, err = s.RecallDrill(testOperator, PeopleMemoryDrillInput{
				TurnID: "turn-2", PersonID: pid, ItemKind: DrillItemClaim, ItemID: "nope",
				TimeWindowFrom: 1, TimeWindowTo: math.MaxInt64,
			})
			if err != nil {
				t.Fatalf("drill bogus: %v", err)
			}
			if res.Status != "not_available" {
				t.Fatalf("expected not_available for bogus id, got %+v", res)
			}

			// not_available: time window excludes the recorded-at time
			res, err = s.RecallDrill(testOperator, PeopleMemoryDrillInput{
				TurnID: "turn-2", PersonID: pid, ItemKind: DrillItemClaim, ItemID: claimID,
				TimeWindowFrom: recordedAt + 1, TimeWindowTo: recordedAt + 100000,
			})
			if err != nil {
				t.Fatalf("drill window: %v", err)
			}
			if res.Status != "not_available" {
				t.Fatalf("expected not_available for out-of-window, got %+v", res)
			}

			// not_available: person hard-forgotten (no longer active)
			if _, err := s.HardForget(testOperator, pid); err != nil {
				t.Fatalf("hardforget: %v", err)
			}
			res, err = s.RecallDrill(testOperator, PeopleMemoryDrillInput{
				TurnID: "turn-3", PersonID: pid, ItemKind: DrillItemClaim, ItemID: claimID,
				TimeWindowFrom: 1, TimeWindowTo: math.MaxInt64,
			})
			if err != nil {
				t.Fatalf("drill after forget: %v", err)
			}
			if res.Status != "not_available" {
				t.Fatalf("expected not_available after hard-forget, got %+v", res)
			}
		})
	}
}

// TestRecallDrillProjectionTruncated verifies the per-call token cap: a very
// long claim is projected truncated to <= PeopleMemoryDrillMaxTokensPerCall
// tokens (maxDrillTokensPerCall=500), with an ellipsis marker.
func TestRecallDrillProjectionTruncated(t *testing.T) {
	s := newTestFileStore(t)
	cand := &CaptureCandidate{
		PersonDraft: &PersonIdentityDraft{DisplayName: "长文本", PrivateAliases: []string{"lt"}},
		ClaimDrafts: []CandidateClaimDraft{
			{
				DraftID:         "c1",
				Payload:         PersonClaimPayload{Kind: ClaimKindUserAssessment, Statement: strings.Repeat("好", 4000), Stance: "endorsed"},
				NormalizedDraft: "long",
				SourceRole:      "owner_explicit",
			},
		},
		SourceMessageRef: SourceRef{SourceKind: "message_text", ThreadID: "t1", MessageID: "m1"},
	}
	stored, err := s.Propose(testOperator, cand)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	rec, err := s.ApproveDrafts(testOperator, stored.CandidateID, []string{"c1"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	claims, _ := s.ListClaims(testOperator, rec.PersonID)
	claimID := claims[0].ClaimID

	res, err := s.RecallDrill(testOperator, PeopleMemoryDrillInput{
		TurnID: "turn-x", PersonID: rec.PersonID, ItemKind: DrillItemClaim, ItemID: claimID,
		TimeWindowFrom: 1, TimeWindowTo: math.MaxInt64,
	})
	if err != nil {
		t.Fatalf("drill: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("expected ok, got %+v", res)
	}
	if res.EstimatedTokens > PeopleMemoryDrillMaxTokensPerCall {
		t.Fatalf("projection exceeded cap: %d > %d", res.EstimatedTokens, PeopleMemoryDrillMaxTokensPerCall)
	}
	if !strings.HasSuffix(res.Text, "…") {
		t.Fatalf("expected truncated text ending in ellipsis, got %q", res.Text)
	}
}
