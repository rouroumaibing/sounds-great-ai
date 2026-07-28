package capability

import (
	"bufio"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"sounds-great-ai/pkg/pack"
)

// SearchMatch represents a single code search match.
type SearchMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// CodeSearch is a file system code search adapter.
type CodeSearch struct {
	workspace string
}

// NewCodeSearch creates a new CodeSearch capability for the given workspace.
func NewCodeSearch(workspace string) *CodeSearch {
	return &CodeSearch{workspace: workspace}
}

func (c *CodeSearch) Name() string    { return "code_search" }
func (c *CodeSearch) Version() string { return "v1" }

func (c *CodeSearch) Init(ctx context.Context) error { return nil }
func (c *CodeSearch) Health() error                  { return nil }
func (c *CodeSearch) Close() error                   { return nil }

// getStringConfig extracts a string value from a config map with a default fallback.
func getStringConfig(cfg map[string]any, key string, defaultVal string) string {
	if cfg == nil {
		return defaultVal
	}
	v, ok := cfg[key]
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok {
		return defaultVal
	}
	return s
}

// getBoolConfig extracts a bool value from a config map with a default fallback.
// It handles bool values and the string "true".
func getBoolConfig(cfg map[string]any, key string, defaultVal bool) bool {
	if cfg == nil {
		return defaultVal
	}
	v, ok := cfg[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true"
	default:
		return defaultVal
	}
}

var errMaxMatches = errors.New("code_search: max matches reached")

// Run executes the code search over the workspace files.
func (c *CodeSearch) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	pattern := getStringConfig(input.CapabilityConfig, "pattern", "")
	if pattern == "" {
		pattern = input.Query
	}
	if pattern == "" {
		return nil, errors.New("code_search: empty pattern")
	}

	ignoreCase := getBoolConfig(input.CapabilityConfig, "ignore_case", false)

	searchPattern := pattern
	if ignoreCase {
		searchPattern = strings.ToLower(pattern)
	}

	validExts := map[string]bool{
		".go": true, ".ts": true, ".js": true, ".py": true,
		".md": true, ".json": true, ".yaml": true, ".yml": true,
	}
	skipDirs := map[string]bool{
		".git": true, "vendor": true, "node_modules": true, "readonly-docs": true,
	}

	var matches []SearchMatch

	err := filepath.WalkDir(c.workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(matches) >= 100 {
			return errMaxMatches
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		if !validExts[filepath.Ext(path)] {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		relPath, err := filepath.Rel(c.workspace, path)
		if err != nil {
			relPath = path
		}
		relPath = filepath.ToSlash(relPath)

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if len(matches) >= 100 {
				break
			}
			line := scanner.Text()
			lineToSearch := line
			if ignoreCase {
				lineToSearch = strings.ToLower(line)
			}
			if strings.Contains(lineToSearch, searchPattern) {
				matches = append(matches, SearchMatch{
					File:    relPath,
					Line:    lineNum,
					Content: line,
				})
			}
		}
		// Silently continue on scanner errors (e.g. long lines exceeding buffer).
		_ = scanner.Err()

		return nil
	})
	if err != nil && !errors.Is(err, errMaxMatches) {
		return nil, err
	}

	results := make([]any, len(matches))
	for i, m := range matches {
		results[i] = m
	}

	return &pack.TaskOutput{
		Data: map[string]any{
			"matches": matches,
			"count":   len(matches),
		},
		Results: results,
	}, nil
}
