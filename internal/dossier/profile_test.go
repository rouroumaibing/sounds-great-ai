package dossier

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleDossier = `# 狗狗能力画像档案

### 边牧 · @边牧 · ` + "`dog:bianmu`" + `

一句话画像：编排与架构双强。

- **① 原生峰值**：任务拆解与动态路由

` + "```yaml" + `
# structured-profile: dog:bianmu
entityId: "bianmu"
oneLiner: "编排与架构双强：复杂任务拆干净、球传对地方"
l0RosterSummary: "任务拆解、路由决策、结果合成"
l0RoutingNote: "编排非 Boss：不替队友做主"
routingSignals:
  peakCapabilities:
    - "复杂任务拆解与动态路由"
    - "多方结果合成"
  antiSignals:
    - "纯检索任务（传金毛）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: 手写人设"
` + "```" + `

### 金毛 · @金毛 · ` + "`dog:jinmao`" + `

` + "```yaml" + `
# structured-profile: dog:jinmao
entityId: "jinmao"
oneLiner: "知识寻回，只交证据不做推断"
l0RosterSummary: "检索、上下文组装"
routingSignals:
  peakCapabilities: ["快速检索", "上下文组装"]
provenance:
  version: "0.2"
  date: "2026-08-23"
` + "```" + `

### 无标记块 · ` + "`dog:ignored`" + `

` + "```yaml" + `
someOther: "block without marker"
` + "```" + `
`

func TestParseDossierProfiles(t *testing.T) {
	profiles := ParseDossierProfiles(sampleDossier)

	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d: %v", len(profiles), profiles)
	}

	bianmu, ok := profiles["bianmu"]
	if !ok {
		t.Fatal("missing bianmu profile")
	}
	if bianmu.OneLiner == "" || bianmu.L0RosterSummary == "" || bianmu.L0RoutingNote == "" {
		t.Errorf("bianmu string fields not parsed: %+v", bianmu)
	}
	if want := []string{"复杂任务拆解与动态路由", "多方结果合成"}; !equalStrings(bianmu.RoutingSignals.PeakCapabilities, want) {
		t.Errorf("peakCapabilities = %v, want %v", bianmu.RoutingSignals.PeakCapabilities, want)
	}
	if want := []string{"纯检索任务（传金毛）"}; !equalStrings(bianmu.RoutingSignals.AntiSignals, want) {
		t.Errorf("antiSignals = %v, want %v", bianmu.RoutingSignals.AntiSignals, want)
	}
	if bianmu.Provenance.Version != "0.1" || bianmu.Provenance.Date != "2026-08-22" {
		t.Errorf("provenance = %+v", bianmu.Provenance)
	}
	if len(bianmu.Provenance.PrimarySources) != 1 || bianmu.Provenance.PrimarySources[0] != "baseline: 手写人设" {
		t.Errorf("primarySources = %v", bianmu.Provenance.PrimarySources)
	}

	// Inline list form + provenance without primarySources.
	jinmao, ok := profiles["jinmao"]
	if !ok {
		t.Fatal("missing jinmao profile")
	}
	if want := []string{"快速检索", "上下文组装"}; !equalStrings(jinmao.RoutingSignals.PeakCapabilities, want) {
		t.Errorf("inline peakCapabilities = %v, want %v", jinmao.RoutingSignals.PeakCapabilities, want)
	}
	if jinmao.Provenance.Version != "0.2" {
		t.Errorf("jinmao provenance version = %q", jinmao.Provenance.Version)
	}
	if jinmao.Provenance.PrimarySources != nil {
		t.Errorf("jinmao primarySources should be nil, got %v", jinmao.Provenance.PrimarySources)
	}
}

func TestParseDossierProfilesEntityIDMismatch(t *testing.T) {
	content := "```yaml\n# structured-profile: dog:bianmu\nentityId: \"jinmao\"\noneLiner: \"x\"\n```"
	profiles := ParseDossierProfiles(content)
	if len(profiles) != 0 {
		t.Errorf("entityId mismatch must fail the block, got %v", profiles)
	}
}

func TestParseDossierProfilesEmpty(t *testing.T) {
	if got := ParseDossierProfiles(""); got == nil || len(got) != 0 {
		t.Errorf("empty content should give empty non-nil map, got %v", got)
	}
}

// The real repo dossier is the canonical format contract — parse it and
// assert every shipped dog identity is present.
func TestParseShippedDossier(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", DossierRelativePath))
	if err != nil {
		t.Skipf("shipped dossier not found: %v", err)
	}
	profiles := ParseDossierProfiles(string(content))

	required := []string{
		"bianmu", "jinmao", "xigou", "demu", "zangao", "zhonghuatianyuanquan",
		"bianmu-sonnet", "bianmu-codex", "jinmao-flash", "jinmao-opencode",
		"xigou-claude", "demu-claude", "zangao-sonnet", "zhonghuatianyuanquan-spark",
	}
	for _, id := range required {
		p, ok := profiles[id]
		if !ok {
			t.Errorf("shipped dossier missing profile for %s", id)
			continue
		}
		if p.OneLiner == "" || p.L0RosterSummary == "" || p.L0RoutingNote == "" {
			t.Errorf("profile %s missing required projections: %+v", id, p)
		}
		if p.Provenance.Version == "" || p.Provenance.Date == "" {
			t.Errorf("profile %s missing provenance: %+v", id, p.Provenance)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
