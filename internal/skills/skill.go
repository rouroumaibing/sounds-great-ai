package skills

import (
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	ID          string
	Name        string
	Description string
	Trigger     string
	Triggers    []string
	NotFor      []string
	Output      string
	Body        string
	FilePath    string
}

type skillFrontmatter struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Trigger     string   `yaml:"trigger"`
	Triggers    []string `yaml:"triggers"`
	NotFor      []string `yaml:"not_for"`
	Output      string   `yaml:"output"`
}

func ParseSkill(path string, data []byte) (*Skill, error) {
	content := string(data)
	s := &Skill{FilePath: path}
	if !strings.HasPrefix(content, "---\n") {
		s.Body = content
		return s, nil
	}
	parts := strings.SplitN(content[4:], "\n---\n", 2)
	if len(parts) != 2 {
		s.Body = content
		return s, nil
	}
	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
		s.Body = content
		return s, nil
	}
	s.ID = fm.ID
	s.Name = fm.Name
	s.Description = fm.Description
	s.Trigger = fm.Trigger
	s.Triggers = fm.Triggers
	s.NotFor = fm.NotFor
	s.Output = fm.Output
	s.Body = strings.TrimPrefix(parts[1], "\n")
	if s.ID == "" {
		if s.Name != "" {
			s.ID = kebabCase(s.Name)
		} else {
			s.ID = idFromFilename(path)
		}
	}
	return s, nil
}

func (s *Skill) AllTriggers() []string {
	if len(s.Triggers) > 0 {
		return s.Triggers
	}
	if s.Trigger != "" {
		return []string{s.Trigger}
	}
	return nil
}

func kebabCase(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "-"))
}

func idFromFilename(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".md")
	if strings.EqualFold(base, "SKILL") {
		return strings.ToLower(filepath.Base(filepath.Dir(path)))
	}
	return strings.ToLower(base)
}
