package capability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"

	"sounds-great-ai/pkg/pack"
)

// FilterResult is the structured output of sensitive_filter.
type FilterResult struct {
	Blocked         bool     `json:"blocked"`
	Reason          string   `json:"reason"`
	CleanedText     string   `json:"cleaned_text"`
	FlaggedPatterns []string `json:"flagged_patterns"`
}

// builtInSensitivePatterns compiled once at construction.
var builtInSensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[=:]\s*\S+`),
	regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
}

// SensitiveFilter filters text for keywords and sensitive patterns.
// Built-in patterns are compiled at construction; user patterns AND keywords
// are refreshed via Double-Checked Locking only when their respective
// CapabilityConfig entries change.
type SensitiveFilter struct {
	builtInPatterns []*regexp.Regexp

	mu sync.RWMutex

	userPatterns    []*regexp.Regexp
	userPatternsKey string

	keywords    []string
	keywordsKey string
}

// NewSensitiveFilter compiles built-in patterns. Panics on compile failure
// (programmer error — patterns are hardcoded).
func NewSensitiveFilter() (*SensitiveFilter, error) {
	if len(builtInSensitivePatterns) == 0 {
		return nil, errors.New("sensitive_filter: no built-in patterns")
	}
	return &SensitiveFilter{
		builtInPatterns: builtInSensitivePatterns,
	}, nil
}

func (s *SensitiveFilter) Name() string    { return "sensitive_filter" }
func (s *SensitiveFilter) Version() string { return "v1" }

func (s *SensitiveFilter) Init(ctx context.Context) error { return nil }
func (s *SensitiveFilter) Health() error                  { return nil }
func (s *SensitiveFilter) Close() error                   { return nil }

// Run filters input.Query (or each Previous output's Data) by keyword block
// + pattern flag.
func (s *SensitiveFilter) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	// Resolve user patterns + keywords (DCL refresh)
	if err := s.maybeRefreshConfig(input.CapabilityConfig); err != nil {
		return nil, err
	}

	text := input.Query
	if text == "" && input.Previous != nil {
		// Fallback: concatenate Previous outputs
		text = formatPreviousOutputs(input.Previous)
	}

	result := s.filter(text)
	return &pack.TaskOutput{
		Data: map[string]any{"filter": result},
	}, nil
}

// maybeRefreshConfig uses Double-Checked Locking to refresh both user-supplied
// patterns and keywords only when their respective config slices change.
// Both DCL sections share s.mu (single lock for both is acceptable and keeps
// filter()'s read section race-free under a single RLock).
func (s *SensitiveFilter) maybeRefreshConfig(cfg map[string]any) error {
	if cfg == nil {
		return nil
	}

	// --- Patterns DCL (recompile user-supplied regexps on change) ---
	if patternsRaw, ok := cfg["patterns"]; ok {
		patterns, err := toStringSlice(patternsRaw)
		if err != nil {
			return err
		}
		key := hashKey(patterns)

		// First check (RLock, fast path)
		s.mu.RLock()
		if s.userPatternsKey == key {
			s.mu.RUnlock()
		} else {
			s.mu.RUnlock()

			// Second check (Lock, recompile)
			s.mu.Lock()
			if s.userPatternsKey != key {
				compiled := make([]*regexp.Regexp, 0, len(patterns))
				for _, p := range patterns {
					re, err := regexp.Compile(p)
					if err != nil {
						s.mu.Unlock()
						return errors.New("sensitive_filter: invalid pattern " + p + ": " + err.Error())
					}
					compiled = append(compiled, re)
				}
				s.userPatterns = compiled
				s.userPatternsKey = key
			}
			s.mu.Unlock()
		}
	}

	// --- Keywords DCL (re-snapshot keywords on change) ---
	if keywordsRaw, ok := cfg["keywords"]; ok {
		keywords, err := toStringSlice(keywordsRaw)
		if err != nil {
			return err
		}
		key := hashKey(keywords)

		// First check (RLock, fast path)
		s.mu.RLock()
		if s.keywordsKey == key {
			s.mu.RUnlock()
		} else {
			s.mu.RUnlock()

			// Second check (Lock, re-snapshot)
			s.mu.Lock()
			if s.keywordsKey != key {
				// Copy to avoid aliasing the caller's slice.
				cp := make([]string, len(keywords))
				copy(cp, keywords)
				s.keywords = cp
				s.keywordsKey = key
			}
			s.mu.Unlock()
		}
	}

	return nil
}

func (s *SensitiveFilter) filter(text string) FilterResult {
	result := FilterResult{CleanedText: text}

	// Snapshot BOTH user patterns and keywords under a single RLock to avoid
	// TOCTOU between the two reads. Built-in patterns are immutable after
	// construction, so they don't need the lock — but we copy them here so the
	// pattern-matching loop runs lock-free.
	s.mu.RLock()
	allPatterns := append([]*regexp.Regexp{}, s.builtInPatterns...)
	allPatterns = append(allPatterns, s.userPatterns...)
	keywords := append([]string(nil), s.keywords...)
	s.mu.RUnlock()

	// 1. Keyword layer (block on first match)
	for _, kw := range keywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(kw)) {
			result.Blocked = true
			result.Reason = "keyword"
			result.CleanedText = strings.ReplaceAll(text, kw, "***")
			return result
		}
	}

	// 2. Pattern layer (flag, do not block)
	for _, re := range allPatterns {
		if re.MatchString(text) {
			result.FlaggedPatterns = append(result.FlaggedPatterns, re.String())
		}
	}
	if len(result.FlaggedPatterns) > 0 && result.Reason == "" {
		result.Reason = "pattern"
	}
	return result
}

// hashKey returns a deterministic hash of a string slice for change detection.
func hashKey(ss []string) string {
	b, _ := json.Marshal(ss)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func toStringSlice(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	switch s := v.(type) {
	case []string:
		return s, nil
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out, nil
	}
	return nil, errors.New("patterns: expected []string or []any")
}
