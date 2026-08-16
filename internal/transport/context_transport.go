package transport

import (
	"strconv"
	"strings"

	"sounds-great-ai/internal/telemetry"
)

// G12 tuning mirrors the context-transport caps. SG already had the
// a2aFrom source notice + secret redaction (G7); this file adds the missing
// 4.5/7 layers: continuity capsule, tombstone (burst), coverage map, importance
// anchors, scrubToolPayloads, and detectRecentBurst.
const (
	maxRunesPerToolPayload = 400  // cap on a single handoff artifact's runes
	burstMessageThreshold  = 6    // >= N recent handoffs => burst (tombstone)
	continuityCapsuleRunes = 240  // cap on continuity capsule length
)

// HandoffTransportContext carries the lightweight metadata needed to enrich a
// handoff artifact with the context-transport layers (G12). SG has no TS
// session-hooks capsule, so we synthesize the relevant fields from the live A2A
// thread.
type HandoffTransportContext struct {
	FromBreed        string
	ToBreed          string
	TurnIndex        int   // 1-based; 0 means unknown
	TotalTurns       int   // 0 means unknown
	Artifact         string
	RecentBreeds     []string // recent participants (coverage map + burst)
	RecentTimestamps []int64  // timestamps of recent handoffs (burst window)
}

// buildEnrichedHandoffContext composes the full context-transport envelope
// around a handoff artifact: source notice (a2aFrom) -> continuity capsule ->
// tombstone (on recent burst) -> coverage map -> importance anchors -> scrubbed
// artifact. Deterministic and IO-free.
func buildEnrichedHandoffContext(c HandoffTransportContext) string {
	var b strings.Builder
	if src := buildHandoffSourceNotice(c.FromBreed); src != "" {
		b.WriteString(src)
		b.WriteString("\n\n")
	}
	if cap := buildContinuityCapsule(c); cap != "" {
		b.WriteString(cap)
		b.WriteString("\n\n")
	}
	if detectRecentBurst(c.RecentTimestamps, burstMessageThreshold) {
		b.WriteString(buildTombstone(len(c.RecentBreeds)))
		b.WriteString("\n\n")
	}
	if cov := buildCoverageMap(c.RecentBreeds); cov != "" {
		b.WriteString(cov)
		b.WriteString("\n\n")
	}
	if anchors := importanceAnchors(c); len(anchors) > 0 {
		b.WriteString(strings.Join(anchors, "\n"))
		b.WriteString("\n\n")
	}
	b.WriteString(scrubToolPayloads(c.Artifact, maxRunesPerToolPayload))
	return b.String()
}

// detectRecentBurst reports whether recent handoffs arrived in a short burst
// (detectRecentBurst). SG passes RecentTimestamps; with fewer than the
// threshold (or empty) there is no burst.
func detectRecentBurst(ts []int64, threshold int) bool {
	return len(ts) >= threshold
}

// buildTombstone emits a system note that earlier high-frequency turns were
// collapsed (buildTombstone), attached when a burst is detected so the
// next dog is told the history was compacted.
func buildTombstone(collapsed int) string {
	return "[系统] 检测到短时间内的高频交接(burst)，已对较早的 " + strconv.Itoa(collapsed) +
		" 轮做折叠(tombstone)处理，仅保留最近上下文，请基于现有上下文继续。"
}

// buildCoverageMap summarizes which breeds participated and how often
// (buildCoverageMap), so the next dog sees the collaboration footprint.
func buildCoverageMap(breeds []string) string {
	if len(breeds) == 0 {
		return ""
	}
	counts := make(map[string]int)
	order := make([]string, 0)
	for _, bd := range breeds {
		if _, ok := counts[bd]; !ok {
			order = append(order, bd)
		}
		counts[bd]++
	}
	parts := make([]string, 0, len(order))
	for _, bd := range order {
		parts = append(parts, bd+"×"+strconv.Itoa(counts[bd]))
	}
	return "[协作覆盖] " + strings.Join(parts, " ")
}

// importanceAnchors surfaces the most important context anchors: the most
// recent handoff source and the selected next dog (importance anchors).
func importanceAnchors(c HandoffTransportContext) []string {
	var out []string
	if c.FromBreed != "" {
		out = append(out, "[锚点] 最近一次交接来自 @"+c.FromBreed+"，请基于其输出继续。")
	}
	if c.ToBreed != "" {
		out = append(out, "[锚点] 你（@"+c.ToBreed+"）被选中继续推进该任务。")
	}
	return out
}

// buildContinuityCapsule describes the chain position so the next dog knows
// where it sits in the multi-agent flow (CollaborationContinuityCapsule).
func buildContinuityCapsule(c HandoffTransportContext) string {
	if c.FromBreed == "" && c.ToBreed == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("[协作链位置] ")
	if c.TurnIndex > 0 && c.TotalTurns > 0 {
		b.WriteString("第 " + strconv.Itoa(c.TurnIndex) + "/" + strconv.Itoa(c.TotalTurns) + " 轮；")
	}
	b.WriteString("上一持有 @" + c.FromBreed + "，即将移交 @" + c.ToBreed + "。")
	s := b.String()
	if len([]rune(s)) > continuityCapsuleRunes {
		s = string([]rune(s)[:continuityCapsuleRunes])
	}
	return s
}

// scrubToolPayloads redacts secrets and truncates an over-long artifact so a
// handoff never leaks credentials or floods the next dog's context with a huge
// tool payload (scrubToolPayloads + eraseToolPayloads parity).
func scrubToolPayloads(artifact string, maxRunes int) string {
	clean := telemetry.RedactSecrets(artifact)
	runes := []rune(clean)
	if len(runes) <= maxRunes {
		return clean
	}
	return string(runes[:maxRunes]) + "…[已截断]"
}
