package hooks

import (
	"strings"

	"sounds-great-ai/internal/skills"
)

type ResolveResult struct {
	Status string
	Reason string
	Vars   map[string]string
}

type Resolver interface {
	Resolve(input *AssemblerInput) ResolveResult
}

// IdentityResolver always fires, injecting breed identity.
type IdentityResolver struct{}

func (r *IdentityResolver) Resolve(input *AssemblerInput) ResolveResult {
	return ResolveResult{
		Status: "fired",
		Vars: map[string]string{
			"BreedID":         input.BreedID,
			"BreedName":       input.BreedName,
			"RoleDescription": input.RoleDescription,
		},
	}
}

// AlwaysFireResolver fires unconditionally (for restrictions, iron-laws, guardrails).
type AlwaysFireResolver struct{}

func (r *AlwaysFireResolver) Resolve(input *AssemblerInput) ResolveResult {
	return ResolveResult{Status: "fired"}
}

// PhaseAnchorResolver fires unconditionally, injecting current Phase.
type PhaseAnchorResolver struct{}

func (r *PhaseAnchorResolver) Resolve(input *AssemblerInput) ResolveResult {
	return ResolveResult{
		Status: "fired",
		Vars: map[string]string{
			"CurrentPhase": input.CurrentPhase,
		},
	}
}

// ReAnchorResolver fires only when ToolCallCount > 5.
type ReAnchorResolver struct{}

func (r *ReAnchorResolver) Resolve(input *AssemblerInput) ResolveResult {
	if input.ToolCallCount > 5 {
		return ResolveResult{Status: "fired"}
	}
	return ResolveResult{Status: "skipped", Reason: "tool_call_count_below_threshold"}
}

func DefaultResolvers() map[string]Resolver {
	return map[string]Resolver{
		"IdentityResolver":    &IdentityResolver{},
		"AlwaysFireResolver":  &AlwaysFireResolver{},
		"PhaseAnchorResolver": &PhaseAnchorResolver{},
		"ReAnchorResolver":    &ReAnchorResolver{},
		"LeaderRefResolver":   &LeaderRefResolver{},
	}
}

// SkillTriggerResolver 动态匹配当前查询命中的已启用 skill，并注入 SKILL_NAME /
// SKILL_TRIGGER / SKILL_ID 模板变量供 d11 hook 引用。匹配基于 trigger 与 name 的
// 子串包含，属确定性模式匹配（无 LLM），且匹配集合来自持久化意图（非硬编码）。
// 无命中时变量置空，模板渲染为空——符合「d11 只命名当前激活 skill，不粘贴正文」
// 的设计（正文由 Build 的 SkillIDs 注入）。
type SkillTriggerResolver struct {
	Skills *skills.SkillManager
}

func (r *SkillTriggerResolver) Resolve(input *AssemblerInput) ResolveResult {
	vars := map[string]string{"SKILL_NAME": "", "SKILL_TRIGGER": "", "SKILL_ID": ""}
	if r.Skills == nil || input.Query == "" {
		return ResolveResult{Status: "fired", Vars: vars}
	}
	q := strings.ToLower(input.Query)
	carrier := input.Carrier

	// 显式指令优先：查询中以 `skill:<id-or-name>` 形式给出精确意图信号，避免散文中
	// 提到 skill 名导致误命中。命中即返回，不进入子串宽松匹配。
	if tag, ok := extractSkillTag(q); ok {
		for _, s := range r.Skills.AllEnabled() {
			if !mountedToCarrier(r.Skills, s.ID, carrier) {
				continue
			}
			if s.ID == tag || strings.EqualFold(s.Name, tag) {
				vars["SKILL_NAME"] = s.Name
				vars["SKILL_TRIGGER"] = "skill:" + tag
				vars["SKILL_ID"] = s.ID
				return ResolveResult{Status: "fired", Vars: vars}
			}
		}
	}

	for _, s := range r.Skills.AllEnabled() {
		// G5：仅匹配挂载到当前 carrier 的 skill（MountPoints 为空=全 carrier），
		// 避免 d11 命名了未挂载到本 carrier 的 skill，与挂载范围错位。
		if !mountedToCarrier(r.Skills, s.ID, carrier) {
			continue
		}
		for _, tr := range s.AllTriggers() {
			if tr != "" && strings.Contains(q, strings.ToLower(tr)) {
				vars["SKILL_NAME"] = s.Name
				vars["SKILL_TRIGGER"] = tr
				vars["SKILL_ID"] = s.ID
				return ResolveResult{Status: "fired", Vars: vars}
			}
		}
		if s.Name != "" && strings.Contains(q, strings.ToLower(s.Name)) {
			vars["SKILL_NAME"] = s.Name
			vars["SKILL_ID"] = s.ID
			return ResolveResult{Status: "fired", Vars: vars}
		}
	}
	return ResolveResult{Status: "fired", Vars: vars}
}

// extractSkillTag 从查询中提取显式 `skill:<token>` 指令。`token` 允许字母、数字、
// 连字符、下划线（匹配 skill id 与常见 name 形式）。大小写不敏感。
func extractSkillTag(q string) (string, bool) {
	const prefix = "skill:"
	idx := strings.Index(q, prefix)
	if idx < 0 {
		return "", false
	}
	rest := q[idx+len(prefix):]
	end := 0
	for end < len(rest) {
		c := rest[end]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			end++
			continue
		}
		break
	}
	if end == 0 {
		return "", false
	}
	return rest[:end], true
}

// mountedToCarrier 判断某 skill 是否挂载到给定 carrier，供 d11 resolver 按执行
// 目标的挂载范围过滤。carrier 为空（未知来源）时不做过滤保持宽松；MountPoints
// 为空表示挂载到所有 carrier（与前端「全量（默认）」语义一致）。
func mountedToCarrier(mgr *skills.SkillManager, id, carrier string) bool {
	if carrier == "" {
		return true
	}
	it := mgr.IntentOf(id)
	if it == nil || !it.Enabled {
		return false
	}
	if len(it.MountPoints) == 0 {
		return true
	}
	for _, c := range it.MountPoints {
		if c == carrier {
			return true
		}
	}
	return false
}
