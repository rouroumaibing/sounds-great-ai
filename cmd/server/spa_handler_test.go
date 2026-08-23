package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// mapFS builds an in-memory dist tree: fstest.MapFile values implement
// io.ReadSeeker, so http.ServeContent works against them like real files.
func mapFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for name, content := range files {
		m[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return m
}

func doGet(h http.Handler, target, secFetchDest, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if secFetchDest != "" {
		req.Header.Set("Sec-Fetch-Dest", secFetchDest)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSPAHandler_NavigationFallsBackToIndex(t *testing.T) {
	primary, secondary := rankSpaRoots(
		spaFS{fsys: mapFS(map[string]string{"index.html": "<html>disk</html>"}), id: 10}, spaFS{})
	h := spaHandlerFromRoots(primary, secondary, spaFS{})

	rec := doGet(h, "/members", "document", "text/html,application/xhtml+xml")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "disk") {
		t.Fatalf("body %q, want disk index.html", body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("index Cache-Control = %q, want no-cache", cc)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("index Content-Type = %q, want text/html", ct)
	}
}

func TestSPAHandler_MissingAssetIs404NotHTML(t *testing.T) {
	// The pre-fix behavior served index.html here, which browsers reject as a
	// module script (strict MIME checking) — the "stale tab after redeploy"
	// failure mode.
	primary, secondary := rankSpaRoots(
		spaFS{fsys: mapFS(map[string]string{"index.html": "<html></html>"}), id: 10}, spaFS{})
	h := spaHandlerFromRoots(primary, secondary, spaFS{})

	rec := doGet(h, "/assets/MemberManagement-stale.js", "script", "*/*")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); strings.Contains(ct, "text/html") {
		t.Fatalf("missing asset Content-Type = %q, must not be text/html", ct)
	}
}

func TestSPAHandler_AssetsAreImmutable(t *testing.T) {
	primary, secondary := rankSpaRoots(spaFS{fsys: mapFS(map[string]string{
		"index.html":           "<html></html>",
		"assets/app-abc123.js": "// app",
		"assets/fonts-x.woff2": "font",
	}), id: 10}, spaFS{})
	h := spaHandlerFromRoots(primary, secondary, spaFS{})

	rec := doGet(h, "/assets/app-abc123.js", "script", "*/*")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "// app" {
		t.Fatalf("body = %q", body)
	}
	wantCC := "public, max-age=31536000, immutable"
	if cc := rec.Header().Get("Cache-Control"); cc != wantCC {
		t.Fatalf("asset Cache-Control = %q, want %q", cc, wantCC)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("asset Content-Type = %q, want javascript MIME", ct)
	}
}

func TestSPAHandler_SWAndManifestAreNoCache(t *testing.T) {
	primary, secondary := rankSpaRoots(spaFS{fsys: mapFS(map[string]string{
		"index.html":           "<html></html>",
		"sw.js":                "// sw",
		"registerSW.js":        "// reg",
		"manifest.webmanifest": "{}",
	}), id: 10}, spaFS{})
	h := spaHandlerFromRoots(primary, secondary, spaFS{})

	for _, name := range []string{"sw.js", "registerSW.js", "manifest.webmanifest"} {
		rec := doGet(h, "/"+name, "script", "*/*")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", name, rec.Code)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Fatalf("%s Cache-Control = %q, want no-cache", name, cc)
		}
	}
}

func TestSPAHandler_GraceServesPreviousBuildAssets(t *testing.T) {
	primary, secondary := rankSpaRoots(
		spaFS{fsys: mapFS(map[string]string{"index.html": "<html>new</html>"}), id: 20}, spaFS{})
	h := spaHandlerFromRoots(primary, secondary,
		spaFS{fsys: mapFS(map[string]string{"assets/old-chunk.js": "// old"}), id: 10})

	// A tab from before the redeploy still references the old chunk hash.
	rec := doGet(h, "/assets/old-chunk.js", "script", "*/*")
	if rec.Code != http.StatusOK || rec.Body.String() != "// old" {
		t.Fatalf("grace miss: status=%d body=%q", rec.Code, rec.Body.String())
	}
	// The new index backs the fallback for navigations.
	rec = doGet(h, "/", "document", "text/html")
	if !strings.Contains(rec.Body.String(), "new") {
		t.Fatalf("index body = %q, want new build", rec.Body.String())
	}
}

func TestRankSpaRoots_EmbeddedNewerThanDiskWins(t *testing.T) {
	// Release upgrade: binary embeds build 30, stale dist 20 sits on disk.
	primary, secondary := rankSpaRoots(
		spaFS{fsys: mapFS(map[string]string{"index.html": "disk", "assets/only-on-disk.js": "x"}), id: 20},
		spaFS{fsys: mapFS(map[string]string{"index.html": "embedded"}), id: 30})
	if primary.id != 30 || secondary.id != 20 {
		t.Fatalf("ranked (%d,%d), want embedded 30 first, disk 20 second", primary.id, secondary.id)
	}

	// Local iteration: dist 40 is newer than the compile-time snapshot 30.
	primary, secondary = rankSpaRoots(
		spaFS{fsys: mapFS(map[string]string{"index.html": "disk"}), id: 40},
		spaFS{fsys: mapFS(map[string]string{"index.html": "embedded"}), id: 30})
	if primary.id != 40 {
		t.Fatalf("primary id = %d, want disk 40", primary.id)
	}

	// Disk missing entirely: embedded serves alone.
	primary, _ = rankSpaRoots(spaFS{}, spaFS{fsys: mapFS(map[string]string{"index.html": "embedded"}), id: 30})
	if primary.fsys == nil {
		t.Fatal("embedded must win when disk dist is unavailable")
	}
}

func TestSPAHandler_DotfilesNeverServed(t *testing.T) {
	primary, secondary := rankSpaRoots(spaFS{fsys: mapFS(map[string]string{
		"index.html": "<html></html>",
		".build-id":  "12345",
	}), id: 10}, spaFS{})
	h := spaHandlerFromRoots(primary, secondary, spaFS{})

	if rec := doGet(h, "/.build-id", "empty", "*/*"); rec.Code != http.StatusNotFound {
		t.Fatalf("dotfile status = %d, want 404", rec.Code)
	}
}

func TestSPAHandler_APINotHandled(t *testing.T) {
	primary, secondary := rankSpaRoots(
		spaFS{fsys: mapFS(map[string]string{"index.html": "<html></html>"}), id: 10}, spaFS{})
	h := spaHandlerFromRoots(primary, secondary, spaFS{})

	if rec := doGet(h, "/api/nope", "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("api status = %d, want 404", rec.Code)
	}
}

func TestDiskBuildID(t *testing.T) {
	dir := t.TempDir()
	if id := diskBuildID(dir); id != 0 {
		t.Fatalf("empty dir id = %d, want 0", id)
	}
	if err := os.WriteFile(filepath.Join(dir, ".build-id"), []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if id := diskBuildID(dir); id != 42 {
		t.Fatalf(".build-id id = %d, want 42", id)
	}
}

func TestDiskSpaFSRequiresIndex(t *testing.T) {
	dir := t.TempDir()
	if s := diskSpaFS(dir); s.fsys != nil {
		t.Fatal("fs without index.html must be unavailable")
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := diskSpaFS(dir); s.fsys == nil || s.id == 0 {
		t.Fatal("dist with index.html must be available with a build id")
	}
}
