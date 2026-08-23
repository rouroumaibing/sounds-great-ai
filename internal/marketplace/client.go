// Package marketplace implements the plugin marketplace client
// (panels-roadmap P4): fetch a remote index (cached, TTL-bounded), and
// install an indexed plugin by downloading its archive and verifying the
// publisher's ed25519 signature before handing it to the plugins installer.
//
// Trust model: the index itself travels over HTTPS and may be cached, but it
// is NOT trusted for code — every tarball must carry a valid ed25519
// signature from a configured publisher key. A tampered index can therefore
// not smuggle in unsigned or wrongly-signed packages. With no trusted keys
// configured, installs fail closed (browsing still works).
package marketplace

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Item is one entry in the marketplace index.
type Item struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Publisher   string `json:"publisher,omitempty"`
	Tarball     string `json:"tarball"`   // https URL to the .zip archive
	Signature   string `json:"signature"` // base64 ed25519 over the archive bytes
	Digest      string `json:"digest"`    // hex sha256 of the archive bytes
	Installs    int64  `json:"installs,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
}

// Index is the wire shape of index.json at the index URL.
type Index struct {
	Plugins []Item `json:"plugins"`
}

// Client fetches and caches the index and verifies packages.
type Client struct {
	indexURL string
	http     *http.Client

	mu         sync.Mutex
	cached     *Index
	fetchedAt  time.Time
	fetchError string // last fetch failure, surfaced for honest offline state
}

// NewClient builds a client for the given index URL (empty = disabled).
func NewClient(indexURL string) *Client {
	return &Client{
		indexURL: strings.TrimSpace(indexURL),
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// IndexURL reports the configured index URL ("" = marketplace disabled).
func (c *Client) IndexURL() string { return c.indexURL }

// ErrDisabled is returned when no index URL is configured.
var ErrDisabled = fmt.Errorf("marketplace index not configured (SG_MARKETPLACE_INDEX)")

// List returns the (cached ≤5min) index, optionally filtered by a query
// substring over id/name/description/publisher. An unreachable index yields
// the stale cache if present, otherwise the fetch error.
func (c *Client) List(query string) ([]Item, string, error) {
	if c.indexURL == "" {
		return nil, "", ErrDisabled
	}
	c.mu.Lock()
	needFetch := c.cached == nil || time.Since(c.fetchedAt) > 5*time.Minute
	c.mu.Unlock()

	if needFetch {
		idx, err := c.fetch()
		c.mu.Lock()
		if err != nil {
			c.fetchError = err.Error()
			// keep stale cache on failure
			if c.cached != nil {
				err = nil
			}
		} else {
			c.cached = idx
			c.fetchError = ""
			c.fetchedAt = time.Now()
		}
		c.mu.Unlock()
		if err != nil {
			return nil, "", err
		}
	}

	c.mu.Lock()
	idx, staleNote := c.cached, c.fetchError
	c.mu.Unlock()

	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]Item, 0, len(idx.Plugins))
	for _, it := range idx.Plugins {
		if q == "" ||
			strings.Contains(strings.ToLower(it.ID), q) ||
			strings.Contains(strings.ToLower(it.Name), q) ||
			strings.Contains(strings.ToLower(it.Description), q) ||
			strings.Contains(strings.ToLower(it.Publisher), q) {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, staleNote, nil
}

// Find returns the newest entry for an id (highest version string, stable
// for ties).
func (c *Client) Find(id string) (Item, bool, error) {
	items, _, err := c.List("")
	if err != nil {
		return Item{}, false, err
	}
	var found Item
	var ok bool
	for _, it := range items {
		if it.ID != id {
			continue
		}
		if !ok || it.Version > found.Version {
			found, ok = it, true
		}
	}
	return found, ok, nil
}

func (c *Client) fetch() (*Index, error) {
	resp, err := c.http.Get(c.indexURL)
	if err != nil {
		return nil, fmt.Errorf("fetch index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch index: http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	if idx.Plugins == nil {
		idx.Plugins = []Item{}
	}
	return &idx, nil
}

// --- package download + verification ----------------------------------------

// Download fetches the archive bytes for an item (64MiB budget).
func (c *Client) Download(it Item) ([]byte, error) {
	resp, err := c.http.Get(it.Tarball)
	if err != nil {
		return nil, fmt.Errorf("download package: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download package: http %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxPackageBytes))
}

const maxPackageBytes = 64 << 20

// TrustedKeys parses the SG_MARKETPLACE_PUBKEYS env value: comma-separated
// base64 ed25519 public keys. Empty/invalid entries are skipped; an empty
// result means installs are disabled (fail closed).
func TrustedKeys() []ed25519.PublicKey {
	raw := os.Getenv("SG_MARKETPLACE_PUBKEYS")
	var keys []ed25519.PublicKey
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(part)
		if err != nil || len(data) != ed25519.PublicKeySize {
			continue
		}
		keys = append(keys, ed25519.PublicKey(data))
	}
	return keys
}

// ErrNoTrustedKeys is returned when installs are attempted without any
// configured publisher key.
var ErrNoTrustedKeys = fmt.Errorf("no trusted publisher keys configured (SG_MARKETPLACE_PUBKEYS); marketplace install disabled")

// Verify checks the sha256 digest and the ed25519 signature of the archive
// bytes against the trusted key set. Both must pass.
func Verify(data []byte, it Item, keys []ed25519.PublicKey) error {
	if len(keys) == 0 {
		return ErrNoTrustedKeys
	}
	// digest first: cheap and catches corruption before signature math
	if d := it.Digest; d != "" {
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), d) {
			return fmt.Errorf("digest mismatch: package bytes do not match index entry")
		}
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(it.Signature))
	if err != nil {
		return fmt.Errorf("signature decode: %w", err)
	}
	for _, k := range keys {
		if ed25519.Verify(k, data, sig) {
			return nil
		}
	}
	return fmt.Errorf("signature does not match any trusted publisher key")
}
