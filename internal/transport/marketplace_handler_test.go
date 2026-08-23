package transport

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"sounds-great-ai/internal/marketplace"
	"sounds-great-ai/internal/plugins"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/skills"
)

// marketplaceFixture stands up an index host + package host with one signed
// plugin, and returns a PluginsHandler wired to it plus the trusted key
// (installed into SG_MARKETPLACE_PUBKEYS via t.Setenv).
func marketplaceFixture(t *testing.T) (*http.ServeMux, *atomicHolder) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SG_MARKETPLACE_PUBKEYS", base64.StdEncoding.EncodeToString(pub))

	// Build a real plugin zip.
	var pkg bytes.Buffer
	zw := zip.NewWriter(&pkg)
	w, _ := zw.Create("plugin.json")
	if _, err := w.Write([]byte(`{"id":"acme","name":"Acme","version":"1.0.0"}`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	pkgBytes := pkg.Bytes()

	// Package host serves the archive; the holder lets tests flip bytes.
	holder := &atomicHolder{data: pkgBytes}
	pkgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if holder.tampered {
			w.Write(append(holder.data, 'x'))
			return
		}
		w.Write(holder.data)
	}))
	t.Cleanup(pkgSrv.Close)

	sig := ed25519.Sign(priv, pkgBytes)
	sum := sha256.Sum256(pkgBytes)
	idxSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(marketplace.Index{Plugins: []marketplace.Item{{
			ID:        "acme",
			Name:      "Acme Plugin",
			Version:   "1.0.0",
			Publisher: "sounds-great-ai",
			Tarball:   pkgSrv.URL + "/acme.zip",
			Signature: base64.StdEncoding.EncodeToString(sig),
			Digest:    hex.EncodeToString(sum[:]),
		}}})
	}))
	t.Cleanup(idxSrv.Close)

	tmp := t.TempDir()
	svc := plugins.NewService(tmp)
	sk := skills.NewManagerWithConfig(filepath.Join(tmp, "skills-config.json"), "", map[string]string{})
	store := settings.NewFileSettingsStore(filepath.Join(tmp, "accounts.json"), filepath.Join(tmp, "catalog.json"), false)
	h := NewPluginsHandler(svc, sk, store, marketplace.NewClient(idxSrv.URL))
	return h.Routes(), holder
}

type atomicHolder struct {
	data     []byte
	tampered bool
}

func mktJSONReq(t *testing.T, mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		rdr = bytes.NewReader(data)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// The happy path: browse the index, install the signed package (lands
// disabled like every P3 install), index then marks it installed.
func TestMarketplaceBrowseAndVerifiedInstall(t *testing.T) {
	mux, _ := marketplaceFixture(t)

	rec := mktJSONReq(t, mux, "GET", "/api/marketplace?query=", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("browse status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listing struct {
		Plugins []struct {
			ID        string `json:"id"`
			Installed bool   `json:"installed"`
		} `json:"plugins"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listing)
	if len(listing.Plugins) != 1 || listing.Plugins[0].ID != "acme" || listing.Plugins[0].Installed {
		t.Fatalf("listing = %+v", listing)
	}

	rec = mktJSONReq(t, mux, "POST", "/api/marketplace/install", map[string]string{"id": "acme"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("install status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Now shows as installed.
	rec = mktJSONReq(t, mux, "GET", "/api/marketplace?query=", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &listing)
	if !listing.Plugins[0].Installed {
		t.Error("installed flag not set after marketplace install")
	}

	// Unknown id → 404.
	rec = mktJSONReq(t, mux, "POST", "/api/marketplace/install", map[string]string{"id": "ghost"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown id = %d, want 404", rec.Code)
	}
}

// Tampered package bytes must be refused by the signature gate (403) and
// never reach the installer.
func TestMarketplaceTamperedPackageRejected(t *testing.T) {
	mux, holder := marketplaceFixture(t)
	holder.tampered = true

	rec := mktJSONReq(t, mux, "POST", "/api/marketplace/install", map[string]string{"id": "acme"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("tampered install = %d body=%s, want 403", rec.Code, rec.Body.String())
	}

	// Nothing installed: plugins list stays empty.
	rec = mktJSONReq(t, mux, "GET", "/api/plugins", nil)
	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("tampered package must not install, got %+v", list)
	}
}

// No trusted keys configured → install fails closed with a clear error,
// browsing still works.
func TestMarketplaceFailsClosedWithoutKeys(t *testing.T) {
	mux, _ := marketplaceFixture(t)
	t.Setenv("SG_MARKETPLACE_PUBKEYS", "")

	rec := mktJSONReq(t, mux, "POST", "/api/marketplace/install", map[string]string{"id": "acme"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("no-keys install = %d body=%s, want 403", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SG_MARKETPLACE_PUBKEYS") {
		t.Errorf("error must point at the env knob: %s", rec.Body.String())
	}
	rec = mktJSONReq(t, mux, "GET", "/api/marketplace", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("browse with no keys = %d, want 200", rec.Code)
	}
}

func TestMarketplaceDisabledIndex(t *testing.T) {
	tmp := t.TempDir()
	h := NewPluginsHandler(
		plugins.NewService(tmp), nil, nil, marketplace.NewClient(""),
	)
	mux := h.Routes()
	rec := mktJSONReq(t, mux, "GET", "/api/marketplace", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("disabled marketplace browse = %d, want 503", rec.Code)
	}
}
