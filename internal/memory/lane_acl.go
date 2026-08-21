package memory

import "sync"

// SensitivityLevel is the 4-tier data-sensitivity classification
// (public/internal/private/restricted) with a// rank order so access can be gated by clearance. "" is treated as public for
// backward compatibility (pre-ACL entries have no sensitivity tag).
type SensitivityLevel string

const (
	SensPublic     SensitivityLevel = "public"
	SensInternal   SensitivityLevel = "internal"
	SensPrivate    SensitivityLevel = "private"
	SensRestricted SensitivityLevel = "restricted"
)

// SensitivityRank returns the numeric rank of a sensitivity level (higher =
// more restricted). Unknown / "" → public (0).
func SensitivityRank(s string) int {
	switch SensitivityLevel(s) {
	case SensPublic, "":
		return 0
	case SensInternal:
		return 1
	case SensPrivate:
		return 2
	case SensRestricted:
		return 3
	}
	return 0
}

// ValidSensitivity reports whether s is a known level (or empty).
func ValidSensitivity(s string) bool {
	switch SensitivityLevel(s) {
	case SensPublic, SensInternal, SensPrivate, SensRestricted, "":
		return true
	}
	return false
}

// collectionGrants maps collectionID -> operators allowed to see its entries
// regardless of ownership. A// non-empty CollectionID restricts visibility to its grantees + the entry's
// owner; an empty CollectionID imposes no collection restriction (visibility
// then depends only on owner scope + sensitivity clearance).
var (
	grantMu          sync.RWMutex
	collectionGrants = map[string][]string{}
)

// SetCollectionGrants installs the collection→operators grant map. Call once at
// startup from config; concurrent reads are safe.
func SetCollectionGrants(g map[string][]string) {
	grantMu.Lock()
	defer grantMu.Unlock()
	collectionGrants = g
}

func collectionAllowed(e *LaneEntry, operator string) bool {
	if e.CollectionID == "" {
		return true
	}
	grantMu.RLock()
	defer grantMu.RUnlock()
	for _, op := range collectionGrants[e.CollectionID] {
		if op == operator {
			return true
		}
	}
	return false
}

// operatorClearance maps operator -> sensitivity clearance (rank). The empty
// operator is the system/admin scope and sees everything (Restricted=3). Named
// operators default to Internal (1) unless overridden.
var (
	clearMu          sync.RWMutex
	operatorClearance = map[string]int{}
)

// SetOperatorClearance installs per-operator clearance overrides.
func SetOperatorClearance(c map[string]int) {
	clearMu.Lock()
	defer clearMu.Unlock()
	operatorClearance = c
}

// ClearanceFor returns the sensitivity clearance rank for an operator.
func ClearanceFor(operator string) int {
	if operator == "" {
		return 3
	}
	clearMu.RLock()
	defer clearMu.RUnlock()
	if c, ok := operatorClearance[operator]; ok {
		return c
	}
	return 1
}

// EntryVisible reports whether operator may see entry under the 4-level
// sensitivity model + collection grant ACL (// CollectionSensitivity + authorizedCollections). It combines orthogonal axes:
//  1. scope — owner scope (operatorMatches) OR an explicit collection grant
//             when the entry carries a non-empty CollectionID (a "" CollectionID
//             is owner-scoped only, no grant override)
//  2. clearance — entry sensitivity rank must not exceed operator clearance
func EntryVisible(e *LaneEntry, operator string) bool {
	owner := operatorMatches(e, operator)
	if e.CollectionID != "" {
		if !(owner || collectionAllowed(e, operator)) {
			return false
		}
	} else if !owner {
		return false
	}
	if SensitivityRank(e.Sensitivity) > ClearanceFor(operator) {
		return false
	}
	return true
}
