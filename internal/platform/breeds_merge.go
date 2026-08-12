package platform

import (
	"log"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

// MergedBreeds returns the effective breed registry: template seeds (read-only
// fallback) merged with the runtime catalog, where the catalog always wins by
// ID.
//
// Per plan 1.2 / decision D2 the catalog (.sounds-great-ai/dog-catalog.json)
// is the single runtime truth; the pack template (dog-template.json) is only a
// seed used on first init (and as a fallback when the catalog is entirely
// empty/missing).
//
// On first init — when the catalog has no breeds yet — the template breeds are
// copied into the catalog so that subsequent deletions of a seed persist across
// restart (no template resurrection).
func MergedBreeds(templateBreeds map[string]*pack.BreedConfig, store settings.SettingsStore) (map[string]*pack.BreedConfig, error) {
	if err := seedCatalogIfEmpty(templateBreeds, store); err != nil {
		return nil, err
	}
	catalog, err := store.ListBreeds()
	if err != nil {
		return nil, err
	}
	merged := make(map[string]*pack.BreedConfig, len(templateBreeds)+len(catalog))
	for id, b := range templateBreeds {
		merged[id] = b
	}
	for _, b := range catalog {
		merged[b.ID] = b // catalog wins by ID
	}
	return merged, nil
}

// seedCatalogIfEmpty copies the template breeds into the catalog when the
// catalog currently has no breeds. It is idempotent: if the catalog already
// has breeds (migrated, previously seeded, or runtime-created) it is left
// untouched.
func seedCatalogIfEmpty(templateBreeds map[string]*pack.BreedConfig, store settings.SettingsStore) error {
	existing, err := store.ListBreeds()
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	for _, b := range templateBreeds {
		if b == nil || b.ID == "" {
			continue
		}
		// Copy so the template slice is never mutated; keep source as-is
		// (template seeds are editable per D2 — they are not marked "system").
		b2 := *b
		if err := store.CreateBreed(&b2); err != nil {
			return err
		}
	}
	log.Printf("Seeded runtime catalog with %d template breeds", len(templateBreeds))
	return nil
}
