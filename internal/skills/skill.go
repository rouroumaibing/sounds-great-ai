package skills

import (
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill 是单个技能的内存表示。字段分为 frontmatter 元数据（ID/Name/Description
// /Category/Triggers/RequiresMcp/NotFor/Output）与运行时字段（Source 由加载目录
// 决定、Body 为正文、FilePath 为源文件路径）。
type Skill struct {
	ID          string
	Name        string
	Description string
	Category    string
	Trigger     string
	Triggers    []string
	NotFor      []string
	RequiresMcp []string
	Output      string
	Source      string // packs | user | plugin（由加载目录决定）
	Body        string
	FilePath    string
}

type skillFrontmatter struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Trigger     string   `yaml:"trigger"`
	Triggers    []string `yaml:"triggers"`
	Category    string   `yaml:"category"`
	NotFor      []string `yaml:"not_for"`
	RequiresMcp []string `yaml:"requires_mcp"`
	Output      string   `yaml:"output"`
}

// ParseSkill 解析一个 SKILL.md（Markdown + YAML frontmatter）为 Skill。
// 无 frontmatter 或解析失败时整体回退为正文，保证向前兼容。
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
	s.Category = fm.Category
	s.Trigger = fm.Trigger
	s.Triggers = fm.Triggers
	s.NotFor = fm.NotFor
	s.RequiresMcp = fm.RequiresMcp
	s.Output = fm.Output
	s.Body = strings.TrimPrefix(parts[1], "\n")
	if s.ID == "" {
		if s.Name != "" {
			s.ID = kebabCase(s.Name)
		} else {
			s.ID = idFromFilename(path)
		}
	}
	// 旧格式兜底：frontmatter 无 triggers 时，尝试从描述中抽取「触发词：」。
	if len(s.Triggers) == 0 {
		if ts := extractTriggersFromDescription(s.Description); len(ts) > 0 {
			s.Triggers = ts
		}
	}
	return s, nil
}

// extractTriggersFromDescription 解析描述中以「触发词：」/「触发词:」开头的逗号
// （支持全角「，」与半角「,」）分隔触发词列表，作为旧格式 skill 的兜底。
func extractTriggersFromDescription(desc string) []string {
	for _, prefix := range []string{"触发词：", "触发词:"} {
		if idx := strings.Index(desc, prefix); idx >= 0 {
			rest := desc[idx+len(prefix):]
			if end := strings.IndexAny(rest, "\n。"); end >= 0 {
				rest = rest[:end]
			}
			parts := strings.Split(rest, "，")
			if len(parts) == 1 {
				parts = strings.Split(rest, ",")
			}
			var out []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					out = append(out, p)
				}
			}
			return out
		}
	}
	return nil
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
