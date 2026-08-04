package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SkillManager struct {
	skills map[string]*Skill
	dir    string
}

func NewManager(dir string) *SkillManager {
	return &SkillManager{skills: make(map[string]*Skill), dir: dir}
}

func (m *SkillManager) LoadFromDir() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return fmt.Errorf("read skills dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(m.dir, entry.Name(), "SKILL.md")
			data, err := os.ReadFile(skillPath)
			if err != nil {
				continue
			}
			skill, err := ParseSkill(skillPath, data)
			if err != nil || skill.ID == "" {
				continue
			}
			m.skills[skill.ID] = skill
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, entry.Name()))
		if err != nil {
			continue
		}
		skill, err := ParseSkill(entry.Name(), data)
		if err != nil || skill.ID == "" {
			continue
		}
		m.skills[skill.ID] = skill
	}
	return nil
}

func (m *SkillManager) Get(id string) *Skill { return m.skills[id] }

func (m *SkillManager) InjectSkills(systemPrompt string, skills []*Skill) string {
	if len(skills) == 0 {
		return systemPrompt
	}
	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n--- Skills ---\n")
	for _, s := range skills {
		sb.WriteString(s.Body)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func (m *SkillManager) All() []*Skill {
	result := make([]*Skill, 0, len(m.skills))
	for _, s := range m.skills {
		result = append(result, s)
	}
	return result
}
