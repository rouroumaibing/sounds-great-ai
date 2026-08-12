package prompt

import (
	"strings"
	"testing"

	"sounds-great-ai/internal/skills"

	"sounds-great-ai/pkg/pack"
)

func TestBuilderBuildIdentity(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {
			ID:               "bianmu",
			Name:             "边牧",
			DisplayName:      "边牧",
			Personality:      "聪明、善于统筹全局",
			RoleDescription:  "任务分解与调度专家",
			TeamStrengths:    "任务分解、多agent调度",
			DefaultVariantID: "v1",
			Variants: []pack.Variant{
				{ID: "v1", SystemPrompt: "你是边牧，指挥官。"},
			},
		},
	}
	b := NewBuilder(breeds, nil)
	result := b.Build(BuildRequest{BreedID: "bianmu"})

	if !strings.Contains(result, "边牧") {
		t.Error("expected breed name in output")
	}
	if !strings.Contains(result, "聪明、善于统筹全局") {
		t.Error("expected personality in output")
	}
	if !strings.Contains(result, "任务分解与调度专家") {
		t.Error("expected role description in output")
	}
	if !strings.Contains(result, "你是边牧，指挥官。") {
		t.Error("expected variant system prompt in output")
	}
	if !strings.Contains(result, "## 限制") {
		t.Error("expected restrictions section")
	}
}

func TestBuilderBuildRoster(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {
			ID: "bianmu", Name: "边牧", DisplayName: "边牧",
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1"}},
		},
		"xigou": {
			ID: "xigou", Name: "细狗", DisplayName: "细狗",
			TeamStrengths:    "代码搜索、分析",
			RoleDescription:  "代码搜索专家",
			MentionPatterns:  []string{"@细狗", "@xigou"},
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1"}},
		},
	}
	b := NewBuilder(breeds, nil)
	result := b.Build(BuildRequest{BreedID: "bianmu"})

	if !strings.Contains(result, "## 队友名册") {
		t.Error("expected teammate roster section")
	}
	if !strings.Contains(result, "细狗") {
		t.Error("expected teammate name in roster")
	}
	if !strings.Contains(result, "@细狗") {
		t.Error("expected mention pattern in roster")
	}
	if !strings.Contains(result, "代码搜索、分析") {
		t.Error("expected strengths in roster")
	}
}

func TestBuilderBuildUnknownBreed(t *testing.T) {
	t.Parallel()
	b := NewBuilder(map[string]*pack.BreedConfig{}, nil)
	result := b.Build(BuildRequest{BreedID: "nonexistent"})
	if result != "" {
		t.Error("expected empty string for unknown breed")
	}
}

func TestBuilderBuildNoRosterForSingleBreed(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {
			ID: "bianmu", Name: "边牧", DisplayName: "边牧",
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1"}},
		},
	}
	b := NewBuilder(breeds, nil)
	result := b.Build(BuildRequest{BreedID: "bianmu"})
	if strings.Contains(result, "## 队友名册") {
		t.Error("expected no roster for single breed")
	}
}

func TestBuilderBuildSpecificVariant(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"bianmu": {
			ID: "bianmu", Name: "边牧", DisplayName: "边牧",
			DefaultVariantID: "v1",
			Variants: []pack.Variant{
				{ID: "v1", SystemPrompt: "variant 1 prompt"},
				{ID: "v2", SystemPrompt: "variant 2 prompt"},
			},
		},
	}
	b := NewBuilder(breeds, nil)
	result := b.Build(BuildRequest{BreedID: "bianmu", VariantID: "v2"})
	if !strings.Contains(result, "variant 2 prompt") {
		t.Error("expected v2 system prompt")
	}
	if strings.Contains(result, "variant 1 prompt") {
		t.Error("should not contain v1 prompt")
	}
}

func TestBuildSkillRosterEmpty(t *testing.T) {
	t.Parallel()
	b := NewBuilder(map[string]*pack.BreedConfig{}, nil)
	result := b.buildSkillRoster()
	if result != "" {
		t.Errorf("expected empty string for nil skills, got %q", result)
	}
}

func TestBuildSkillRosterNoSkills(t *testing.T) {
	t.Parallel()
	mgr := skills.NewManager("")
	_ = mgr.LoadFromDir() // loads nothing, initializes the map

	breeds := map[string]*pack.BreedConfig{
		"bianmu": {
			ID: "bianmu", Name: "边牧", DisplayName: "边牧",
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1"}},
		},
	}
	b := NewBuilder(breeds, mgr)
	result := b.buildSkillRoster()
	if result != "" {
		t.Errorf("expected empty string for no skills, got %q", result)
	}
}

func TestBuildSkillRosterFormat(t *testing.T) {
	t.Parallel()
	mgr := skills.NewManager("../../packs/default/skills")
	if err := mgr.LoadFromDir(); err != nil {
		t.Skipf("skills dir not available: %v", err)
	}
	all := mgr.All()
	if len(all) == 0 {
		t.Skip("no skills loaded")
	}

	breeds := map[string]*pack.BreedConfig{
		"bianmu": {
			ID: "bianmu", Name: "边牧", DisplayName: "边牧",
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1"}},
		},
	}
	b := NewBuilder(breeds, mgr)
	result := b.buildSkillRoster()

	if !strings.Contains(result, "## 可用技能") {
		t.Error("expected '## 可用技能' header")
	}
	if !strings.Contains(result, "| 技能 |") {
		t.Error("expected table header")
	}
	found := false
	for _, s := range all {
		if strings.Contains(result, s.Name) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one skill name in roster")
	}
}

func TestBuildIncludesSkillRoster(t *testing.T) {
	t.Parallel()
	mgr := skills.NewManager("../../packs/default/skills")
	if err := mgr.LoadFromDir(); err != nil {
		t.Skipf("skills dir not available: %v", err)
	}
	if len(mgr.All()) == 0 {
		t.Skip("no skills loaded")
	}

	breeds := map[string]*pack.BreedConfig{
		"bianmu": {
			ID: "bianmu", Name: "边牧", DisplayName: "边牧",
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1"}},
		},
	}
	b := NewBuilder(breeds, mgr)
	result := b.Build(BuildRequest{BreedID: "bianmu"})

	if !strings.Contains(result, "## 可用技能") {
		t.Error("expected skill roster in Build output")
	}
}

func TestBuildStillInjectsSkillBodiesWithSkillIDs(t *testing.T) {
	t.Parallel()
	mgr := skills.NewManager("../../packs/default/skills")
	if err := mgr.LoadFromDir(); err != nil {
		t.Skipf("skills dir not available: %v", err)
	}
	if len(mgr.All()) == 0 {
		t.Skip("no skills loaded")
	}

	breeds := map[string]*pack.BreedConfig{
		"bianmu": {
			ID: "bianmu", Name: "边牧", DisplayName: "边牧",
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1"}},
		},
	}
	b := NewBuilder(breeds, mgr)
	result := b.Build(BuildRequest{BreedID: "bianmu", SkillIDs: []string{"tdd"}})

	if !strings.Contains(result, "## 可用技能") {
		t.Error("expected skill roster in Build output")
	}
	if !strings.Contains(result, "## 技能") {
		t.Error("expected skill body section for explicit SkillIDs")
	}
}
