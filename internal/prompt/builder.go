package prompt

import (
	"fmt"
	"strings"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/skills"
)

// Builder constructs system prompts for CLI adapter invocations.
// It assembles breed identity, teammate roster, safety rules, and skill prompts
// into a single system prompt string injected into the CLI agent.
type Builder struct {
	breeds map[string]*config.BreedConfig
	skills *skills.SkillManager
}

// NewBuilder creates a prompt builder from breed configs and skills.
func NewBuilder(breeds map[string]*config.BreedConfig, skillMgr *skills.SkillManager) *Builder {
	return &Builder{
		breeds: breeds,
		skills: skillMgr,
	}
}

// BuildRequest specifies what to include in the system prompt.
type BuildRequest struct {
	BreedID   string   // target breed
	VariantID string   // specific variant (empty = default)
	SkillIDs  []string // skill prompt IDs to inject
	RAGContext string  // retrieved knowledge context (empty = no RAG)
}

// Build constructs the full system prompt for a CLI adapter invocation.
// The prompt is composed of:
//  1. Static identity — breed name, personality, role, restrictions
//  2. Teammate roster — other breeds available for collaboration
//  3. Variant system prompt — breed's own custom prompt from config
//  4. Skills — injected skill prompts
func (b *Builder) Build(req BuildRequest) string {
	breed, ok := b.breeds[req.BreedID]
	if !ok {
		return ""
	}

	variant := breed.DefaultVariant()
	if req.VariantID != "" {
		for i := range breed.Variants {
			if breed.Variants[i].ID == req.VariantID {
				variant = &breed.Variants[i]
				break
			}
		}
	}
	if variant == nil {
		return ""
	}

	var sb strings.Builder

	// 1. Static identity
	sb.WriteString(b.buildIdentity(breed))

	// 2. Teammate roster
	sb.WriteString(b.buildRoster(req.BreedID))

	// 3. Variant system prompt
	if variant.SystemPrompt != "" {
		sb.WriteString("\n## 角色提示\n\n")
		sb.WriteString(variant.SystemPrompt)
		sb.WriteString("\n")
	}

	// 4. Skill roster (always injected, not gated by SkillIDs)
	sb.WriteString(b.buildSkillRoster())

	// 5. Skill bodies (only if explicitly requested via SkillIDs)
	if len(req.SkillIDs) > 0 && b.skills != nil {
		sb.WriteString(b.buildSkills(req.SkillIDs))
	}

	// 6. RAG context (retrieved knowledge from vector store)
	if req.RAGContext != "" {
		sb.WriteString("\n## Retrieved Knowledge\n\n")
		sb.WriteString(req.RAGContext)
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildSkillRoster generates a markdown table of all available skills.
// Lists skill name, description (truncated to 80 chars), and triggers.
// Does NOT inject skill bodies — agents see the roster and self-select.
func (b *Builder) buildSkillRoster() string {
	if b.skills == nil {
		return ""
	}
	all := b.skills.All()
	if len(all) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 可用技能\n\n")
	sb.WriteString("| 技能 | 描述 | 触发词 |\n")
	sb.WriteString("|------|------|--------|\n")

	for _, skill := range all {
		desc := skill.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		triggers := strings.Join(skill.AllTriggers(), ", ")
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", skill.Name, desc, triggers))
	}
	sb.WriteString("\n")
	return sb.String()
}

// buildIdentity constructs the breed's static identity section.
func (b *Builder) buildIdentity(breed *config.BreedConfig) string {
	var sb strings.Builder

	sb.WriteString("# 身份\n\n")
	sb.WriteString(fmt.Sprintf("你是 **%s**（%s），犬种特工队的成员。\n\n", breed.DisplayName, breed.Name))

	if breed.Personality != "" {
		sb.WriteString(fmt.Sprintf("**性格：** %s\n\n", breed.Personality))
	}
	if breed.RoleDescription != "" {
		sb.WriteString(fmt.Sprintf("**职责：** %s\n\n", breed.RoleDescription))
	}
	if breed.TeamStrengths != "" {
		sb.WriteString(fmt.Sprintf("**擅长：** %s\n\n", breed.TeamStrengths))
	}

	// Restrictions
	sb.WriteString("## 限制\n\n")
	sb.WriteString("- 不越界：只做自己职责范围内的事\n")
	sb.WriteString("- 不推理他人的领域：需要跨领域协作时，@mention 对应的队友\n")
	sb.WriteString("- 遵守铁律：不删除数据存储、不杀进程、不改运行时配置\n\n")

	return sb.String()
}

// buildRoster constructs the teammate roster section.
func (b *Builder) buildRoster(selfID string) string {
	if len(b.breeds) <= 1 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 队友名册\n\n")
	sb.WriteString("| 犬种 | @mention | 擅长 | 职责 |\n")
	sb.WriteString("|------|----------|------|------|\n")

	for id, breed := range b.breeds {
		if id == selfID {
			continue
		}

		mention := ""
		if len(breed.MentionPatterns) > 0 {
			mention = breed.MentionPatterns[0]
		}

		strengths := breed.TeamStrengths
		if strengths == "" && len(breed.Variants) > 0 {
			v := breed.Variants[0]
			if len(v.Strengths) > 0 {
				strengths = strings.Join(v.Strengths, ", ")
			}
		}

		role := breed.RoleDescription
		if role == "" {
			role = "—"
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			breed.DisplayName, mention, strengths, role))
	}

	sb.WriteString("\n")
	return sb.String()
}

// buildSkills constructs the injected skills section.
func (b *Builder) buildSkills(skillIDs []string) string {
	var sb strings.Builder
	sb.WriteString("## 技能\n\n")

	for _, id := range skillIDs {
		skill := b.skills.Get(id)
		if skill == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n", skill.Name))
		if skill.Description != "" {
			sb.WriteString(skill.Description)
			sb.WriteString("\n\n")
		}
		if skill.Body != "" {
			sb.WriteString(skill.Body)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}
