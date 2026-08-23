// Package plugins implements the plugin lifecycle (panels-roadmap P3):
// a plugin is a self-contained distribution package that may carry breed
// definitions (BreedConfig source="plugin") and skills (SKILL.md dirs).
// Install lands the package on disk with every skill entering the pending
// security review; enabling registers its breeds through the existing
// settings-store validation; uninstall removes everything it added.
//
// Security rails (see docs/plans/panels-roadmap.md P3):
//   - archives unpack into a temp dir first; zip-slip (entries escaping the
//     target via absolute paths or ..) is rejected
//   - installs start disabled: skills must be approved via the skills
//     security pipeline before the plugin can be enabled
//   - the manifest declares permissions; consumers enforce them
package plugins

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Manifest is plugin.json at the archive root.
type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Validate checks manifest invariants.
func (m *Manifest) Validate() error {
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("plugin id must match %s", idPattern.String())
	}
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("plugin name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return errors.New("plugin version is required")
	}
	if len([]rune(m.Description)) > 2000 {
		return errors.New("description too long (max 2000 runes)")
	}
	return nil
}

// Entry is the registry record for one installed plugin.
type Entry struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	Enabled     bool      `json:"enabled"`
	InstalledAt time.Time `json:"installed_at"`
}

// View is the read model served by the API: registry entry + manifest +
// what the package carries (breed ids, skill ids) for the UI.
type View struct {
	Manifest
	Enabled     bool      `json:"enabled"`
	InstalledAt time.Time `json:"installed_at"`
	BreedIDs    []string  `json:"breeds"`
	SkillIDs    []string  `json:"skills"`
}

// Service owns the plugin directory layout and registry:
//
//	<configRoot>/plugins/<id>/         plugin payload
//	<configRoot>/plugins/registry.json installed entries
type Service struct {
	root string // <configRoot>/plugins
}

func NewService(configRoot string) *Service {
	return &Service{root: filepath.Join(configRoot, "plugins")}
}

func (s *Service) dir(id string) string       { return filepath.Join(s.root, id) }
func (s *Service) skillsDir(id string) string { return filepath.Join(s.dir(id), "skills") }

// registry ------------------------------------------------------------------

type registryDoc struct {
	Plugins []Entry `json:"plugins"`
}

func (s *Service) loadRegistry() registryDoc {
	doc := registryDoc{Plugins: []Entry{}}
	data, err := os.ReadFile(filepath.Join(s.root, "registry.json"))
	if err != nil {
		return doc
	}
	_ = json.Unmarshal(data, &doc)
	if doc.Plugins == nil {
		doc.Plugins = []Entry{}
	}
	return doc
}

func (s *Service) saveRegistry(doc registryDoc) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.root, "registry.json"), data, 0o600)
}

// lifecycle -----------------------------------------------------------------

// Install unpacks a plugin archive (zip) into a temp dir, validates the
// manifest, and moves the payload to its final directory. The plugin is
// registered DISABLED: enable requires the operator (and approved skills).
// Reinstalling an existing id is refused — uninstall first.
func (s *Service) Install(f SeekableReader) (View, error) {
	zr, _, err := openZip(f)
	if err != nil {
		return View{}, fmt.Errorf("open archive: %w", err)
	}

	tmp, err := os.MkdirTemp("", "sg-plugin-*")
	if err != nil {
		return View{}, err
	}
	defer os.RemoveAll(tmp)

	if err := extractZip(zr, tmp); err != nil {
		return View{}, fmt.Errorf("unpack: %w", err)
	}
	// Allow the manifest at the archive root or one level down (common
	// single-top-level-dir layout).
	manifest, payloadDir, err := findManifest(tmp)
	if err != nil {
		return View{}, err
	}

	dst := s.dir(manifest.ID)
	if _, err := os.Stat(dst); err == nil {
		return View{}, fmt.Errorf("plugin %s already installed (uninstall first)", manifest.ID)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return View{}, err
	}
	if err := os.Rename(payloadDir, dst); err != nil {
		return View{}, fmt.Errorf("move payload: %w", err)
	}

	doc := s.loadRegistry()
	doc.Plugins = append(doc.Plugins, Entry{
		ID:          manifest.ID,
		Version:     manifest.Version,
		Enabled:     false,
		InstalledAt: time.Now().UTC(),
	})
	if err := s.saveRegistry(doc); err != nil {
		_ = os.RemoveAll(dst)
		return View{}, err
	}
	return s.view(manifest.ID)
}

// List returns every installed plugin (registry order, oldest first).
func (s *Service) List() ([]View, error) {
	doc := s.loadRegistry()
	views := make([]View, 0, len(doc.Plugins))
	for _, e := range doc.Plugins {
		v, err := s.view(e.ID)
		if err != nil {
			continue // orphaned payload without a readable manifest: skip
		}
		views = append(views, v)
	}
	return views, nil
}

// Get returns one plugin view.
func (s *Service) Get(id string) (View, error) {
	return s.view(id)
}

// SetEnabled flips the enabled flag in the registry. Enabling an unknown
// plugin fails; the skills-approved precondition is enforced by the caller
// (transport) which owns the SkillManager.
func (s *Service) SetEnabled(id string, enabled bool) error {
	doc := s.loadRegistry()
	for i := range doc.Plugins {
		if doc.Plugins[i].ID == id {
			doc.Plugins[i].Enabled = enabled
			return s.saveRegistry(doc)
		}
	}
	return fmt.Errorf("plugin %s not installed", id)
}

// Uninstall removes the payload and registry entry. The caller cleans up any
// breeds/skills it registered on enable.
func (s *Service) Uninstall(id string) error {
	doc := s.loadRegistry()
	idx := -1
	for i := range doc.Plugins {
		if doc.Plugins[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("plugin %s not installed", id)
	}
	doc.Plugins = append(doc.Plugins[:idx], doc.Plugins[idx+1:]...)
	if err := s.saveRegistry(doc); err != nil {
		return err
	}
	return os.RemoveAll(s.dir(id))
}

// Payload accessors for the enable/disable hooks ------------------------------

// SkillsDir returns the plugin's skills source dir (may not exist).
func (s *Service) SkillsDir(id string) string { return s.skillsDir(id) }

// EnabledPlugins lists ids of plugins currently enabled (restart recovery).
func (s *Service) EnabledPlugins() []string {
	doc := s.loadRegistry()
	var out []string
	for _, e := range doc.Plugins {
		if e.Enabled {
			out = append(out, e.ID)
		}
	}
	return out
}

// BreedConfigs reads breeds/*.json from the payload. Malformed files are
// skipped and reported in the returned error list (install-preview tolerant).
func (s *Service) BreedConfigs(id string) (raws []json.RawMessage, ids []string, errs []string) {
	dir := filepath.Join(s.dir(id), "breeds")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, nil // no breeds shipped
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, e.Name()+": "+err.Error())
			continue
		}
		var probe struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(data, &probe)
		raws = append(raws, json.RawMessage(data))
		ids = append(ids, probe.ID)
	}
	return raws, ids, errs
}

// view assembles the API read model from registry + payload.
func (s *Service) view(id string) (View, error) {
	var m Manifest
	data, err := os.ReadFile(filepath.Join(s.dir(id), "plugin.json"))
	if err != nil {
		return View{}, fmt.Errorf("plugin %s: manifest missing: %w", id, err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return View{}, fmt.Errorf("plugin %s: manifest invalid: %w", id, err)
	}
	v := View{Manifest: m}
	for _, e := range s.loadRegistry().Plugins {
		if e.ID == id {
			v.Enabled = e.Enabled
			v.InstalledAt = e.InstalledAt
			v.Version = e.Version
			break
		}
	}
	// skill ids from directory names carrying SKILL.md
	if entries, err := os.ReadDir(s.skillsDir(id)); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				if _, err := os.Stat(filepath.Join(s.skillsDir(id), e.Name(), "SKILL.md")); err == nil {
					v.SkillIDs = append(v.SkillIDs, e.Name())
				}
			}
		}
	}
	_, breedIDs, _ := s.BreedConfigs(id)
	v.BreedIDs = breedIDs
	sort.Strings(v.SkillIDs)
	sort.Strings(v.BreedIDs)
	return v, nil
}

// Total uncompressed size is capped so a hostile archive cannot fill the
// disk (zip-bomb defense alongside the zip-slip checks in safeJoin).
const maxUncompressed = 64 << 20 // 64 MiB

func extractZip(zr *zip.Reader, dst string) error {
	var total uint64
	for _, f := range zr.File {
		clean, err := safeJoin(dst, f.Name)
		if err != nil {
			return fmt.Errorf("entry %q rejected: %w", f.Name, err)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(clean, 0o755); err != nil {
				return err
			}
			continue
		}
		total += f.UncompressedSize64
		if total > maxUncompressed {
			return errors.New("archive too large when decompressed (max 64MiB)")
		}
		if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
			return err
		}
		if err := copyZipFile(f, clean); err != nil {
			return err
		}
	}
	return nil
}

func copyZipFile(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(rc, maxUncompressed))
	return err
}

// safeJoin resolves name inside dst, erroring if it escapes dst.
func safeJoin(dst, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes archive root")
	}
	target := filepath.Join(dst, clean)
	rel, err := filepath.Rel(dst, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes archive root")
	}
	return target, nil
}

// findManifest locates plugin.json at root or one level down; it also refuses
// ambiguous layouts (multiple candidate manifests).
func findManifest(tmp string) (Manifest, string, error) {
	candidates := []string{filepath.Join(tmp, "plugin.json")}
	entries, err := os.ReadDir(tmp)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				p := filepath.Join(tmp, e.Name(), "plugin.json")
				if _, err := os.Stat(p); err == nil {
					candidates = append(candidates, p)
				}
			}
		}
	}
	found := -1
	for i, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if found >= 0 {
				return Manifest{}, "", errors.New("multiple plugin.json candidates in archive")
			}
			found = i
		}
	}
	if found < 0 {
		return Manifest{}, "", errors.New("plugin.json not found in archive")
	}
	data, err := os.ReadFile(candidates[found])
	if err != nil {
		return Manifest{}, "", err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, "", fmt.Errorf("invalid plugin.json: %w", err)
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, "", err
	}
	return m, filepath.Dir(candidates[found]), nil
}
