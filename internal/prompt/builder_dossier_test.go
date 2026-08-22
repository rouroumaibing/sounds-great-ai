package prompt

import (
	"strings"
	"testing"

	"sounds-great-ai/pkg/pack"
)

type stubDossierReader struct {
	oneLiner    map[string]string
	rosterSum   map[string]string
	routingNote map[string]string
}

func (s *stubDossierReader) OneLiner(dogID string) (string, bool) {
	v, ok := s.oneLiner[dogID]
	return v, ok
}
func (s *stubDossierReader) RosterSummary(dogID string) (string, bool) {
	v, ok := s.rosterSum[dogID]
	return v, ok
}
func (s *stubDossierReader) RoutingNote(dogID string) (string, bool) {
	v, ok := s.routingNote[dogID]
	return v, ok
}

func dossierTestBreeds() map[string]*pack.BreedConfig {
	return map[string]*pack.BreedConfig{
		"bianmu": {
			ID:               "bianmu",
			Name:             "边牧",
			DisplayName:      "边牧",
			Personality:      "聪明有主见",
			RoleDescription:  "编排者",
			TeamStrengths:    "任务拆解、路由",
			DogID:            "bianmu",
			Caution:          "不替队友做主",
			Restrictions:     []string{"禁止硬编码流程"},
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1"}},
		},
		"jinmao": {
			ID:               "jinmao",
			Name:             "金毛",
			DisplayName:      "金毛",
			RoleDescription:  "检索",
			TeamStrengths:    "检索、上下文",
			DogID:            "jinmao",
			DefaultVariantID: "v1",
			Variants:         []pack.Variant{{ID: "v1"}},
		},
	}
}

func TestBuilderIdentityPrefersDossierOneLiner(t *testing.T) {
	t.Parallel()
	breeds := dossierTestBreeds()
	b := NewBuilder(breeds, nil)
	b.SetDossier(&stubDossierReader{
		oneLiner: map[string]string{"bianmu": "档案一句话画像（证据版）"},
	})
	result := b.Build(BuildRequest{BreedID: "bianmu"})

	if !strings.Contains(result, "**擅长：** 档案一句话画像（证据版）") {
		t.Errorf("identity 擅长 line should use dossier one-liner, got:\n%s", result)
	}
	if strings.Contains(result, "**擅长：** 任务拆解、路由") {
		t.Errorf("config team_strengths should be replaced when dossier has one-liner")
	}
}

func TestBuilderIdentityFallsBackWithoutDossier(t *testing.T) {
	t.Parallel()
	result := NewBuilder(dossierTestBreeds(), nil).Build(BuildRequest{BreedID: "bianmu"})
	if !strings.Contains(result, "**擅长：** 任务拆解、路由") {
		t.Errorf("without dossier reader, config team_strengths must be used")
	}
}

func TestBuilderRosterDossierColumns(t *testing.T) {
	t.Parallel()
	breeds := dossierTestBreeds()
	b := NewBuilder(breeds, nil)
	b.SetDossier(&stubDossierReader{
		rosterSum:   map[string]string{"jinmao": "档案摘要（检索与组装，证据版）"},
		routingNote: map[string]string{"jinmao": "档案路由边界：只检索不推理"},
	})
	result := b.Build(BuildRequest{BreedID: "bianmu"})
	roster := result[strings.Index(result, "## 队友名册"):]

	if !strings.Contains(roster, "| 狗狗 | @mention | 擅长 | 路由边界 |") {
		t.Errorf("roster header must use 路由边界 column, got:\n%s", roster)
	}
	if !strings.Contains(roster, "档案摘要（检索与组装，证据版）") {
		t.Errorf("roster strengths must prefer dossier summary:\n%s", roster)
	}
	if !strings.Contains(roster, "档案路由边界：只检索不推理") {
		t.Errorf("roster boundary must prefer dossier routing note:\n%s", roster)
	}
}

func TestBuilderRosterFallbackAndRestrictions(t *testing.T) {
	t.Parallel()
	// No dossier entry for jinmao: config fallbacks apply, restrictions stay.
	result := NewBuilder(dossierTestBreeds(), nil).Build(BuildRequest{BreedID: "jinmao"})
	roster := result[strings.Index(result, "## 队友名册"):]

	if !strings.Contains(roster, "任务拆解、路由") {
		t.Errorf("config team_strengths fallback missing:\n%s", roster)
	}
	if !strings.Contains(roster, "不替队友做主") {
		t.Errorf("config caution fallback missing:\n%s", roster)
	}
	if !strings.Contains(roster, "硬限制：禁止硬编码流程") {
		t.Errorf("hard restrictions must always render:\n%s", roster)
	}
}

func TestBuilderRosterEmptyBoundaryRendersDash(t *testing.T) {
	t.Parallel()
	breeds := map[string]*pack.BreedConfig{
		"a": {ID: "a", DisplayName: "A", DefaultVariantID: "v", Variants: []pack.Variant{{ID: "v"}}},
		"b": {ID: "b", DisplayName: "B", DefaultVariantID: "v", Variants: []pack.Variant{{ID: "v"}}},
	}
	result := NewBuilder(breeds, nil).Build(BuildRequest{BreedID: "a"})
	roster := result[strings.Index(result, "## 队友名册"):]
	lines := strings.Split(strings.TrimSpace(roster), "\n")
	var dataRows []string
	for _, l := range lines {
		if strings.HasPrefix(l, "| B ") {
			dataRows = append(dataRows, l)
		}
	}
	if len(dataRows) != 1 {
		t.Fatalf("expected one B row, got %v", dataRows)
	}
	if !strings.HasSuffix(strings.TrimSpace(dataRows[0]), "| — |") {
		t.Errorf("empty boundary must render as dash: %q", dataRows[0])
	}
}
