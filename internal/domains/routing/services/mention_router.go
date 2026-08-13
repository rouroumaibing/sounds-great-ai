package services

import (
	"context"
	"sort"
	"strings"

	"sounds-great-ai/internal/domains/routing/ports"
	"sounds-great-ai/pkg/pack"
)

// MentionRouterService routes @mentions in a user message to one or more breeds.
// It is the D4-2 migration target for the routing logic that used to live in
// internal/platform/router.go — same algorithm, now behind the
// ports.IMentionRouter port so the transport layer depends on the domain, not
// the flat platform package.
type MentionRouterService struct {
	// patterns sorted by length descending (longest-first) so that "@边牧"
	// matches before "@边" when both are registered mention patterns.
	patterns []mentionPattern
}

type mentionPattern struct {
	pattern string
	breedID string
}

// NewMentionRouterService builds a router from breed configs.
func NewMentionRouterService(breeds map[string]*pack.BreedConfig) *MentionRouterService {
	var patterns []mentionPattern
	for breedID, breed := range breeds {
		for _, p := range breed.MentionPatterns {
			patterns = append(patterns, mentionPattern{pattern: p, breedID: breedID})
		}
	}
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i].pattern) > len(patterns[j].pattern)
	})
	return &MentionRouterService{patterns: patterns}
}

// Route parses @mentions from a message and returns a routing decision.
func (s *MentionRouterService) Route(_ context.Context, message string) (ports.RoutingDecision, error) {
	if s == nil || len(s.patterns) == 0 {
		// No breeds are registered (e.g. empty first-run catalog). Do not fall
		// back to a hard-coded breed that does not exist — surface an explicit
		// warning so the caller can prompt the user to add members first.
		return ports.RoutingDecision{
			TargetBreeds: []string{},
			Strategy:     "single",
			Warnings:     []string{"无可用犬，请先在成员管理添加成员"},
		}, nil
	}

	lowerMsg := strings.ToLower(message)

	type match struct {
		breedID  string
		position int
	}
	var matches []match
	seen := make(map[string]bool)

	for _, mp := range s.patterns {
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
		return ports.RoutingDecision{
			TargetBreeds: []string{"bianmu"},
			Strategy:     "single",
		}, nil
	case 1:
		return ports.RoutingDecision{
			TargetBreeds: targets,
			Strategy:     "single",
			HasMentions:  true,
		}, nil
	default:
		strategy := "parallel"
		if isSerialIntent(message) {
			// A serial intent (e.g. "@a → @b", "串联", "serial") means the
			// mentioned breeds form a pipeline: each dog's output is threaded
			// into the next (see WSHandler.executeSerial). This activates the
			// routeSerial worklist path that used to be dead code.
			strategy = "serial"
		}
		return ports.RoutingDecision{
			TargetBreeds: targets,
			Strategy:     strategy,
			HasMentions:  true,
		}, nil
	}
}

// serialMarkers are tokens that signal a serial pipeline intent when multiple
// breeds are mentioned in one message. Without any of these the default is a
// parallel fan-out (clowder-ai parity: route-serial vs route-parallel).
var serialMarkers = []string{
	"串联", "串行", "serial", "依次", "顺序",
	"→", "->", ">>",
}

// isSerialIntent reports whether the message asks for a serial pipeline rather
// than a parallel fan-out of the mentioned breeds.
func isSerialIntent(message string) bool {
	lower := strings.ToLower(message)
	for _, m := range serialMarkers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// Ensure MentionRouterService satisfies the port at compile time.
var _ ports.IMentionRouter = (*MentionRouterService)(nil)
