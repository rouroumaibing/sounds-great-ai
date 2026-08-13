package platform

import (
	"path/filepath"
	"testing"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

// TestSyncTemplateBreedsSeedsEmpty verifies that, into an empty catalog,
// template breeds (which carry no `enabled` field, so the zero value is false)
// are seeded as ENABLED with an available roster entry. Regression guard for
// the bug where the whole dog team surfaced as "已停用".
func TestSyncTemplateBreedsSeedsEmpty(t *testing.T) {
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
	var seeded *pack.BreedConfig
	for _, b := range breeds {
		if b.ID == "testdog" {
			seeded = b
		}
	}
	if seeded == nil {
		t.Fatalf("seeded breed not found in catalog")
	}
	if !seeded.Enabled {
		t.Errorf("seeded breed Enabled = false, want true")
	}

	roster, err := store.GetRoster()
	if err != nil {
		t.Fatalf("GetRoster error: %v", err)
	}
	entry, ok := roster["testdog"]
	if !ok {
		t.Fatalf("roster entry for testdog missing")
	}
	if !entry.Available {
		t.Errorf("roster entry Available = false, want true")
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

// TestSyncTemplateBreedsSkipsDeleted verifies that a template breed the
// customer explicitly deleted is NOT resurrected on upgrade sync (decision D2).
func TestSyncTemplateBreedsSkipsDeleted(t *testing.T) {
	dir := t.TempDir()
	store := settings.NewFileSettingsStore(
		filepath.Join(dir, "accounts.json"),
		filepath.Join(dir, "dog-catalog.json"),
		false,
	)

	templateBreeds := map[string]*pack.BreedConfig{
		"keepme":   {ID: "keepme", Name: "Keep"},
		"dropme":   {ID: "dropme", Name: "Drop"},
		"newbreed": {ID: "newbreed", Name: "New"},
	}

	// Initial sync seeds all three.
	if err := SyncTemplateBreeds(templateBreeds, store); err != nil {
		t.Fatalf("initial sync error: %v", err)
	}

	// Customer deletes "dropme".
	if err := store.DeleteBreed("dropme"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	// Upgrade adds a new template breed "evennewer". "dropme" must stay gone.
	templateBreeds2 := map[string]*pack.BreedConfig{
		"keepme":    {ID: "keepme", Name: "Keep"},
		"dropme":    {ID: "dropme", Name: "Drop"},
		"newbreed":  {ID: "newbreed", Name: "New"},
		"evennewer": {ID: "evennewer", Name: "Even Newer"},
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

// TestMergedBreedsCatalogWins confirms the catalog breed wins over the template
// by ID when both define the same breed.
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

	templateBreeds := map[string]*pack.BreedConfig{
		"shared": {ID: "shared", Name: "Template Name", DisplayName: "Template Dog"},
	}
	merged, err := MergedBreeds(templateBreeds, store)
	if err != nil {
		t.Fatalf("MergedBreeds error: %v", err)
	}
	if merged["shared"].DisplayName != "Catalog Dog" {
		t.Errorf("catalog breed did not win: got %q", merged["shared"].DisplayName)
	}
}
