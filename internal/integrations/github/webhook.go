// Package github implements the GitHub integration transport (roadmap P1-A):
// inbound webhook parsing with fail-closed HMAC signature verification and PR
// state tracking that the task board (P0-5) can consume.
package github

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// WebhookEvent is a normalized GitHub webhook event.
type WebhookEvent struct {
	Type       string // "pull_request" | "push" | "issues" ...
	Action     string // opened | closed | synchronized ...
	Repo       string
	PRNumber   int
	PRState    string // open | closed | merged
	Title      string
	Sender     string
	DeliveryID string // X-GitHub-Delivery, for idempotency
}

// ErrBadSignature is returned when a webhook signature fails verification.
var ErrBadSignature = errors.New("github: bad webhook signature (fail-closed)")

// rawPullRequest is the subset of a pull_request event we parse.
type rawPullRequest struct {
	Action string `json:"action"`
	Repo   struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
	PullRequest struct {
		Number int    `json:"number"`
		State  string `json:"state"` // "open" | "closed"
		Title  string `json:"title"`
		Merged bool   `json:"merged"`
	} `json:"pull_request"`
}

// ParseWebhook extracts a normalized event. Signatures are NOT verified here;
// callers must VerifySignature first. Non-PR events return a best-effort event
// with Type set but PRNumber 0.
func ParseWebhook(payload []byte) (*WebhookEvent, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &probe); err != nil {
		return nil, err
	}
	ev := &WebhookEvent{Type: probe.Type}
	if probe.Type != "pull_request" {
		return ev, nil
	}
	var rp rawPullRequest
	if err := json.Unmarshal(payload, &rp); err != nil {
		return nil, err
	}
	ev.Action = rp.Action
	ev.Repo = rp.Repo.FullName
	ev.Sender = rp.Sender.Login
	ev.PRNumber = rp.PullRequest.Number
	ev.Title = rp.PullRequest.Title
	ev.PRState = rp.PullRequest.State
	if rp.PullRequest.Merged {
		ev.PRState = "merged"
	}
	return ev, nil
}

// VerifySignature validates a GitHub X-Hub-Signature (sha1=hex) using the
// secret. Fail-closed: an empty/mismatched signature returns ErrBadSignature.
func VerifySignature(payload []byte, signature, secret string) error {
	if signature == "" {
		return ErrBadSignature
	}
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	want := "sha1=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return ErrBadSignature
	}
	return nil
}
