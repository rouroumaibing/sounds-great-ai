package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/internal/skills"
)

// TestSkillsListItemSlicesNeverNull guards the API contract for
// GET /api/skills: triggers and mountPoints must always serialize as arrays.
// nil slices marshal to null, and SkillsPanel calls .map/.includes on both —
// a freshly wiped workspace (no persisted intents, no triggers on a skill)
// used to crash the whole settings panel into the ErrorBoundary.
func TestSkillsListItemSlicesNeverNull(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No triggers in frontmatter on purpose: AllTriggers returns nil.
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// NewManager keeps everything intent-less (no persisted config), which is
	// exactly the state of a workspace right after `make clean deep`.
	mgr := skills.NewManager(dir)
	if err := mgr.Scan(); err != nil {
		t.Fatal(err)
	}
	h := NewSkillsHandler(mgr, "", "")
	rec := httptest.NewRecorder()
	h.list(rec, httptest.NewRequest(http.MethodGet, "/api/skills", nil))

	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	for _, field := range []string{"triggers", "mountPoints"} {
		v, ok := items[0][field]
		if !ok {
			t.Fatalf("%s missing from response", field)
		}
		arr, ok := v.([]any)
		if !ok {
			t.Fatalf("%s = %#v, want an array (never null)", field, v)
		}
		if len(arr) != 0 {
			t.Fatalf("%s = %#v, want empty array", field, arr)
		}
	}
}
