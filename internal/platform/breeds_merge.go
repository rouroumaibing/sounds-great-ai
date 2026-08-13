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
// seed (first init) and an additive source of new breeds on upgrade.
//
// On first init — when the catalog has no breeds yet — the template breeds are
// copied into the catalog so subsequent deletions of a seed persist across
// restart (no template resurrection). On every init we also additively sync any
// NEW template breeds into an existing customer catalog, skipping breeds the
// customer has explicitly deleted (tracked in the store's deleted_breeds set).
func MergedBreeds(templateBreeds map[string]*pack.BreedConfig, store settings.SettingsStore) (map[string]*pack.BreedConfig, error) {
	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
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

// SyncTemplateBreeds additively copies template breeds into the catalog:
//   - breeds already present (by ID) are left untouched
//   - breeds the customer deleted (recorded in the store's deleted_breeds set)
//     are skipped, so a removed template dog is never resurrected (decision D2)
//   - any other template breed not yet in the catalog is added (enabled=true)
//
// This makes new template dogs appear for existing customers after an upgrade,
// while honoring the "no resurrection of deleted seeds" rule. It is idempotent:
// re-running it only adds breeds still missing.
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
	deletedIDs := make(map[string]bool)
	if dl, err := store.ListDeletedBreeds(); err == nil {
		for _, id := range dl {
			if id != "" {
				deletedIDs[id] = true
			}
		}
	}

	added := 0
	for _, b := range templateBreeds {
		if b == nil || b.ID == "" {
			continue
		}
		if existingIDs[b.ID] || deletedIDs[b.ID] {
			continue
		}
		// Copy so the template slice is never mutated; mark enabled=true so
		// freshly synced seeds surface as active in the UI (the template
		// carries no `enabled` field, whose zero value would disable them).
		b2 := *b
		b2.Enabled = true
		if err := store.CreateBreed(&b2); err != nil {
			return err
		}
		added++
	}
	if added > 0 {
		log.Printf("Synced %d new template breeds into catalog", added)
	}
	return nil
}
