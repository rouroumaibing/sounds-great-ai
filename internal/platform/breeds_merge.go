package platform

import (
	"encoding/json"
	"log"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

// MergedBreeds returns the effective breed registry. Per decision D1/D3 the
// runtime registry contains ONLY catalog breeds; the pack template
// (dog-template.json) is not auto-injected as active dogs. The template remains
// available as an "add member" menu via GetTemplates, which reads
// dog-template.json directly.
//
// Merge semantics follow clowder's cat-config-loader (deep merge + id-keyed
// backfill), adopted 2026-08-17 because it is strictly better than the previous
// "catalog wins entirely" rule:
//   - catalog edits win per-field (so a template upgrade never clobbers a user's
//     catalog edits),
//   - nested objects are recursively merged,
//   - the variants array is merged by id: catalog variants override template
//     variants, NEW template variants are backfilled, and catalog-only variants
//     are preserved.
//
// SyncTemplateBreeds (called here) keeps breed *membership* in sync with the
// template: first run writes an EMPTY catalog + marks every template breed seen
// (D1); an upgrade auto-adds genuinely-new template breeds (D3). Breed-level
// field/variant backfill is delivered by the deep merge below.
func MergedBreeds(templateBreeds map[string]*pack.BreedConfig, store settings.SettingsStore) (map[string]*pack.BreedConfig, error) {
	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
		return nil, err
	}
	catalog, err := store.ListBreeds()
	if err != nil {
		return nil, err
	}
	merged := make(map[string]*pack.BreedConfig, len(catalog))
	for _, c := range catalog {
		if c == nil {
			continue
		}
		if tmpl, ok := templateBreeds[c.ID]; ok {
			// clowder-style deep merge: template is the BASE, catalog is the
			// OVERLAY. Catalog edits win per-field; new template fields/variants
			// are backfilled without clobbering catalog edits.
			merged[c.ID] = deepMergeBreeds(tmpl, c)
		} else {
			merged[c.ID] = c
		}
	}
	return merged, nil
}

// deepMergeBreeds returns the effective breed after merging the template (base)
// with the catalog (overlay). See deepMergeConfig for the algorithm.
func deepMergeBreeds(base, overlay *pack.BreedConfig) *pack.BreedConfig {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}
	bJSON, err := json.Marshal(base)
	if err != nil {
		return overlay
	}
	oJSON, err := json.Marshal(overlay)
	if err != nil {
		return overlay
	}
	var bMap, oMap map[string]interface{}
	_ = json.Unmarshal(bJSON, &bMap)
	_ = json.Unmarshal(oJSON, &oMap)
	merged := deepMergeConfig(bMap, oMap)
	out, err := json.Marshal(merged)
	if err != nil {
		return overlay
	}
	var result pack.BreedConfig
	if err := json.Unmarshal(out, &result); err != nil {
		return overlay
	}
	return &result
}

// breedAtomicObjectKeys are object fields replaced wholesale by the catalog
// overlay rather than field-merged. Mirrors clowder's ATOMIC_OBJECT_KEYS to
// prevent stale sub-fields surviving a provider/model switch.
var breedAtomicObjectKeys = map[string]bool{
	"color":       true,
	"voice_config": true,
}

// deepMergeConfig is a Go port of clowder's deepMergeConfig (cat-config-loader):
// overlay fields override base; atomic object keys replace base entirely;
// id-keyed arrays are merged by id (base-only items preserved); other objects
// recurse; other arrays/primitives are replaced by overlay.
func deepMergeConfig(base, overlay map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(base)+len(overlay))
	for k, v := range base {
		merged[k] = v
	}
	for k, oVal := range overlay {
		bVal := merged[k]
		switch {
		case breedAtomicObjectKeys[k]:
			merged[k] = oVal
		case isIDArray(oVal) && isIDArray(bVal):
			merged[k] = mergeByID(bVal.([]interface{}), oVal.([]interface{}))
		case isObj(oVal) && isObj(bVal):
			merged[k] = deepMergeConfig(bVal.(map[string]interface{}), oVal.(map[string]interface{}))
		default:
			merged[k] = oVal
		}
	}
	return merged
}

func isObj(v interface{}) bool {
	m, ok := v.(map[string]interface{})
	return ok && m != nil
}

func isIDArray(v interface{}) bool {
	arr, ok := v.([]interface{})
	if !ok || len(arr) == 0 {
		return false
	}
	for _, it := range arr {
		m, ok := it.(map[string]interface{})
		if !ok || m == nil {
			return false
		}
		if _, has := m["id"]; !has {
			return false
		}
	}
	return true
}

// mergeByID merges two id-keyed arrays: overlay items override base items by
// id (recursively, so catalog edits survive a template upgrade), overlay-only
// items are appended, and base-only items are preserved (template backfill).
func mergeByID(base, overlay []interface{}) []interface{} {
	bm := make(map[string]interface{}, len(base))
	for _, it := range base {
		if m, ok := it.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok {
				bm[id] = it
			}
		}
	}
	seen := make(map[string]bool, len(overlay))
	result := make([]interface{}, 0, len(base)+len(overlay))
	for _, o := range overlay {
		om, ok := o.(map[string]interface{})
		if !ok {
			result = append(result, o)
			continue
		}
		id, _ := om["id"].(string)
		seen[id] = true
		if b, ok := bm[id]; ok {
			result = append(result, deepMergeConfig(b.(map[string]interface{}), om))
		} else {
			result = append(result, o)
		}
	}
	for _, b := range base {
		if m, ok := b.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok && !seen[id] {
				result = append(result, b)
			}
		}
	}
	return result
}

// SyncTemplateBreeds reconciles the runtime catalog with the template using the
// seen_template_breeds set so that:
//   - the first run produces an empty catalog (no auto-injected dogs), and
//   - an upgrade auto-adds genuinely new template dogs without ever resurrecting
//     a dog the customer deleted or chose not to add.
//
// It is idempotent: re-running it only adds template breeds still missing and
// not yet seen.
func SyncTemplateBreeds(templateBreeds map[string]*pack.BreedConfig, store settings.SettingsStore) error {
	existing, err := store.ListBreeds()
	if err != nil {
		return err
	}
	existingIDs := make(map[string]bool, len(existing))
	for _, b := range existing {
		if b != nil {
			existingIDs[b.ID] = true
		}
	}

	seen, err := store.ListSeenTemplateBreeds()
	if err != nil {
		return err
	}
	seenSet := make(map[string]bool, len(seen))
	for _, id := range seen {
		if id != "" {
			seenSet[id] = true
		}
	}

	// Migration for a pre-seen catalog (built before this mechanism existed):
	// seed the seen set from whatever is already in the catalog (breeds +
	// explicit deletions) so the first run of the new logic neither duplicates
	// existing dogs nor resurrects deleted ones. Idempotent across restarts.
	if len(seenSet) == 0 {
		for id := range existingIDs {
			seenSet[id] = true
		}
		if deleted, derr := store.ListDeletedBreeds(); derr == nil {
			for _, id := range deleted {
				if id != "" {
					seenSet[id] = true
				}
			}
		}
	}

	// First run: the catalog file does not exist yet. Write an empty catalog
	// and mark every template breed as seen, so the user starts with no dogs
	// (D1) and a later restart keeps the catalog empty.
	if !store.CatalogFileExists() {
		for id := range templateBreeds {
			seenSet[id] = true
		}
		return store.AddSeenTemplateBreeds(keys(seenSet))
	}

	// Existing catalog: add only genuinely-new template breeds (D3), marking
	// each seen so it is never re-added after a later deletion.
	added := 0
	for _, b := range templateBreeds {
		if b == nil || b.ID == "" {
			continue
		}
		if existingIDs[b.ID] || seenSet[b.ID] {
			continue
		}
		// Copy so the template slice is never mutated; mark enabled=true so a
		// freshly synced template dog surfaces as active in the UI (the
		// template carries no `enabled` field, whose zero value disables it).
		b2 := *b
		b2.Enabled = true
		if err := store.CreateBreed(&b2); err != nil {
			return err
		}
		if err := store.AddSeenTemplateBreeds([]string{b.ID}); err != nil {
			return err
		}
		added++
	}
	if added > 0 {
		log.Printf("Synced %d new template breeds into catalog", added)
	}
	return nil
}

// keys returns the keys of a string→bool set.
func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
