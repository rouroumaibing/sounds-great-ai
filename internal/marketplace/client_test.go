package marketplace

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func genKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func signedItem(t *testing.T, pub ed25519.PublicKey, priv ed25519.PrivateKey, data []byte) Item {
	t.Helper()
	sig := ed25519.Sign(priv, data)
	sum := sha256.Sum256(data)
	return Item{
		ID:        "acme",
		Name:      "Acme Plugin",
		Version:   "1.0.0",
		Tarball:   "https://example.com/acme.zip",
		Signature: base64.StdEncoding.EncodeToString(sig),
		Digest:    hex.EncodeToString(sum[:]),
	}
}

func TestVerifyAcceptsSignedPackage(t *testing.T) {
	pub, priv := genKey(t)
	data := []byte("fake zip bytes")
	it := signedItem(t, pub, priv, data)
	if err := Verify(data, it, []ed25519.PublicKey{pub}); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerifyRejectsTamperedBytesWrongKeyAndDigest(t *testing.T) {
	pub, priv := genKey(t)
	data := []byte("original archive")
	it := signedItem(t, pub, priv, data)

	// Tampered bytes: digest AND signature both fail.
	if err := Verify([]byte("tampered archive"), it, []ed25519.PublicKey{pub}); err == nil {
		t.Error("tampered bytes must be rejected")
	}

	// Wrong trusted key: signature fails even with matching digest.
	otherPub, _ := genKey(t)
	if err := Verify(data, it, []ed25519.PublicKey{otherPub}); err == nil {
		t.Error("signature from untrusted key must be rejected")
	}

	// Digest mismatch alone (signature valid over different bytes).
	bad := it
	bad.Digest = strings.Repeat("0", 64)
	if err := Verify(data, bad, []ed25519.PublicKey{pub}); err == nil {
		t.Error("digest mismatch must be rejected")
	}
}

func TestVerifyFailsClosedWithoutKeys(t *testing.T) {
	pub, priv := genKey(t)
	data := []byte("x")
	it := signedItem(t, pub, priv, data)
	if err := Verify(data, it, nil); err != ErrNoTrustedKeys {
		t.Errorf("no keys: got %v, want ErrNoTrustedKeys", err)
	}
}

func TestTrustedKeysEnvParsing(t *testing.T) {
	t.Setenv("SG_MARKETPLACE_PUBKEYS", "")
	if got := TrustedKeys(); len(got) != 0 {
		t.Errorf("empty env: %d keys", len(got))
	}
	pub, _ := genKey(t)
	enc := base64.StdEncoding.EncodeToString(pub)
	t.Setenv("SG_MARKETPLACE_PUBKEYS", "not-base64!!!, "+enc+" ,, "+enc)
	keys := TrustedKeys()
	if len(keys) != 2 {
		t.Fatalf("keys = %d, want 2 (dupes kept, invalid skipped)", len(keys))
	}
}

func TestClientListCacheQueryAndFind(t *testing.T) {
	var hits atomic.Int64
	idx := Index{Plugins: []Item{
		{ID: "alpha", Name: "Alpha", Version: "1.0.0", Publisher: "acme"},
		{ID: "beta", Name: "Beta Tool", Version: "2.0.0", Description: "search helpers"},
		{ID: "beta", Version: "2.1.0"},
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(idx)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	all, _, err := c.List("")
	if err != nil || len(all) != 3 {
		t.Fatalf("list = %v err=%v", len(all), err)
	}
	// second list within TTL hits cache: index fetched once
	if _, _, err := c.List(""); err != nil {
		t.Fatal(err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("index fetched %d times, want 1 (TTL cache)", got)
	}

	// query filter hits name + description + publisher
	q, _, _ := c.List("beta tool")
	if len(q) != 1 || q[0].ID != "beta" {
		t.Errorf("query 'beta tool' = %+v", q)
	}
	q, _, _ = c.List("acme")
	if len(q) != 1 || q[0].ID != "alpha" {
		t.Errorf("query 'acme' = %+v", q)
	}

	// Find picks the highest version
	it, ok, _ := c.Find("beta")
	if !ok || it.Version != "2.1.0" {
		t.Errorf("find beta = %+v ok=%v", it, ok)
	}
	if _, ok, _ := c.Find("ghost"); ok {
		t.Error("find ghost should miss")
	}
}

func TestClientOfflineServesStaleCacheThenErrors(t *testing.T) {
	var fail atomic.Bool
	idx := Index{Plugins: []Item{{ID: "alpha", Version: "1.0.0"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(idx)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, _, err := c.List(""); err != nil {
		t.Fatal(err)
	}

	// Force expiry + failure: stale cache still serves, with a note.
	c.mu.Lock()
	c.fetchedAt = c.fetchedAt.Add(-10 * 60 * 60 * 1e9) // 10h ago
	c.mu.Unlock()
	fail.Store(true)
	items, note, err := c.List("")
	if err != nil {
		t.Fatalf("stale cache not served: %v", err)
	}
	if len(items) != 1 || note == "" {
		t.Errorf("stale = %+v note=%q", items, note)
	}
}

func TestDisabledClient(t *testing.T) {
	c := NewClient("")
	if _, _, err := c.List(""); err != ErrDisabled {
		t.Errorf("disabled client: %v", err)
	}
}
