package memory

import (
	"fmt"
	"strings"
	"sync"
)

// LibraryCatalog is the federation registry: it registers multiple Collections
// (each wrapping an existing LaneRegistry of typed lanes) together with their
// governance metadata — name, ACL reference, and permitted scanner level. It is
// the backbone for cross-Collection federated recall (P0-4) and the foundation
// for the higher-order write-side / recall stages (P1-C).
//
// It reuses the existing memory types rather than redefining a collection
// abstraction: a Collection IS a LaneRegistry plus metadata, and federated
// results are plain LaneEntry values tagged with their source collection.
type LibraryCatalog struct {
	mu          sync.RWMutex
	collections map[string]*Collection
}

// NewLibraryCatalog creates an empty federation registry.
func NewLibraryCatalog() *LibraryCatalog {
	return &LibraryCatalog{collections: make(map[string]*Collection)}
}

// Collection is a single federated namespace. It wraps a LaneRegistry (the
// collection's typed lanes) and governance metadata:
//   - ID:         stable collection/namespace id (alignment with LaneEntry.CollectionID)
//   - Name:       human-readable label
//   - ACLRef:     reference to an ACL policy (sensitivity floor / grant set id).
//                 It is a string key consumed by the ACL layer (see lane_acl.go);
//                 the catalog does not store ACL rules itself — it only points at them.
//   - ScanLevel:  the deepest scan tier this collection permits by default.
//   - Registry:   the reused LaneRegistry holding this collection's lanes.
type Collection struct {
	ID         string        // collection/namespace id
	Name       string        // human-readable name
	ACLRef     string        // ACL policy reference (sensitivity floor / grant set)
	ScanLevel  ScanLevel     // default permitted scan depth for this collection
	Registry   *LaneRegistry // reused: the collection's typed lanes
}

// Register adds (or replaces) a Collection in the federation. It returns an
// error for an empty ID or a nil Registry, preserving a usable invariant that
// every registered collection is fully wired.
func (c *LibraryCatalog) Register(col *Collection) error {
	if col == nil || col.ID == "" {
		return errCatalog("register: collection id required")
	}
	if col.Registry == nil {
		return errCatalog("register: collection %q has nil registry", col.ID)
	}
	c.mu.Lock()
	c.collections[col.ID] = col
	c.mu.Unlock()
	return nil
}

// Deregister removes a Collection from the federation (no error if absent).
func (c *LibraryCatalog) Deregister(id string) {
	c.mu.Lock()
	delete(c.collections, id)
	c.mu.Unlock()
}

// List returns all registered collections (unsorted snapshot).
func (c *LibraryCatalog) List() []*Collection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*Collection, 0, len(c.collections))
	for _, col := range c.collections {
		out = append(out, col)
	}
	return out
}

// Get returns a registered collection by ID. The second value is false if the
// ID is not registered.
func (c *LibraryCatalog) Get(id string) (*Collection, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	col, ok := c.collections[id]
	return col, ok
}

// FederatedResult is a single hit from a federated search, carrying the
// originating LaneEntry plus the source Collection it came from (so callers can
// attribute / re-scope results across the federation).
type FederatedResult struct {
	Entry      *LaneEntry // the matched lane entry (reused type)
	SourceID   string     // originating collection id
	SourceName string     // originating collection name
}

// FederatedSearch performs a cross-Collection federated recall. It queries each
// named collection (an unknown id is a hard error so callers cannot silently
// miss a shard), matches the query against entry content/type with a
// case-insensitive substring, and merges the hits into a single result list.
// Each result keeps its source Collection marker (SourceID/SourceName).
//
// Visibility (ACL) is applied per entry via EntryVisible when operator is
// non-empty, so a named operator only sees entries cleared by the collection's
// ACL + sensitivity model. "" operator = system scope (sees all).
func (c *LibraryCatalog) FederatedSearch(query string, collections []string, operator string) ([]*FederatedResult, error) {
	if len(collections) == 0 {
		return nil, errCatalog("federated search: no collections specified")
	}
	c.mu.RLock()
	picked := make([]*Collection, 0, len(collections))
	for _, id := range collections {
		col, ok := c.collections[id]
		if !ok {
			c.mu.RUnlock()
			return nil, errCatalog("federated search: unknown collection %q", id)
		}
		picked = append(picked, col)
	}
	c.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	var merged []*FederatedResult
	for _, col := range picked {
		for _, t := range col.Registry.LaneTypes() {
			lane := col.Registry.Lane(t)
			if lane == nil {
				continue
			}
			for _, e := range lane.All() {
				if !visibleForOperator(e, operator) {
					continue
				}
				if q == "" || matchEntry(e, q) {
					merged = append(merged, &FederatedResult{
						Entry:      e,
						SourceID:   col.ID,
						SourceName: col.Name,
					})
				}
			}
		}
	}
	return merged, nil
}

// visibleForOperator applies the ACL layer; "" operator is the system scope and
// bypasses per-operator restriction (EntryVisible already grants it everything).
func visibleForOperator(e *LaneEntry, operator string) bool {
	return EntryVisible(e, operator)
}

// matchEntry reports whether a lower-cased query appears in the entry content
// or its lane type. Empty query matches everything (caller already handled).
func matchEntry(e *LaneEntry, q string) bool {
	return strings.Contains(strings.ToLower(e.Content), q) ||
		strings.Contains(strings.ToLower(string(e.Type)), q)
}

// catalogError is the typed error returned by LibraryCatalog operations.
type catalogError string

func (e catalogError) Error() string { return string(e) }

func errCatalog(format string, args ...interface{}) error {
	if len(args) == 0 {
		return catalogError(format)
	}
	return catalogError(fmt.Sprintf(format, args...))
}
