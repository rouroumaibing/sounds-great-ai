package hooks

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

type RegisteredHook struct {
	Manifest HookManifest
	Template string
	Dir      string
}

type Registry struct {
	hooks    map[string]*RegisteredHook
	hooksDir string
}

func NewRegistry(hooksDir string) *Registry {
	return &Registry{
		hooks:    make(map[string]*RegisteredHook),
		hooksDir: hooksDir,
	}
}

func (r *Registry) Scan() error {
	entries, err := os.ReadDir(r.hooksDir)
	if err != nil {
		return fmt.Errorf("read hooks dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(r.hooksDir, entry.Name())
		m, err := ParseHookManifest(dir)
		if err != nil {
			log.Printf("Warning: skip hook %s: %v", entry.Name(), err)
			continue
		}
		if _, exists := r.hooks[m.ID]; exists {
			log.Printf("Warning: duplicate hook ID %s, skipping", m.ID)
			continue
		}
		tmplContent := ""
		if m.Template != "" {
			data, _ := os.ReadFile(filepath.Join(dir, m.Template))
			tmplContent = string(data)
		}
		r.hooks[m.ID] = &RegisteredHook{
			Manifest: *m,
			Template: tmplContent,
			Dir:      dir,
		}
	}
	return nil
}

func (r *Registry) GetStageHooks(stage string) []*RegisteredHook {
	var result []*RegisteredHook
	for _, h := range r.hooks {
		if h.Manifest.Stage == stage && h.Manifest.Enabled {
			result = append(result, h)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Manifest.Order < result[j].Manifest.Order
	})
	return result
}

func (r *Registry) Get(id string) *RegisteredHook {
	return r.hooks[id]
}

func (r *Registry) All() []*RegisteredHook {
	result := make([]*RegisteredHook, 0, len(r.hooks))
	for _, h := range r.hooks {
		result = append(result, h)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Manifest.Stage != result[j].Manifest.Stage {
			return result[i].Manifest.Stage < result[j].Manifest.Stage
		}
		return result[i].Manifest.Order < result[j].Manifest.Order
	})
	return result
}
