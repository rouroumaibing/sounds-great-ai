package dossier

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func writeDossier(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "team"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, DossierRelativePath), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoaderCachingAndInvalidate(t *testing.T) {
	dir := t.TempDir()
	l := NewLoader()

	if l.IsAvailable(dir) {
		t.Error("missing dossier must report unavailable (community fallback)")
	}
	if l.HasEntry(dir, "bianmu") {
		t.Error("missing dossier must have no entries")
	}

	writeDossier(t, dir, "```yaml\n# structured-profile: dog:bianmu\nentityId: \"bianmu\"\nl0RosterSummary: \"v1\"\n```")

	if !l.IsAvailable(dir) {
		t.Error("dossier present must report available")
	}
	if s, ok := l.GetRosterSummary(dir, "bianmu"); !ok || s != "v1" {
		t.Errorf("GetRosterSummary = %q, %v", s, ok)
	}
	if _, ok := l.GetRoutingNote(dir, "bianmu"); ok {
		t.Error("no routing note in fixture")
	}

	// Rewrite the file; the cached value must stay until Invalidate.
	writeDossier(t, dir, "```yaml\n# structured-profile: dog:bianmu\nentityId: \"bianmu\"\nl0RosterSummary: \"v2\"\n```")
	if s, _ := l.GetRosterSummary(dir, "bianmu"); s != "v1" {
		t.Errorf("cache must hold v1 before invalidate, got %q", s)
	}

	l.Invalidate(dir)
	if s, ok := l.GetRosterSummary(dir, "bianmu"); !ok || s != "v2" {
		t.Errorf("after invalidate GetRosterSummary = %q, %v", s, ok)
	}
}

func TestReaderProjection(t *testing.T) {
	dir := t.TempDir()
	writeDossier(t, dir, "```yaml\n# structured-profile: dog:jinmao\nentityId: \"jinmao\"\noneLiner: \"知识寻回\"\nl0RosterSummary: \"检索\"\nl0RoutingNote: \"只检索不推理\"\n```")

	r := NewReader(NewLoader(), dir)
	if v, ok := r.OneLiner("jinmao"); !ok || v != "知识寻回" {
		t.Errorf("OneLiner = %q, %v", v, ok)
	}
	if v, ok := r.RoutingNote("jinmao"); !ok || v != "只检索不推理" {
		t.Errorf("RoutingNote = %q, %v", v, ok)
	}
	if _, ok := r.OneLiner("unknown"); ok {
		t.Error("unknown dog must miss")
	}
}

func TestLoaderConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	writeDossier(t, dir, "```yaml\n# structured-profile: dog:x\nentityId: \"x\"\n```")
	l := NewLoader()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Load(dir)
			l.HasEntry(dir, "x")
			l.Invalidate(dir)
		}()
	}
	wg.Wait()
}
