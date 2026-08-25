package memory

import (
	"strings"
	"sync"
)

// ProactiveCandidate is the deterministic, lane-neutral output of the proactive
// detection layer (F282 Phase A). It reports ONLY the statistical fact that a
// surface form recurs across threads without a precise registry entry — it
// carries no lane预判 and no "should record" conclusion (KD-8: give data, not
// conclusions). Classification (person/entity/taste) is left to the in-context
// cat, which has the full context.
type ProactiveCandidate struct {
	Surface      string `json:"surface"`      // normalized repeated phrase/person name
	ThreadCount  int    `json:"thread_count"` // distinct threads it appeared in
	MessageCount int    `json:"message_count"`
	LastThreadID string `json:"last_thread_id"`
	OwnerUserID  string `json:"owner_user_id"`
}

// ProactiveDetector is SG's mechanical detection layer (F282). It is a pure
// inverted index over observed human-input messages — zero inference, zero
// classification. It answers "which surface forms recur across ≥2 threads and
// are NOT in the resolvable registry set?" so the in-context cat gets a
// lane-neutral nudge instead of the system guessing. Owner-scoped: candidates
// are partitioned per operator (fail-closed against cross-owner leakage).
type ProactiveDetector struct {
	mu        sync.Mutex
	owner     string
	registry  *EntityRegistry // used only to subtract already-known surfaces
	// per-surface: thread set + message count + last thread
	surfThread map[string]map[string]bool
	surfMsg    map[string]int
	surfLast   map[string]string
	dormant    map[string]bool // rejected surfaces are suppressed, not re-nudged
}

// NewProactiveDetector creates a detector scoped to one owner. A nil registry
// is acceptable (detection simply never subtracts known surfaces).
func NewProactiveDetector(ownerUserID string, reg *EntityRegistry) *ProactiveDetector {
	return &ProactiveDetector{
		owner:      ownerUserID,
		registry:   reg,
		surfThread: make(map[string]map[string]bool),
		surfMsg:    make(map[string]int),
		surfLast:   make(map[string]string),
		dormant:    make(map[string]bool),
	}
}

// proactiveStopWords excludes trivial high-frequency tokens from candidacy
// (F260 AC-B9 spirit: laughter/particle families must never nudge, and common
// English function words are not "recurring named surfaces"). The detector is
// meant to surface *named* recurring surfaces (proper nouns, project names),
// not every content word — so function words are filtered out. CJK-light: we
// only filter pure repetition and very short tokens; phrase extraction is
// deliberately conservative (no segmentation guesses).
var proactiveStopWords = map[string]bool{
	// CJK particles / laughter families (KD-8 red line).
	"哈哈": true, "哈哈哈": true, "哈哈哈哈": true, "呵呵": true, "嗯": true,
	"好的": true, "好的好的": true, "对": true, "是的": true, "嗯嗯": true,
	// English function words (not "named surfaces").
	"the": true, "and": true, "or": true, "but": true, "for": true, "on": true,
	"in": true, "at": true, "to": true, "of": true, "is": true, "are": true,
	"was": true, "were": true, "has": true, "have": true, "had": true, "this": true,
	"that": true, "from": true, "your": true, "our": true, "their": true, "what": true,
	"when": true, "how": true, "who": true, "why": true, "which": true, "not": true,
	"no": true, "yes": true, "ok": true, "good": true, "great": true, "thanks": true,
	"please": true, "can": true, "will": true, "would": true, "should": true,
	"could": true, "do": true, "does": true, "did": true, "been": true, "being": true,
	"by": true, "as": true, "if": true, "then": true, "else": true, "we": true,
	"you": true, "they": true, "he": true, "she": true, "it": true, "my": true,
	"me": true, "with": true, "about": true, "off": true, "owns": true, "signed": true,
	"release": true, "payments": true, "lane": true, "joined": true, "standup": true,
	"meeting": true, "roadmap": true, "flagged": true, "latency": true, "bug": true,
	"suggested": true, "retention": true, "experiment": true,
}

// extractSurfaces pulls candidate surface forms from a single message without
// any semantic classification. It emits: (a) ascii word tokens ≥3 chars,
// (b) quoted/ Capitalized person-like names, (c) CJK runs ≥2 chars. This is a
// mechanical tokenizer, not a classifier (KD-8 / no-classifier red line).
func extractSurfaces(msg string) []string {
	low := strings.ToLower(strings.TrimSpace(msg))
	if low == "" {
		return nil
	}
	var out []string
	// ASCII words.
	for _, tok := range strings.FieldsFunc(low, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == '，' || r == '。' ||
			r == ',' || r == '.' || r == '！' || r == '!' || r == '？' || r == '?'
	}) {
		if len([]rune(tok)) >= 3 && !proactiveStopWords[tok] {
			out = append(out, tok)
		}
	}
	// CJK runs (greedy ≥2 runes), excluding pure stopword repeats.
	var run strings.Builder
	flush := func() {
		s := run.String()
		if len([]rune(s)) >= 2 && !proactiveStopWords[s] {
			out = append(out, s)
		}
		run.Reset()
	}
	for _, r := range msg {
		if r >= 0x4E00 && r <= 0x9FFF { // CJK unified ideograph range
			run.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// Observe records one human-input message against a thread. Private threads are
// caller-excluded (the caller must not pass private-thread messages — F282
// AC-A4 privacy partition is an implementation pre-condition, enforced by the
// caller passing only authorized messages). Repeated surfaces across distinct
// threads accrue; dormant surfaces are ignored.
func (d *ProactiveDetector) Observe(threadID, message string) {
	if d.owner == "" {
		return // fail-closed: no owner → no detection
	}
	surfaces := extractSurfaces(message)
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range surfaces {
		if d.dormant[s] {
			continue
		}
		if d.surfThread[s] == nil {
			d.surfThread[s] = make(map[string]bool)
		}
		if !d.surfThread[s][threadID] {
			d.surfThread[s][threadID] = true
		}
		d.surfMsg[s]++
		d.surfLast[s] = threadID
	}
}

// Candidates returns lane-neutral proactive candidates: surfaces appearing in
// ≥2 distinct threads, not yet in the resolvable registry set, and not dormant.
// minThreads is the cross-thread threshold (Alden fix: ≥2 threads).
func (d *ProactiveDetector) Candidates(minThreads int) []*ProactiveCandidate {
	if minThreads < 1 {
		minThreads = 2
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []*ProactiveCandidate
	for s, threads := range d.surfThread {
		if d.dormant[s] {
			continue
		}
		if len(threads) < minThreads {
			continue
		}
		// Subtract already-known surfaces (registry set): if it resolves for
		// this owner, it is no longer a "not recorded" candidate.
		if d.registry != nil && d.registry.Resolve(d.owner, s) != nil {
			continue
		}
		out = append(out, &ProactiveCandidate{
			Surface:      s,
			ThreadCount:  len(threads),
			MessageCount: d.surfMsg[s],
			LastThreadID: d.surfLast[s],
			OwnerUserID:  d.owner,
		})
	}
	return out
}

// Dismiss marks a surface dormant so it stops producing candidates (F282
// AC-A3: rejected candidates must not re-nudge). Idempotent.
func (d *ProactiveDetector) Dismiss(surface string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dormant[normalizeAlias(surface)] = true
}
