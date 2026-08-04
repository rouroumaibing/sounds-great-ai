package agent

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type SkillManager struct {
	mu     sync.RWMutex
	skills map[string]string
}

func (s *SkillManager) Load(skillDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skills = make(map[string]string)
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(skillDir, entry.Name(), "SKILL.md")
			content, err := os.ReadFile(skillPath)
			if err != nil {
				continue
			}
			s.skills[entry.Name()] = string(content)
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(skillDir, name))
		if err != nil {
			return err
		}
		skillName := strings.TrimSuffix(name, ".md")
		s.skills[skillName] = string(content)
	}
	return nil
}

func (s *SkillManager) Get(name string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	content, ok := s.skills[name]
	return content, ok
}

func (s *SkillManager) Reload(skillDir string) error {
	return s.Load(skillDir)
}
