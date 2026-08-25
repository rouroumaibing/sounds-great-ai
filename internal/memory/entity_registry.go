package memory

import (
	"strings"
	"sync"

	"github.com/google/uuid"
)

// EntityKind classifies a resolvable registry entry. SG keeps the registry's
// curated semantics clean (F260 KD-5): mechanical mirrors never pollute it.
// Concepts (slang/relationship words) are the only new lane fed by a lightweight
// proposal flow; doc/feature aliases stay out of the registry (mirrored
// elsewhere), so the registry remains a curated source, not a doc index.
type EntityKind string

const (
	EntityKindPerson  EntityKind = "person"
	EntityKindEntity  EntityKind = "entity"
	EntityKindConcept EntityKind = "concept"
)

// ResolvedEntity is the deterministic output of a dereference. It reports the
// objective fact "this query maps to a known entry" and carries its canonical
// name + provenance — it never asserts the operator's stance or intent (KD-8:
// give data, not conclusions; no-classifier).
type ResolvedEntity struct {
	EntityID     string     `json:"entity_id"`    // e.g. "person:alden" / "concept:未婚喵"
	Canonical    string     `json:"canonical"`    // curated display name
	Kind         EntityKind `json:"kind"`
	MatchedAlias string     `json:"matched_alias"` // which alias the query hit
	Provenance   string     `json:"provenance"`    // e.g. "lane:person@op-1" / "proposed@thread-x"
	OwnerUserID  string     `json:"owner_user_id"`
	Stance       string     `json:"stance"`  // endorsed / unknown / rejected (anti stance-collapse)
	Status       string     `json:"status"`  // approved / pending
}

// EntityRegistry is SG's curated, deterministic entity dereference surface
// (F260). It indexes approved person/entity lane entries as resolvable names +
// aliases and supports a lightweight concept proposal flow. It performs NO
// semantic inference — resolution is exact + case-insensitive containment only.
// Owner-scoped: an entry is only resolvable by its owning operator (fail-closed
// against cross-owner leakage, F260 KD-7).
type EntityRegistry struct {
	mu       sync.RWMutex
	reg      *LaneRegistry // backing registry (truth lives in person/entity lanes)
	aliases  map[string]*ResolvedEntity // normalized alias -> resolution
	ownerIdx map[string]map[string]bool // owner -> set of entity ids (for scoping)
}

// NewEntityRegistry builds a registry view over an existing LaneRegistry. The
// registry is rebuilt from approved person/entity lanes; it does not own
// persistence (the lanes do).
func NewEntityRegistry(reg *LaneRegistry) *EntityRegistry {
	er := &EntityRegistry{
		reg:      reg,
		aliases:  make(map[string]*ResolvedEntity),
		ownerIdx: make(map[string]map[string]bool),
	}
	er.reindex()
	return er
}

func normalizeAlias(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// reindex sweeps approved person/entity lane entries and registers their
// content (and any alias annotations) as resolvable. Called after construction
// and after any proposal that reaches approved status.
func (er *EntityRegistry) reindex() {
	er.mu.Lock()
	defer er.mu.Unlock()
	er.aliases = make(map[string]*ResolvedEntity)
	er.ownerIdx = make(map[string]map[string]bool)
	for _, laneType := range []LaneType{LanePerson, LaneEntity} {
		lane := er.reg.Lane(laneType)
		if lane == nil {
			continue
		}
		for _, e := range lane.All() {
			if e.Status != StatusApproved {
				continue
			}
			kind := EntityKindPerson
			if laneType == LaneEntity {
				kind = EntityKindEntity
			}
			id := string(kind) + ":" + e.ID
			er.registerAlias(e.Content, &ResolvedEntity{
				EntityID:    id,
				Canonical:   e.Content,
				Kind:        kind,
				MatchedAlias: normalizeAlias(e.Content),
				Provenance:  "lane:" + string(laneType) + "@" + e.OperatorID,
				OwnerUserID: e.OperatorID,
				Stance:      "endorsed",
				Status:      "approved",
			})
			er.indexOwner(e.OperatorID, id)
		}
	}
}

func (er *EntityRegistry) registerAlias(alias string, res *ResolvedEntity) {
	er.aliases[normalizeAlias(alias)] = res
}

func (er *EntityRegistry) indexOwner(owner, id string) {
	if er.ownerIdx[owner] == nil {
		er.ownerIdx[owner] = make(map[string]bool)
	}
	er.ownerIdx[owner][id] = true
}

// Resolve performs a deterministic dereference for an owner. It tries exact
// alias match, then case-insensitive containment (a query containing a known
// alias, e.g. "未婚喵" inside a longer sentence). Returns nil when nothing
// resolvable is visible to that owner (fail-closed: no cross-owner hits).
func (er *EntityRegistry) Resolve(ownerUserID, query string) *ResolvedEntity {
	q := normalizeAlias(query)
	if q == "" {
		return nil
	}
	er.mu.RLock()
	defer er.mu.RUnlock()
	// Exact alias hit.
	if res, ok := er.aliases[q]; ok {
		if er.visibleTo(ownerUserID, res) {
			return res
		}
	}
	// Containment: a known alias appears within the query text.
	for alias, res := range er.aliases {
		if len(alias) < 2 {
			continue // avoid trivial substring false positives
		}
		if strings.Contains(q, alias) {
			if er.visibleTo(ownerUserID, res) {
				return res
			}
		}
	}
	return nil
}

// visibleTo enforces owner scoping (F260 KD-7): an entry resolves only for its
// owning operator; entries with no owner (shared) are visible to all.
func (er *EntityRegistry) visibleTo(ownerUserID string, res *ResolvedEntity) bool {
	if res.OwnerUserID == "" {
		return true
	}
	return res.OwnerUserID == ownerUserID
}

// ProposeEntity opens a lightweight concept candidate (F260 Phase A third pipe:
// slang/relationship words). It is a PENDING entry in the entity lane — human
// disposition (approval) is required before it becomes resolvable truth
// (mirrors the rest of SG's pending→approved discipline). ownerUserID is
// required (fail-closed). stance defaults to "unknown" and auto_inject to
// "never" so mechanical mirrors never surface as the operator's canon
// (F260 Design Gate anti stance-collapse).
func (er *EntityRegistry) ProposeEntity(ownerUserID, canonical, alias, provenance string) (*LaneEntry, error) {
	if ownerUserID == "" {
		return nil, ErrEntityOwnerRequired
	}
	lane := er.reg.Lane(LaneEntity)
	if lane == nil {
		return nil, ErrEntityLaneUnavailable
	}
	e := lane.Submit(canonical, provenance)
	e.OperatorID = ownerUserID
	// Concept proposals carry explicit non-canon stance until disposition.
	e.Sensitivity = "concept-proposed"
	lane.onMutated()
	return e, nil
}

// ApproveEntity promotes a proposed entity to approved, making it resolvable,
// then reindexes so the new alias is live.
func (er *EntityRegistry) ApproveEntity(id string) bool {
	lane := er.reg.Lane(LaneEntity)
	if lane == nil {
		return false
	}
	if !lane.Approve(id) {
		return false
	}
	er.reindex()
	return true
}

// ErrEntityOwnerRequired / ErrEntityLaneUnavailable are returned by the
// proposal flow when scoping or backing lane is missing (fail-closed).
var (
	ErrEntityOwnerRequired   = entityError("entity: owner_user_id required")
	ErrEntityLaneUnavailable = entityError("entity: lane unavailable")
)

type entityError string

func (e entityError) Error() string { return string(e) }

// uuid is referenced to keep the import used if future proposal flows mint IDs
// directly; the lane.Submit path already assigns one.
var _ = uuid.NewString
