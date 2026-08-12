package platform

import (
	"sort"
	"strings"

	"sounds-great-ai/pkg/pack"
)

// RoutingDecision is the result of parsing @mentions from a user message.
type RoutingDecision struct {
	TargetBreeds []string // breed IDs in mention order
	Strategy     string   // "single" | "serial"
	HasMentions  bool
	Warnings     []string
}

// Router parses @mentions from user messages using breed config mention_patterns.
// Simplified, self-contained implementation.
type Router struct {
	// patterns sorted by length descending (longest-first)
	patterns []mentionPattern
}

type mentionPattern struct {
	pattern string
	breedID string
}

// NewRouter creates a Router from breed configs.
func NewRouter(breeds map[string]*pack.BreedConfig) *Router {
	var patterns []mentionPattern
	for breedID, breed := range breeds {
		for _, p := range breed.MentionPatterns {
			patterns = append(patterns, mentionPattern{
				pattern: p,
				breedID: breedID,
			})
		}
	}
	// Sort by pattern length descending (longest-first)
	// Prevents "@边" matching before "@边牧"
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i].pattern) > len(patterns[j].pattern)
	})
	return &Router{patterns: patterns}
}

// Route parses @mentions from a message and returns a routing decision.
func (r *Router) Route(message string) RoutingDecision {
	if r == nil || len(r.patterns) == 0 {
		return RoutingDecision{
			TargetBreeds: []string{"bianmu"},
			Strategy:     "single",
		}
	}

	lowerMsg := strings.ToLower(message)

	type match struct {
		breedID  string
		position int
	}
	var matches []match
	seen := make(map[string]bool)

	for _, mp := range r.patterns {
		if seen[mp.breedID] {
			continue
		}
		idx := strings.Index(lowerMsg, strings.ToLower(mp.pattern))
		if idx >= 0 {
			matches = append(matches, match{breedID: mp.breedID, position: idx})
			seen[mp.breedID] = true
		}
	}

	// Sort by position in message (first-mentioned first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].position < matches[j].position
	})

	var targets []string
	for _, m := range matches {
		targets = append(targets, m.breedID)
	}

	switch len(targets) {
	case 0:
		return RoutingDecision{
			TargetBreeds: []string{"bianmu"},
			Strategy:     "single",
		}
	case 1:
		return RoutingDecision{
			TargetBreeds: targets,
			Strategy:     "single",
			HasMentions:  true,
		}
	default:
		return RoutingDecision{
			TargetBreeds: targets,
			Strategy:     "parallel",
			HasMentions:  true,
		}
	}
}
