package pack

import "strings"

// CheckMentionPatternsUnique reports whether any of the given mention patterns
// collides (case-insensitively) with an existing breed's mention_patterns,
// excluding the breed identified by excludeID. It returns the conflicting
// pattern and the owning breed ID when a collision is found (ok == false).
//
// This backs the alias-uniqueness guard on the member-management endpoints: a
// member's @handle (mention pattern) must be globally unique across the pack.
func CheckMentionPatternsUnique(breeds []*BreedConfig, patterns []string, excludeID string) (conflictPattern, ownerID string, ok bool) {
	owned := make(map[string]string) // lowercased pattern -> owner breed ID
	for _, b := range breeds {
		if b == nil || b.ID == excludeID {
			continue
		}
		for _, p := range b.MentionPatterns {
			lp := strings.ToLower(strings.TrimSpace(p))
			if lp == "" {
				continue
			}
			owned[lp] = b.ID
		}
	}
	for _, p := range patterns {
		lp := strings.ToLower(strings.TrimSpace(p))
		if lp == "" {
			continue
		}
		if owner, found := owned[lp]; found {
			return p, owner, false
		}
	}
	return "", "", true
}
