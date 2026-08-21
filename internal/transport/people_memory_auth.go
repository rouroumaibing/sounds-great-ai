package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"regexp"
	"strings"
	"time"

	"sounds-great-ai/internal/domains/threads/ports"
	"sounds-great-ai/internal/settings"
)

// wsCollapse normalizes whitespace the way the source resolver does:
// every run of whitespace becomes a single space and the string is lower-cased.
var wsCollapse = regexp.MustCompile(`\s+`)

// ThreadstoreAuthorizer is the cross-thread source authorizer. It confirms a
// memory item's SourceRef actually points at a thread/message the operator can
// access before the capture is allowed to materialize. Anything unverifiable is
// rejected (fail-closed): zero writes on doubt.
//
// Faithful per-field re-verification:
//   - thread exists and is not deleted
//   - the referenced message exists inside that thread
//   - (when an excerpt is supplied) the excerpt is actually contained in the
//     message body — prevents attributing text to a message it never contained
//   - (when an opaque ref/digest is supplied) it matches the digest derived
//     from the real message — prevents forging a source pointer
//
// SG schema gaps (documented, not silently skipped):
//   - The upstream design checks message-level tombstone / deleted; SG's message model has
//     no such flag, so only thread-level deletion is enforced here.
//   - The upstream design binds the message to ownerUserId. SG's threadstore is shared
//     across operators and is not yet owner-scoped, so operator-thread binding
//     cannot be enforced without a threadstore schema change. The existence +
//     excerpt + digest checks are the enforceable multi-user safety net.
//   TODO(people-memory): enforce ownerUserId binding once ThreadStore is operator-scoped.
type ThreadstoreAuthorizer struct {
	threads ports.IThreadStore
	msgs    ports.IMessageStore
}

// NewThreadstoreAuthorizer builds a threadstore-backed authorizer. If either
// store is nil (no threadstore wired), it degrades to AllowAll so the operator
// private single-user path still works.
func NewThreadstoreAuthorizer(threads ports.IThreadStore, msgs ports.IMessageStore) settings.SourceAuthorizer {
	if threads == nil || msgs == nil {
		return settings.AllowAllAuthorizer{}
	}
	return &ThreadstoreAuthorizer{threads: threads, msgs: msgs}
}

// AuthorizeSource verifies the referenced thread exists and is not deleted, and
// (when a message id is supplied) that the message belongs to that thread and
// passes the field-level re-verification below. Fail-closed on any doubt.
func (a *ThreadstoreAuthorizer) AuthorizeSource(_ context.Context, operatorID string, ref settings.SourceRef) (bool, error) {
	// No thread pointer: manual / operator / transcript sources are allowed.
	if ref.ThreadID == "" && ref.MessageID == "" {
		return true, nil
	}
	if ref.ThreadID != "" {
		all, err := a.threads.ListThreads()
		if err != nil {
			return false, err
		}
		found := false
		for _, t := range all {
			if t.ID == ref.ThreadID {
				if t.DeletedAt != nil {
					return false, nil // deleted thread -> fail closed
				}
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	if ref.MessageID != "" {
		// A message must be anchored to a verifiable thread.
		if ref.ThreadID == "" {
			return false, nil
		}
		msgs, err := a.msgs.GetByThread(ref.ThreadID, 10000)
		if err != nil {
			return false, nil
		}
		var hit *ports.Message
		for _, m := range msgs {
			if m.ID == ref.MessageID {
				hit = m
				break
			}
		}
		if hit == nil {
			return false, nil // message not found in thread -> fail closed
		}
		// Multi-user source re-verification.
		if !a.verifyMessageSource(operatorID, ref, hit) {
			return false, nil
		}
	}
	return true, nil
}

// verifyMessageSource performs the per-field re-verification
// of a captured SourceRef against the real message. Any mismatch fails closed
// (returns false) so a memory item can never be anchored to a message it does
// not honestly correspond to.
func (a *ThreadstoreAuthorizer) verifyMessageSource(operatorID string, ref settings.SourceRef, m *ports.Message) bool {
	// (1) owner/operator binding: SG threads are not yet partitioned by operator
	// at the threadstore layer, so operator-scoped lookups are not enforceable
	// here. Kept as a documented no-op guarded by a TODO so a future
	// operator-tagged threadstore can plug in without changing call sites.
	_ = operatorID

	// (2) excerpt containment: the operator-claimed bounded excerpt must actually
	// appear inside the message body. Prevents attributing text to a message it
	// was never part of (source_excerpt_mismatch).
	if ref.Excerpt != "" {
		if !strings.Contains(normalizeText(m.Content), normalizeText(ref.Excerpt)) {
			return false
		}
	}

	// (3) digest match: when the capturer supplied an opaque ref/digest, it must
	// equal the digest derived from the real message. Optional (ref.Ref == ""
	// passes) so captures predating digest emission stay compatible, but any
	// caller that sets a ref is held to the contract (source_digest_mismatch).
	if ref.Ref != "" {
		if ref.Ref != messageDigest(m) {
			return false
		}
	}

	return true
}

// normalizeText lower-cases and collapses whitespace, matching the
// source-resolver normalize() used for excerpt/digest comparison.
func normalizeText(s string) string {
	return strings.TrimSpace(wsCollapse.ReplaceAllString(strings.ToLower(s), " "))
}

// messageDigest derives the source ref digest from the real message. CLI
// adapters that emit a SourceRef.Ref should set it to messageDigest(msg) so the
// authorizer can cryptographically reject forged source pointers.
func messageDigest(m *ports.Message) string {
	h := sha256.New()
	io.WriteString(h, m.ID)
	io.WriteString(h, "\x00")
	io.WriteString(h, m.Content)
	io.WriteString(h, "\x00")
	io.WriteString(h, m.Timestamp.UTC().Format(time.RFC3339Nano))
	return hex.EncodeToString(h.Sum(nil))
}
