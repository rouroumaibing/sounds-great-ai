package prompt

import (
	"fmt"
	"strings"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/skills"

	"sounds-great-ai/pkg/pack"
)

// ProfileReader resolves a relationship capsule body for a breed's
// relationship key. It is implemented by *settings.ProfileRepository. Declared
// as an interface here (rather than importing the settings package) to avoid a
// hard import cycle: prompt must not depend on settings, and settings must not
// depend on prompt.
type ProfileReader interface {
	// ReadCapsuleBody returns the capsule primer text for the given
	// relationship key, whether one exists, and any I/O error. A missing
	// capsule is reported via the bool, never as an error.
	ReadCapsuleBody(relationshipKey string) (string, bool, error)
}

// ContinuityReader resolves the last-session digest for a breed (Persistent
// Identity P3, homologous F211 continuity bootstrap). Implemented by
// *settings.ContinuityStore; declared as an interface to avoid an import cycle.
type ContinuityReader interface {
	// LastDigest returns the persisted "what this breed was last doing" summary
	// for the given breed (latest rotation), whether one exists, and any I/O
	// error.
	LastDigest(breedID string) (string, bool, error)
	// LastDigestForRotation returns the persisted summary at a specific rotation
	// index (session-rotation-aware re-injection). A missing rotation reports
	// ok=false so the caller can fall back to LastDigest.
	LastDigestForRotation(breedID string, rotationIndex int) (string, bool, error)
}

// Builder constructs system prompts for CLI adapter invocations.
// It assembles breed identity, teammate roster, safety rules, and skill prompts
// into a single system prompt string injected into the CLI agent.
type Builder struct {
	breeds     map[string]*pack.BreedConfig
	skills     *skills.SkillManager
	profiles   ProfileReader
	continuity ContinuityReader
}

// NewBuilder creates a prompt builder from breed configs and skills.
func NewBuilder(breeds map[string]*pack.BreedConfig, skillMgr *skills.SkillManager) *Builder {
	return &Builder{
		breeds: breeds,
		skills: skillMgr,
	}
}

// SetProfiles attaches a relationship-capsule reader (Persistent Identity P1,
// homologous F231). When set, breeds that declare a relationship_key
// and have a persisted capsule get a "关系胶囊" section injected into their
// identity block, so the dog remembers its long-term relationship with the
// operator even after a fresh spawn or an auto-compact wiped history.
func (b *Builder) SetProfiles(p ProfileReader) {
	b.profiles = p
}

// SetContinuity attaches a continuity-digest reader (Persistent Identity P3,
// homologous F211). When set, a breed with a persisted last-session
// digest gets a "续接上下文" section injected, so the dog resumes awareness of
// what it was last working on across restarts and separate one-shot spawns.
func (b *Builder) SetContinuity(c ContinuityReader) {
	b.continuity = c
}

// BuildRequest specifies what to include in the system prompt.
type BuildRequest struct {
	BreedID    string   // target breed
	VariantID  string   // specific variant (empty = default)
	SkillIDs   []string // skill prompt IDs to inject
	RAGContext string   // retrieved knowledge context (empty = no RAG)
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
func (b *Builder) buildIdentity(breed *pack.BreedConfig) string {
	var sb strings.Builder

	sb.WriteString("# 身份\n\n")
	sb.WriteString(fmt.Sprintf("你是 **%s**（%s），狗狗特工队的成员。\n\n", breed.DisplayName, breed.Name))

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

	// Relationship capsule (Persistent Identity P1, homologous F231).
	// Injected as part of the static identity block: it is rebuilt on every
	// spawn, so the dog retains its relationship with the operator across
	// restarts and context compression (which only drops conversation history,
	// never this injected section).
	if b.profiles != nil && breed.RelationshipKey != "" {
		if body, ok, err := b.profiles.ReadCapsuleBody(breed.RelationshipKey); err == nil && ok && strings.TrimSpace(body) != "" {
			// Defensive guard (KD-7): clamp the injected body to the
			// 300-visible-rune budget so a capsule written before this rule or
			// hand-edited off-envelope can never inflate the identity block.
			// The stored value is already validated on write; this only
			// protects legacy/foreign capsule files.
			body = settings.TruncateCapsuleBody(body)
			if strings.TrimSpace(body) == "" {
				goto skipCapsule
			}
			sb.WriteString("## 关系胶囊\n\n")
			sb.WriteString("> 这是你与 operator 之间长期关系的持久画像。它独立于本次会话历史——即使历史被压缩或进程重启，这段关系依然有效。\n\n")
			sb.WriteString(strings.TrimSpace(body))
			sb.WriteString("\n")
		}
	}
skipCapsule:

	// Continuity digest (Persistent Identity P3, homologous F211).
	// Injected as part of the static identity block so the dog resumes awareness
	// of what it was last working on. In one-shot mode each spawn rebuilds this
	// from the persisted digest, preventing a cold start across turns/restarts.
	if b.continuity != nil {
		if summary, ok, err := b.continuity.LastDigest(breed.ID); err == nil && ok && strings.TrimSpace(summary) != "" {
			sb.WriteString("## 续接上下文\n\n")
			sb.WriteString("> 这是你上一次会话留下的续接记录（独立于本次会话历史，重启/新进程仍有效）。若相关，请接着推进；若已无关可忽略。\n\n")
			sb.WriteString(strings.TrimSpace(summary))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// buildRoster constructs the teammate roster section.
func (b *Builder) buildRoster(selfID string) string {
	if len(b.breeds) <= 1 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 队友名册\n\n")
	sb.WriteString("| 狗狗 | @mention | 擅长 | 职责 |\n")
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
