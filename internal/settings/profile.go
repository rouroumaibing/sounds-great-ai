package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// RelationshipCapsule is the homologous "user-profile capsule": a
// persisted primer describing the long-term relationship between the operator
// and a specific relationship-keyed context (a family/platform/breed). Unlike
// breed config (which lives in dog-catalog.json), capsules live in their own
// directory — this keeps the cell-boundary discipline (relationship ≠
// breed config) so a stale capsule can never corrupt the breed catalog and the
// operator can hand-edit a capsule without touching identity config.
//
// The capsule survives restarts and context compression: when the prompt
// builder injects it, the dog "remembers who it is to the operator" even after
// a fresh spawn or an auto-compact wiped the conversation history.
type RelationshipCapsule struct {
	// RelationshipKey is the stable key the capsule is filed under (e.g.
	// "family-sounds-great-ai"). It is the same key declared on a breed's
	// relationship_key field, so a breed binds to its capsule by key.
	RelationshipKey string `json:"relationship_key"`
	// OwnerDog is the breed id that authored/maintains the capsule. Empty means
	// the operator authored it directly.
	OwnerDog string `json:"owner_dog,omitempty"`
	// Status: active | dormant | corrected.
	Status string `json:"status,omitempty"`
	// SourceRef records the provenance coordinates of the capsule (where the
	// knowledge came from), e.g. "operator:manual" or "dog:bianmu#thread-12".
	// This is the "provenance front-matter" discipline: a capsule must
	// carry its source so corrections are traceable.
	SourceRef string `json:"source_ref,omitempty"`
	// CorrectionPath records how a wrong assumption was corrected (empty if
	// never corrected). Keeps the relationship from re-learning a mistake.
	CorrectionPath string `json:"correction_path,omitempty"`
	// EvalApprovals / EvalRejections are the Approval-Hub counters
	// (homologous "养熟" governance): how often a proposed update for
	// this capsule was approved / rejected by the operator.
	EvalApprovals int `json:"eval_approvals,omitempty"`
	EvalRejections int `json:"eval_rejections,omitempty"`
	// ProposedAt / ApprovedAt / RejectedAt are governance timestamps (Unix ms).
	ProposedAt int64 `json:"proposed_at,omitempty"`
	ApprovedAt int64 `json:"approved_at,omitempty"`
	RejectedAt int64 `json:"rejected_at,omitempty"`
	// PendingProposal is a transport-only flag (never persisted): true when a
	// proposal file is awaiting operator approval.
	PendingProposal bool `json:"pending_proposal,omitempty"`
	// Body is the human-readable primer markdown: what the operator expects of
	// this relationship, preferences, do/don't, shared context.
	Body string `json:"body"`
	// UpdatedAt is the last-write Unix millisecond timestamp.
	UpdatedAt int64 `json:"updated_at,omitempty"`
}

// On-disk layout (homologous):
//
//	<ConfigRoot>/profiles/<operator>/relationship/<relationshipKey>-primer.md
//
// Single-operator form: the operator namespace is the leader id (the one human
// operator in SG). The relationship/ sub-directory holds one markdown file per
// relationship key, each with a YAML-ish front-matter header (flat key: value)
// followed by the primer body.
const (
	profilesDirName     = "profiles"
	relationshipDirName = "relationship"
	capsuleSuffix       = "-primer.md"
	// proposalSuffix marks a pending (not-yet-approved) capsule proposal file.
	proposalSuffix = "-proposal.md"
	// MaxCapsuleBodyLen mirrors KD-7: a relationship capsule primer is
	// hard-capped at 300 runes so it can never bloat the injected identity block
	// (the one-shot system prompt is fixed for the whole task, so an unbounded
	// capsule would silently inflate every spawn).
	MaxCapsuleBodyLen = 300
)

// ErrCapsuleTooLong is returned by WriteCapsule / WriteProposal when the body
// exceeds MaxCapsuleBodyLen.
var ErrCapsuleTooLong = errors.New("capsule body exceeds MaxCapsuleBodyLen")

// capsuleStrippedRuneCount returns the number of non-whitespace Unicode
// codepoints in body. This mirrors KD-7's 300-char capsule budget,
// which is measured as `body.replace(/\s/g,'').length` — i.e. visible chars
// (including punctuation / ASCII / symbols / CJK) with whitespace removed, NOT
// the raw rune count. The author chose a tight budget to force distillation:
// an unbounded capsule would silently inflate every injected identity block.
func capsuleStrippedRuneCount(body string) int {
	n := 0
	for _, r := range body {
		if unicode.IsSpace(r) {
			continue
		}
		n++
	}
	return n
}

// TruncateCapsuleBody defensively clamps a capsule body to the 300-stripped-rune
// budget so a capsule written before this rule (or edited by hand) can never
// blow the injected identity block. It keeps the leading content and drops
// trailing runes until the stripped count fits. Returns the (possibly) trimmed
// body, or "" if everything is whitespace.
func TruncateCapsuleBody(body string) string {
	if capsuleStrippedRuneCount(body) <= MaxCapsuleBodyLen {
		return body
	}
	runes := []rune(body)
	out := runes
	for len(out) > 0 {
		candidate := out[:len(out)-1]
		if capsuleStrippedRuneCount(string(candidate)) <= MaxCapsuleBodyLen {
			return string(candidate)
		}
		out = candidate
	}
	return ""
}

// ProfileRepository persists and resolves relationship capsules on disk.
// It is safe for concurrent use (a single RWMutex guards directory reads and
// writes). All writes are atomic (tmp + rename) using writeAtomic.
type ProfileRepository struct {
	root     string // <ConfigRoot>/profiles/<operator>/relationship
	operator string
	mu       sync.RWMutex
}

// NewProfileRepository creates (or opens) the capsule store under configRoot
// for the given operator namespace. The relationship directory is created if
// absent.
func NewProfileRepository(configRoot, operator string) *ProfileRepository {
	if operator == "" {
		operator = "operator"
	}
	root := filepath.Join(configRoot, profilesDirName, operator, relationshipDirName)
	_ = os.MkdirAll(root, 0o755)
	return &ProfileRepository{root: root, operator: operator}
}

// Operator returns the operator namespace this repository is scoped to.
func (r *ProfileRepository) Operator() string { return r.operator }

// sanitizeKey makes a relationship key safe for use as a single filename
// component (no path separators, no traversal). Any disallowed byte becomes
// '_'. This is mandatory: the key is untrusted-ish (breed config) and is used
// directly in a path.
func sanitizeKey(key string) string {
	var b strings.Builder
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "unnamed"
	}
	return out
}

func (r *ProfileRepository) capsulePath(key string) string {
	return filepath.Join(r.root, sanitizeKey(key)+capsuleSuffix)
}

// ReadCapsule loads a capsule by relationship key. The bool reports whether a
// capsule file exists; an error is returned only on I/O or parse failure.
func (r *ProfileRepository) ReadCapsule(key string) (*RelationshipCapsule, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.readCapsuleLocked(key)
}

func (r *ProfileRepository) readCapsuleLocked(key string) (*RelationshipCapsule, bool, error) {
	raw, err := os.ReadFile(r.capsulePath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read capsule %q: %w", key, err)
	}
	c, err := parseCapsule(key, raw)
	if err != nil {
		return nil, false, fmt.Errorf("parse capsule %q: %w", key, err)
	}
	return c, true, nil
}

// ReadCapsuleBody returns just the primer body for injection into a prompt.
// This is the method the prompt builder's ProfileReader interface consumes, so
// identity assembly does not depend on the settings package directly.
func (r *ProfileRepository) ReadCapsuleBody(key string) (string, bool, error) {
	c, ok, err := r.ReadCapsule(key)
	if err != nil || !ok {
		return "", ok, err
	}
	return c.Body, true, nil
}

// WriteCapsule persists (or updates) a capsule. UpdatedAt is set to now; the
// relationship key is taken from c.RelationshipKey (and used for the filename).
func (r *ProfileRepository) WriteCapsule(c *RelationshipCapsule) error {
	if c == nil || c.RelationshipKey == "" {
		return fmt.Errorf("capsule requires a non-empty relationship_key")
	}
	if n := capsuleStrippedRuneCount(c.Body); n > MaxCapsuleBodyLen {
		return fmt.Errorf("%w: body is %d visible runes (after stripping whitespace), limit is %d", ErrCapsuleTooLong, n, MaxCapsuleBodyLen)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c.UpdatedAt = time.Now().UnixMilli()
	data := renderCapsule(c)
	path := r.capsulePath(c.RelationshipKey)
	// Atomic write (tmp + rename); capsules are operator-authored and
	// low-risk, so no .bak snapshot is taken (unlike the breed catalog).
	return writeAtomicRaw(path, data, 0o644)
}

// DeleteCapsule removes a capsule file. Deleting a non-existent capsule is a
// no-op (idempotent), not an error.
func (r *ProfileRepository) DeleteCapsule(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	err := os.Remove(r.capsulePath(key))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete capsule %q: %w", key, err)
	}
	return nil
}

// ListCapsules returns the relationship keys currently stored (sorted).
func (r *ProfileRepository) ListCapsules() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, err := os.ReadDir(r.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list capsules: %w", err)
	}
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, capsuleSuffix) {
			continue
		}
		keys = append(keys, strings.TrimSuffix(name, capsuleSuffix))
	}
	sort.Strings(keys)
	return keys, nil
}

// renderCapsule serializes a capsule to markdown with a flat front-matter
// header. Front-matter keys are limited to safe scalars (strings / ints), so a
// tiny hand-rolled parser (parseCapsule) is sufficient — no YAML dependency.
func renderCapsule(c *RelationshipCapsule) []byte {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("relationship_key: %s\n", c.RelationshipKey))
	sb.WriteString(fmt.Sprintf("owner_dog: %s\n", c.OwnerDog))
	sb.WriteString(fmt.Sprintf("status: %s\n", c.Status))
	sb.WriteString(fmt.Sprintf("source_ref: %s\n", c.SourceRef))
	sb.WriteString(fmt.Sprintf("correction_path: %s\n", c.CorrectionPath))
	sb.WriteString(fmt.Sprintf("eval_approvals: %d\n", c.EvalApprovals))
	sb.WriteString(fmt.Sprintf("eval_rejections: %d\n", c.EvalRejections))
	sb.WriteString(fmt.Sprintf("proposed_at: %d\n", c.ProposedAt))
	sb.WriteString(fmt.Sprintf("approved_at: %d\n", c.ApprovedAt))
	sb.WriteString(fmt.Sprintf("rejected_at: %d\n", c.RejectedAt))
	sb.WriteString(fmt.Sprintf("updated_at: %d\n", c.UpdatedAt))
	sb.WriteString("---\n\n")
	sb.WriteString(c.Body)
	if !strings.HasSuffix(c.Body, "\n") {
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}

// parseCapsule reads the markdown+front-matter format produced by renderCapsule.
// It tolerates a missing body and unknown header keys (ignored).
func parseCapsule(key string, raw []byte) (*RelationshipCapsule, error) {
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		// No front-matter: treat the whole file as the body (lenient import of
		// a hand-written primer without metadata).
		return &RelationshipCapsule{
			RelationshipKey: key,
			Status:          "active",
			Body:            strings.TrimSpace(text),
			UpdatedAt:       0,
		}, nil
	}
	// Find the closing delimiter of the front-matter block.
	rest := text[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("unterminated front-matter")
	}
	header := rest[:end]
	body := rest[end+4:]
	// Strip a leading newline after the closing delimiter.
	body = strings.TrimPrefix(body, "\n")

	c := &RelationshipCapsule{RelationshipKey: key, Status: "active"}
	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		switch k {
		case "owner_dog":
			c.OwnerDog = v
		case "status":
			if v != "" {
				c.Status = v
			}
		case "source_ref":
			c.SourceRef = v
		case "correction_path":
			c.CorrectionPath = v
		case "eval_approvals":
			fmt.Sscanf(v, "%d", &c.EvalApprovals)
		case "eval_rejections":
			fmt.Sscanf(v, "%d", &c.EvalRejections)
		case "proposed_at":
			fmt.Sscanf(v, "%d", &c.ProposedAt)
		case "approved_at":
			fmt.Sscanf(v, "%d", &c.ApprovedAt)
		case "rejected_at":
			fmt.Sscanf(v, "%d", &c.RejectedAt)
		case "updated_at":
			var n int64
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
				c.UpdatedAt = n
			}
		case "relationship_key":
			// Prefer the declared key, but fall back to the filename key.
			if v != "" {
				c.RelationshipKey = v
			}
		}
	}
	c.Body = strings.TrimSpace(body)
	return c, nil
}
