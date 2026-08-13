package platform

import (
	"log"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

// MergedBreeds returns the effective breed registry. Per decision D1/D3 the
// runtime registry contains ONLY catalog breeds; the pack template
// (dog-template.json) is no longer merged into the active registry. The
// template remains available as an "add member" menu via GetTemplates, which
// reads dog-template.json directly.
//
// SyncTemplateBreeds (called here) keeps the catalog in sync with the template:
//   - first run (no catalog file): writes an EMPTY catalog + marks every
//     template breed as seen, so the user starts with no dogs (D1) and a later
//     restart does not re-inject them.
//   - existing catalog: any template breed not already in the catalog and not
//     yet seen is added (Enabled=true) and marked seen — this is the upgrade
//     auto-add of new template dogs (D3). Breeds already seen but not added are
//     never resurrected.
func MergedBreeds(templateBreeds map[string]*pack.BreedConfig, store settings.SettingsStore) (map[string]*pack.BreedConfig, error) {
	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
		return nil, err
	}
	catalog, err := store.ListBreeds()
	if err != nil {
		return nil, err
	}
	merged := make(map[string]*pack.BreedConfig, len(catalog))
	for _, b := range catalog {
		merged[b.ID] = b // only catalog breeds; the template is not active
	}
	return merged, nil
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
