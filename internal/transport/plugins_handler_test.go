package transport

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sounds-great-ai/internal/plugins"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/skills"
)

// pluginPkg builds a real zip archive for the multipart install flow.
func pluginPkg(t *testing.T, id string, withSkills bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	write("plugin.json", fmt.Sprintf(`{"id":%q,"name":"Test Plugin","version":"1.0.0"}`, id))
	if withSkills {
		write("skills/greet/SKILL.md", "---\nid: greet\nname: Greet\ndescription: says hi\n---\nbody")
	}
	write("breeds/pal.json", fmt.Sprintf(`{"id":%q,"display_name":"Pal","enabled":true,"mention_patterns":["@pal"]}`, id+"-pal"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func multipartInstall(t *testing.T, mux *http.ServeMux, pkg []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("package", "plugin.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(pkg); err != nil {
		t.Fatal(err)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/api/plugins/install", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func patchJSON(t *testing.T, mux *http.ServeMux, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, _ := json.Marshal(body)
	req := httptest.NewRequest("PATCH", path, bytes.NewReader(data))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func newPluginsTestHandler(t *testing.T) (*PluginsHandler, *skills.SkillManager, settings.SettingsStore, *http.ServeMux) {
	t.Helper()
	tmp := t.TempDir()
	svc := plugins.NewService(tmp)
	sk := skills.NewManagerWithConfig(
		filepath.Join(tmp, "skills-config.json"), "",
		map[string]string{filepath.Join(tmp, "builtin-skills"): "packs"},
	)
	store := settings.NewFileSettingsStore(
		filepath.Join(tmp, "accounts.json"),
		filepath.Join(tmp, "catalog.json"), false,
	)
	h := NewPluginsHandler(svc, sk, store, nil)
	return h, sk, store, h.Routes()
}

func skillSecurity(t *testing.T, sk *skills.SkillManager, id string) string {
	t.Helper()
	if st := sk.Security().StateOf(id); st != nil {
		return string(st.Status)
	}
	return "<none>"
}

// Full lifecycle over HTTP: install (disabled, skills pending) → enable is
// blocked while a skill is pending → approve via the security pipeline →
// enable registers the plugin breed → disable turns it off → uninstall
// removes the breed and payload.
func TestPluginsHandlerLifecycle(t *testing.T) {
	h, sk, store, mux := newPluginsTestHandler(t)

	// Install.
	rec := multipartInstall(t, mux, pluginPkg(t, "acme", true))
	if rec.Code != http.StatusCreated {
		t.Fatalf("install status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Plugin plugins.View `json:"plugin"`
		Notes  []string     `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Plugin.Enabled {
		t.Error("plugin must install disabled")
	}
	if len(created.Plugin.SkillIDs) != 1 || created.Plugin.SkillIDs[0] != "greet" {
		t.Errorf("skill ids = %v", created.Plugin.SkillIDs)
	}

	// The shipped skill must now be in the pending security review.
	if got := skillSecurity(t, sk, "greet"); got != "pending" {
		t.Fatalf("greet security = %s, want pending", got)
	}

	// Enable is refused while the skill is pending (fail closed, 409 + ids).
	rec = patchJSON(t, mux, "/api/plugins/acme", map[string]any{"enabled": true})
	if rec.Code != http.StatusConflict {
		t.Fatalf("enable-with-pending status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "greet") {
		t.Errorf("409 body must name the blocking skill: %s", rec.Body.String())
	}

	// Approve the skill the way the operator does (skills security API):
	// ApproveSkill stamps the current fingerprint so re-scan keeps it.
	if err := sk.ApproveSkill("greet", "operator"); err != nil {
		t.Fatal(err)
	}

	// Enable now succeeds and registers the plugin breed (source=plugin).
	rec = patchJSON(t, mux, "/api/plugins/acme", map[string]any{"enabled": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("enable status = %d body=%s", rec.Code, rec.Body.String())
	}
	breeds, err := store.ListBreeds()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range breeds {
		if b.ID == "acme-pal" {
			found = true
			if string(b.Source) != "plugin" || !b.Enabled {
				t.Errorf("plugin breed wrong: source=%s enabled=%v", b.Source, b.Enabled)
			}
		}
	}
	if !found {
		t.Fatalf("plugin breed acme-pal not registered: %+v", breeds)
	}

	// Disable flips the breed off but keeps the definition.
	rec = patchJSON(t, mux, "/api/plugins/acme", map[string]any{"enabled": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("disable status = %d", rec.Code)
	}
	breeds, _ = store.ListBreeds()
	stillThere, isEnabled := false, false
	for _, b := range breeds {
		if b.ID == "acme-pal" {
			stillThere = true
			isEnabled = b.Enabled
		}
	}
	if !stillThere {
		t.Error("disable must keep the breed definition")
	}
	if isEnabled {
		t.Error("disable must turn the breed off")
	}

	// Uninstall deletes the breed + payload + registry entry.
	req := httptest.NewRequest("DELETE", "/api/plugins/acme", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("uninstall status = %d", rec.Code)
	}
	breeds, _ = store.ListBreeds()
	for _, b := range breeds {
		if b.ID == "acme-pal" {
			t.Error("plugin breed must be deleted on uninstall")
		}
	}
	if got := h.svc.EnabledPlugins(); len(got) != 0 {
		t.Errorf("registry not cleaned: %v", got)
	}

	// GET collection is now empty.
	req = httptest.NewRequest("GET", "/api/plugins", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var list []plugins.View
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("list after uninstall = %+v", list)
	}
}

func TestPluginsHandlerValidation(t *testing.T) {
	_, _, _, mux := newPluginsTestHandler(t)

	// Non-multipart install → 400.
	req := httptest.NewRequest("POST", "/api/plugins/install", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("non-multipart install = %d, want 400", rec.Code)
	}

	// Missing manifest → 400.
	rec = multipartInstall(t, mux, pluginPkgNoManifest(t))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad package install = %d, want 400", rec.Code)
	}

	// PATCH without enabled → 400.
	rec = patchJSON(t, mux, "/api/plugins/ghost", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("patch without enabled = %d, want 400", rec.Code)
	}

	// PATCH unknown plugin → 404.
	rec = patchJSON(t, mux, "/api/plugins/ghost", map[string]any{"enabled": true})
	if rec.Code != http.StatusNotFound {
		t.Errorf("patch unknown = %d, want 404", rec.Code)
	}

	// Marketplace without an index URL: honest 503 disabled state.
	req = httptest.NewRequest("GET", "/api/marketplace", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "not configured") {
		t.Errorf("marketplace disabled = %d %s, want 503 + note", rec.Code, rec.Body.String())
	}
}

func pluginPkgNoManifest(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("readme.txt")
	if _, err := w.Write([]byte("no manifest here")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Regression: the file upload must respect the 64MiB budget (MaxBytesReader).
func TestPluginsHandlerInstallSizeCap(t *testing.T) {
	_, _, _, mux := newPluginsTestHandler(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("package", "big.zip")
	if _, err := io.CopyBuffer(fw, bytes.NewReader(make([]byte, 1<<20)), nil); err != nil {
		t.Fatal(err)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/api/plugins/install", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.ContentLength = int64(body.Len())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusCreated {
		t.Error("garbage 1MiB zip must not install")
	}
	_ = os.Getenv("TMPDIR")
}
