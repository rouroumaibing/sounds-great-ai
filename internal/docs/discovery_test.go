package docs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover_AndLint(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("good.md", "---\ntitle: Good Doc\nkind: guide\ntags: [a, b]\n---\nbody\n")
	write("bad.md", "no frontmatter at all\n")

	profiles, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
	var good DocProfile
	for _, p := range profiles {
		if p.Path == "good.md" {
			good = p
		}
	}
	if good.Title != "Good Doc" || good.Kind != "guide" {
		t.Fatalf("frontmatter not parsed: %+v", good)
	}
	if len(good.Tags) != 2 {
		t.Fatalf("tags not parsed: %+v", good.Tags)
	}

	bad := Lint(profiles)
	if len(bad) != 1 || bad[0] != "bad.md" {
		t.Fatalf("lint should flag bad.md, got %v", bad)
	}
}
