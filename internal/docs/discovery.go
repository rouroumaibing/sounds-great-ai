// Package docs discovers documentation profiles from frontmatter and builds an
// index (F243). It also exposes a CI lint that fails closed on docs missing a
// required title.
package docs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DocProfile is the discovered profile of a doc file.
type DocProfile struct {
	Path  string
	Title string
	Tags  []string
	Kind  string
}

// Discover scans dir for *.md files and extracts frontmatter profiles. A doc
// with no frontmatter yields a profile with an empty Title (so Lint can flag).
func Discover(dir string) ([]DocProfile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []DocProfile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, profileFor(e.Name(), string(data)))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func profileFor(name, content string) DocProfile {
	p := DocProfile{Path: name}
	if strings.HasPrefix(content, "---\n") {
		end := strings.Index(content[4:], "\n---")
		if end >= 0 {
			fm := content[4 : 4+end]
			for _, line := range strings.Split(fm, "\n") {
				k, v, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				k = strings.TrimSpace(k)
				v = strings.TrimSpace(v)
				switch k {
				case "title":
					p.Title = v
				case "kind":
					p.Kind = v
				case "tags":
					p.Tags = splitTags(v)
				}
			}
		}
	}
	return p
}

func splitTags(s string) []string {
	s = strings.Trim(s, "[]")
	var out []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Lint fails closed: returns the paths of docs missing a title (F243 CI lint).
func Lint(profiles []DocProfile) []string {
	var bad []string
	for _, p := range profiles {
		if p.Title == "" {
			bad = append(bad, p.Path)
		}
	}
	return bad
}
