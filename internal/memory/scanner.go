package memory

import (
	"fmt"
	"regexp"
)

// ScanLevel is the tiered scanner depth/permission used for federated scanning
// (L0–L3). Higher levels scan deeper and are intended to require higher
// clearance. The gate (SecretScanGate) runs BEFORE any level's content is
// exposed, so the tier only widens the set of entries returned, never bypasses
// the secret barrier.
type ScanLevel int

const (
	// LevelL0: catalog/collection metadata only. No entry bodies are scanned
	// or returned — lowest privilege.
	LevelL0 ScanLevel = 0
	// LevelL1: approved canonical truth only.
	LevelL1 ScanLevel = 1
	// LevelL2: + pending candidates (decide-later content).
	LevelL2 ScanLevel = 2
	// LevelL3: + retired/forgotten/internal (deepest, most privileged).
	LevelL3 ScanLevel = 3
)

// LevelName returns a human-readable name for the scan level.
func (l ScanLevel) LevelName() string {
	switch l {
	case LevelL0:
		return "L0-meta"
	case LevelL1:
		return "L1-truth"
	case LevelL2:
		return "L2-pending"
	case LevelL3:
		return "L3-deep"
	default:
		return "L?-unknown"
	}
}

// ValidScanLevel reports whether l is a known tier.
func ValidScanLevel(l ScanLevel) bool {
	switch l {
	case LevelL0, LevelL1, LevelL2, LevelL3:
		return true
	}
	return false
}

// SecretScanGate enforces a fail-closed secret barrier. Before any content is
// scanned, written, or recalled, the gate is consulted; if the content matches
// a credential pattern the gate returns an error and the caller MUST deny the
// operation. Fail-closed: any unrecognized/ambiguous match denies by default.
//
// It is intentionally rule-based (regex) and dependency-free so it can run in
// the hot path without an LLM or network call.
type SecretScanGate struct {
	patterns []*regexp.Regexp
}

// defaultSecretPatterns are the credential shapes the gate screens for. They
// are deliberately conservative (long high-entropy tokens, explicit key labels)
// to keep false positives low while still catching the common leaks.
var defaultSecretPatterns = []*regexp.Regexp{
	// label = value style secrets
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|passwd|password|access[_-]?key|private[_-]?key|client[_-]?secret)\s*[=:]\s*\S+`),
	// bearer / basic auth headers
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`),
	// AWS access key id + secret
	regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`(?i)aws_secret_access_key\s*[=:]\s*\S+`),
	// generic 32+ char hex / base64 high-entropy blob
	regexp.MustCompile(`(?i)[A-Za-z0-9+/]{40,}={0,2}`),
	// PEM private key block
	regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----`),
	// connection strings with embedded credentials
	regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis|amqp|mongodb\+srv)://[^:@/\s]+:[^@/\s]+@`),
}

// NewSecretScanGate builds a gate with the default credential patterns.
func NewSecretScanGate() *SecretScanGate {
	return &SecretScanGate{patterns: defaultSecretPatterns}
}

// Check returns an error (fail-closed) if content looks like it contains a
// secret/credential. On any match the caller must DENY the operation; the gate
// never returns "allowed with warning" for a positive match.
func (g *SecretScanGate) Check(content string) error {
	for _, p := range g.patterns {
		if p.MatchString(content) {
			return errSecret("secret scan gate: content matches credential pattern %q", p.String())
		}
	}
	return nil
}

// secretError is the typed, fail-closed error returned by the gate.
type secretError string

func (e secretError) Error() string { return string(e) }

func errSecret(format string, args ...interface{}) error {
	if len(args) == 0 {
		return secretError(format)
	}
	return secretError(fmt.Sprintf(format, args...))
}

// Scanner performs tiered, fail-closed scans over a Collection. It always runs
// the SecretScanGate first (on every candidate entry's content) and denies the
// whole scan on the first secret hit. The scan level then selects how many
// entry classes are returned.
type Scanner struct {
	gate *SecretScanGate
}

// NewScanner builds a Scanner with a default secret gate.
func NewScanner() *Scanner {
	return &Scanner{gate: NewSecretScanGate()}
}

// Scan scans col at the requested level. It is fail-closed: if ANY entry that
// would be considered at this level contains a secret, the scan is denied and
// returns an error — no partial results leak. The returned entries respect the
// collection's ScanLevel cap (a request above the collection's permitted tier
// is clamped down). operator scopes results via the ACL layer ("" = system).
func (s *Scanner) Scan(col *Collection, level ScanLevel, operator string) ([]*LaneEntry, error) {
	if col == nil || col.Registry == nil {
		return nil, errSecret("scan: nil collection or registry")
	}
	if !ValidScanLevel(level) {
		return nil, errSecret("scan: invalid level %d", int(level))
	}
	// Clamp to the collection's permitted depth so a caller cannot over-scan a
	// restricted collection.
	if level > col.ScanLevel {
		level = col.ScanLevel
	}
	if level == LevelL0 {
		// Metadata only: nothing is scanned, gate not exercised, no bodies returned.
		return nil, nil
	}

	// Gather candidate entries for the requested tier, fail-closed on secrets.
	var out []*LaneEntry
	for _, t := range col.Registry.LaneTypes() {
		lane := col.Registry.Lane(t)
		if lane == nil {
			continue
		}
		candidates := s.candidatesFor(lane, level)
		for _, e := range candidates {
			if !visibleForOperator(e, operator) {
				continue
			}
			// Gate runs on the raw content BEFORE it is returned/scanned.
			if err := s.gate.Check(e.Content); err != nil {
				return nil, err // fail-closed: deny the entire scan
			}
			out = append(out, e)
		}
	}
	return out, nil
}

// candidatesFor returns the entry classes visible at the given tier, in stable
// order. L1=truth, L2=+pending, L3=+everything (incl. retired/forgotten).
func (s *Scanner) candidatesFor(lane *Lane, level ScanLevel) []*LaneEntry {
	switch level {
	case LevelL1:
		return lane.Truth()
	case LevelL2:
		return append(lane.Truth(), lane.Pending()...)
	default: // LevelL3
		return lane.All()
	}
}

// ScanContent is a convenience guard for callers that want to screen a single
// piece of content (e.g. before a write) without a Collection. It is fail-closed.
func (s *Scanner) ScanContent(content string) error {
	return s.gate.Check(content)
}
