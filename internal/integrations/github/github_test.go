package github

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
)

func hmacSig(payload []byte, secret string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	return "sha1=" + hex.EncodeToString(mac.Sum(nil))
}

func samplePR(action, state string, merged bool) []byte {
	return mustJSON(map[string]any{
		"type":   "pull_request",
		"action": action,
		"repository": map[string]any{"full_name": "acme/widgets"},
		"sender":     map[string]any{"login": "alice"},
		"pull_request": map[string]any{
			"number": 7,
			"state":  state,
			"title":  "add feature",
			"merged": merged,
		},
	})
}

func mustJSON(v map[string]any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestWebhook_ParsePR(t *testing.T) {
	ev, err := ParseWebhook(samplePR("opened", "open", false))
	if err != nil {
		t.Fatal(err)
	}
	if ev.PRNumber != 7 || ev.Repo != "acme/widgets" || ev.PRState != "open" || ev.Sender != "alice" {
		t.Fatalf("parse mismatch: %+v", ev)
	}
}

func TestWebhook_ParseMerged(t *testing.T) {
	ev, _ := ParseWebhook(samplePR("closed", "closed", true))
	if ev.PRState != "merged" {
		t.Fatalf("merged PR must report merged, got %q", ev.PRState)
	}
}

func TestWebhook_VerifySignature(t *testing.T) {
	secret := "whsec"
	payload := samplePR("opened", "open", false)
	// compute the correct signature
	mac := hmacSig(payload, secret)
	if err := VerifySignature(payload, mac, secret); err != nil {
		t.Fatalf("valid signature should verify: %v", err)
	}
	if err := VerifySignature(payload, "sha1=deadbeef", secret); !errors.Is(err, ErrBadSignature) {
		t.Fatal("wrong signature must fail-closed")
	}
	if err := VerifySignature(payload, "", secret); !errors.Is(err, ErrBadSignature) {
		t.Fatal("empty signature must fail-closed")
	}
}

func TestPRTracker_Tracks(t *testing.T) {
	tr := NewPRTracker()
	open, _ := ParseWebhook(samplePR("opened", "open", false))
	tr.Apply(open)
	closed, _ := ParseWebhook(samplePR("closed", "closed", false))
	tr.Apply(closed)

	if p, ok := tr.Get("acme/widgets", 7); !ok || p.State != "closed" {
		t.Fatalf("latest state should be closed: %+v", p)
	}
	if opens := tr.OpenForRepo("acme/widgets"); len(opens) != 0 {
		t.Fatalf("no open PRs expected, got %d", len(opens))
	}
	// a fresh open one
	ev2, _ := ParseWebhook(samplePR("opened", "open", false))
	ev2.PRNumber = 8
	tr.Apply(ev2)
	if opens := tr.OpenForRepo("acme/widgets"); len(opens) != 1 {
		t.Fatalf("one open PR expected, got %d", len(opens))
	}
}
