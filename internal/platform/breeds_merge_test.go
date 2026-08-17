package platform

import (
	"path/filepath"
	"testing"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

// TestSyncTemplateBreedsFirstRunEmptyCatalog verifies decision D1: on a fresh
// first run (no catalog file) the catalog stays empty — no dogs are auto-
// injected — and every template breed is recorded as seen so a later restart
// does not re-inject them.
func TestSyncTemplateBreedsFirstRunEmptyCatalog(t *testing.T) {
	dir := t.TempDir()
	store := settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)

	templateBreeds := map[string]*pack.BreedConfig{
		"testdog": {
			ID:          "testdog",
			Name:        "Test Dog",
			DisplayName: "Test Dog",
			Variants: []pack.Variant{
				{ID: "default", ClientID: "claude", DefaultModel: "claude-3-5-sonnet"},
			},
		},
	}

	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
		t.Fatalf("SyncTemplateBreeds returned error: %v", err)
	}

	breeds, err := store.ListBreeds()
	if err != nil {
		t.Fatalf("ListBreeds error: %v", err)
	}
	if len(breeds) != 0 {
		t.Fatalf("first run must produce an empty catalog, got %d breeds", len(breeds))
	}

	seen, err := store.ListSeenTemplateBreeds()
	if err != nil {
		t.Fatalf("ListSeenTemplateBreeds error: %v", err)
	}
	found := false
	for _, s := range seen {
		if s == "testdog" {
			found = true
		}
	}
	if !found {
		t.Errorf("template breed must be marked seen on first run (got %v)", seen)
	}
}

// TestSyncTemplateBreedsSecondRunStaysEmpty verifies that a restart after a
// first run (catalog empty, all templates seen) does NOT re-inject the dogs.
func TestSyncTemplateBreedsSecondRunStaysEmpty(t *testing.T) {
	dir := t.TempDir()
	store := settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)
	templateBreeds := map[string]*pack.BreedConfig{
		"testdog": {ID: "testdog", Name: "Test Dog", Variants: []pack.Variant{{ID: "default", ClientID: "claude"}}},
	}
	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
		t.Fatalf("first SyncTemplateBreeds: %v", err)
	}
	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
		t.Fatalf("second SyncTemplateBreeds: %v", err)
	}
	breeds, _ := store.ListBreeds()
	if len(breeds) != 0 {
		t.Errorf("second run must keep catalog empty, got %d breeds", len(breeds))
	}
}

// TestSyncTemplateBreedsUpgradeAddsNew verifies decision D3: a newly added
// template breed is auto-synced into an existing catalog and marked seen.
func TestSyncTemplateBreedsUpgradeAddsNew(t *testing.T) {
	dir := t.TempDir()
	store := settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)
	// Seed an existing catalog so this is treated as an upgrade, not first run.
	if err := store.CreateBreed(&pack.BreedConfig{
		ID: "existing", Name: "Existing", Variants: []pack.Variant{{ID: "default", ClientID: "claude"}},
	}); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	templateBreeds := map[string]*pack.BreedConfig{
		"existing": {ID: "existing", Name: "Existing"},
		"newdog":   {ID: "newdog", Name: "New Dog", Variants: []pack.Variant{{ID: "default", ClientID: "claude"}}},
	}
	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
		t.Fatalf("SyncTemplateBreeds: %v", err)
	}
	breeds, _ := store.ListBreeds()
	ids := map[string]bool{}
	for _, b := range breeds {
		ids[b.ID] = true
	}
	if !ids["existing"] {
		t.Error("existing breed dropped")
	}
	if !ids["newdog"] {
		t.Error("new template breed not auto-added on upgrade (D3)")
	}
	seen, _ := store.ListSeenTemplateBreeds()
	seenSet := map[string]bool{}
	for _, s := range seen {
		seenSet[s] = true
	}
	if !seenSet["newdog"] {
		t.Error("newly added breed must be marked seen")
	}
}

// TestSyncTemplateBreedsSkipsDeleted verifies decision D2: a template breed the
// customer explicitly deleted is never resurrected on upgrade sync.
func TestSyncTemplateBreedsSkipsDeleted(t *testing.T) {
	dir := t.TempDir()
	store := settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)

	templateBreeds := map[string]*pack.BreedConfig{
		"keepme":   {ID: "keepme", Name: "Keep", Variants: []pack.Variant{{ID: "default", ClientID: "claude"}}},
		"dropme":   {ID: "dropme", Name: "Drop", Variants: []pack.Variant{{ID: "default", ClientID: "claude"}}},
		"newbreed": {ID: "newbreed", Name: "New", Variants: []pack.Variant{{ID: "default", ClientID: "claude"}}},
	}

	// Seed an existing catalog (so this is an upgrade, not first run) with the
	// three breeds already present.
	for _, b := range templateBreeds {
		b2 := *b
		if err := store.CreateBreed(&b2); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Customer deletes "dropme".
	if err := store.DeleteBreed("dropme"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	// Upgrade adds "evennewer". "dropme" must stay gone.
	templateBreeds2 := map[string]*pack.BreedConfig{
		"keepme":    {ID: "keepme", Name: "Keep"},
		"dropme":    {ID: "dropme", Name: "Drop"},
		"newbreed":  {ID: "newbreed", Name: "New"},
		"evennewer": {ID: "evennewer", Name: "Even Newer", Variants: []pack.Variant{{ID: "default", ClientID: "claude"}}},
	}
	if err := SyncTemplateBreeds(templateBreeds2, store); err != nil {
		t.Fatalf("upgrade sync error: %v", err)
	}

	breeds, _ := store.ListBreeds()
	ids := make(map[string]bool)
	for _, b := range breeds {
		ids[b.ID] = true
	}
	if !ids["keepme"] {
		t.Errorf("keepme missing")
	}
	if !ids["newbreed"] {
		t.Errorf("newbreed missing")
	}
	if !ids["evennewer"] {
		t.Errorf("evennewer not added on upgrade (additive sync broken)")
	}
	if ids["dropme"] {
		t.Errorf("dropme was resurrected despite explicit deletion (D2 violated)")
	}

	deleted, err := store.ListDeletedBreeds()
	if err != nil {
		t.Fatalf("ListDeletedBreeds error: %v", err)
	}
	found := false
	for _, d := range deleted {
		if d == "dropme" {
			found = true
		}
	}
	if !found {
		t.Errorf("dropme not recorded in deleted_breeds")
	}
}

// TestSyncTemplateBreedsIdempotent ensures a second sync adds nothing (no
// duplicate breeds), while a pre-existing catalog breed is preserved.
func TestSyncTemplateBreedsIdempotent(t *testing.T) {
	dir := t.TempDir()
	store := settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)
	if err := store.CreateBreed(&pack.BreedConfig{
		ID:   "preexisting",
		Name: "Pre",
		Variants: []pack.Variant{
			{ID: "default", ClientID: "claude", DefaultModel: "x"},
		},
	}); err != nil {
		t.Fatalf("pre-create error: %v", err)
	}

	templateBreeds := map[string]*pack.BreedConfig{
		"template-only": {ID: "template-only", Name: "Tpl"},
	}

	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
		t.Fatalf("first SyncTemplateBreeds error: %v", err)
	}
	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
		t.Fatalf("second SyncTemplateBreeds error: %v", err)
	}

	breeds, _ := store.ListBreeds()
	ids := make(map[string]int)
	for _, b := range breeds {
		ids[b.ID]++
	}
	if ids["preexisting"] != 1 {
		t.Errorf("pre-existing breed not preserved (count=%d)", ids["preexisting"])
	}
	if ids["template-only"] != 1 {
		t.Errorf("template-only breed count = %d, want 1 (no duplication)", ids["template-only"])
	}
}

// TestMergedBreedsCatalogWins confirms the catalog breed wins over the template
// by ID when both define the same breed, and that a template-only breed which
// is already marked seen (but not added to the catalog) does NOT appear in the
// active registry.
func TestMergedBreedsCatalogWins(t *testing.T) {
	dir := t.TempDir()
	store := settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)
	if err := store.CreateBreed(&pack.BreedConfig{
		ID:          "shared",
		Name:        "Catalog Name",
		DisplayName: "Catalog Dog",
		Variants:    []pack.Variant{{ID: "default", ClientID: "claude"}},
	}); err != nil {
		t.Fatalf("create error: %v", err)
	}
	// Mark the template-only breed as seen (not added) so the upgrade sync does
	// not inject it; this isolates the "catalog wins" assertion.
	if err := store.AddSeenTemplateBreeds([]string{"shared", "template"}); err != nil {
		t.Fatalf("mark seen: %v", err)
	}

	templateBreeds := map[string]*pack.BreedConfig{
		"shared":   {ID: "shared", Name: "Template Name", DisplayName: "Template Dog"},
		"template": {ID: "template", Name: "Template Only"},
	}
	merged, err := MergedBreeds(templateBreeds, store)
	if err != nil {
		t.Fatalf("MergedBreeds error: %v", err)
	}
	if merged["shared"].DisplayName != "Catalog Dog" {
		t.Errorf("catalog breed did not win: got %q", merged["shared"].DisplayName)
	}
	// The template-only breed must NOT appear in the active registry.
	if _, ok := merged["template"]; ok {
		t.Errorf("template-only breed must not be in the active registry (D1/D3)")
	}
}

// TestMergedBreedsFirstRunEmpty verifies decision D1 at the MergeBreeds layer:
// with no catalog file, the active registry is empty even though template
// breeds exist — they are only a menu, not active dogs.
func TestMergedBreedsFirstRunEmpty(t *testing.T) {
	dir := t.TempDir()
	store := settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)
	templateBreeds := map[string]*pack.BreedConfig{
		"bianmu": {ID: "bianmu", Name: "边牧"},
		"xigou":  {ID: "xigou", Name: "细狗"},
	}
	merged, err := MergedBreeds(templateBreeds, store)
	if err != nil {
		t.Fatalf("MergedBreeds error: %v", err)
	}
	if len(merged) != 0 {
		t.Errorf("first run must yield an empty registry (D1), got %d breeds: %v", len(merged), merged)
	}
}

// TestMergedBreedsDeepMergeBackfill verifies the adopted clowder semantics
// (2026-08-17): per-field catalog edits win, and NEW template variants are
// backfilled into the runtime without clobbering existing catalog variants or
// edits. This is the improvement over the old "catalog wins entirely" rule.
func TestMergedBreedsDeepMergeBackfill(t *testing.T) {
	dir := t.TempDir()
	store := settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)
	if err := store.CreateBreed(&pack.BreedConfig{
		ID:       "shared",
		Name:     "Catalog Name",
		Enabled:  true,
		Variants: []pack.Variant{{ID: "v1", ClientID: "claude"}},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	templateBreeds := map[string]*pack.BreedConfig{
		"shared": {
			ID:   "shared",
			Name: "Template Name",
			Variants: []pack.Variant{
				{ID: "v1", ClientID: "codex"},  // differs from catalog v1
				{ID: "v2", ClientID: "gemini"}, // NEW in template
			},
		},
	}
	merged, err := MergedBreeds(templateBreeds, store)
	if err != nil {
		t.Fatalf("MergedBreeds: %v", err)
	}
	got, ok := merged["shared"]
	if !ok {
		t.Fatalf("shared missing from merged")
	}
	if got.Name != "Catalog Name" {
		t.Errorf("catalog name must win: got %q", got.Name)
	}
	if !got.Enabled {
		t.Errorf("catalog enabled must be preserved")
	}
	byID := map[string]pack.Variant{}
	for _, v := range got.Variants {
		byID[v.ID] = v
	}
	if len(byID) != 2 {
		t.Fatalf("expected 2 variants (catalog v1 + backfilled v2), got %d: %v", len(byID), got.Variants)
	}
	if byID["v1"].ClientID != "claude" {
		t.Errorf("catalog v1 edit must win (client=claude), got %q", byID["v1"].ClientID)
	}
	if byID["v2"].ClientID != "gemini" {
		t.Errorf("new template v2 must be backfilled (client=gemini), got %q", byID["v2"].ClientID)
	}
}
