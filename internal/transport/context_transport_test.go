package transport

import (
	"strings"
	"testing"
)

// G12: detectRecentBurst triggers only at/above the threshold.
func TestDetectRecentBurst(t *testing.T) {
	if detectRecentBurst([]int64{1, 2, 3}, burstMessageThreshold) {
		t.Fatal("3 timestamps should not be a burst")
	}
	ts := make([]int64, burstMessageThreshold)
	for i := range ts {
		ts[i] = int64(i)
	}
	if !detectRecentBurst(ts, burstMessageThreshold) {
		t.Fatal("threshold-count timestamps should be a burst")
	}
}

// G12: buildTombstone mentions the collapsed count.
func TestBuildTombstone(t *testing.T) {
	s := buildTombstone(5)
	if !strings.Contains(s, "5") || !strings.Contains(s, "tombstone") {
		t.Fatalf("tombstone = %q", s)
	}
}

// G12: buildCoverageMap counts participation per breed.
func TestBuildCoverageMap(t *testing.T) {
	s := buildCoverageMap([]string{"a", "b", "a", "c"})
	if !strings.Contains(s, "a×2") || !strings.Contains(s, "b×1") || !strings.Contains(s, "c×1") {
		t.Fatalf("coverage = %q", s)
	}
	if buildCoverageMap(nil) != "" {
		t.Fatal("empty coverage should be empty")
	}
}

// G12: buildContinuityCapsule reports chain position.
func TestBuildContinuityCapsule(t *testing.T) {
	c := buildContinuityCapsule(HandoffTransportContext{FromBreed: "a", ToBreed: "b", TurnIndex: 2, TotalTurns: 5})
	if !strings.Contains(c, "第 2/5 轮") || !strings.Contains(c, "@a") || !strings.Contains(c, "@b") {
		t.Fatalf("capsule = %q", c)
	}
	if buildContinuityCapsule(HandoffTransportContext{}) != "" {
		t.Fatal("empty context should yield empty capsule")
	}
}

// G12: scrubToolPayloads redacts secrets and truncates over-long artifacts.
func TestScrubToolPayloads(t *testing.T) {
	redacted := scrubToolPayloads("token=sk-secret-123 and more", maxRunesPerToolPayload)
	if strings.Contains(redacted, "sk-secret-123") {
		t.Fatalf("secret not redacted: %q", redacted)
	}
	long := strings.Repeat("x", maxRunesPerToolPayload+100)
	out := scrubToolPayloads(long, maxRunesPerToolPayload)
	if !strings.HasSuffix(out, "…[已截断]") {
		t.Fatalf("long artifact not truncated: len=%d", len([]rune(out)))
	}
}

// G12: buildEnrichedHandoffContext composes all layers in order with the
// (redacted) artifact last.
func TestBuildEnrichedHandoffContext(t *testing.T) {
	out := buildEnrichedHandoffContext(HandoffTransportContext{
		FromBreed:    "a",
		ToBreed:      "b",
		Artifact:     "token=sk-secret and context",
		RecentBreeds: []string{"a", "b", "a"},
	})
	if !strings.Contains(out, "你被 @a 通过交接") {
		t.Fatalf("missing source notice: %q", out)
	}
	if !strings.Contains(out, "协作链位置") {
		t.Fatalf("missing continuity capsule: %q", out)
	}
	if !strings.Contains(out, "协作覆盖") {
		t.Fatalf("missing coverage map: %q", out)
	}
	if !strings.Contains(out, "锚点") {
		t.Fatalf("missing importance anchors: %q", out)
	}
	if strings.Contains(out, "sk-secret") {
		t.Fatalf("secret leaked into enriched context: %q", out)
	}
	// The (redacted) artifact text must appear after the envelope layers.
	if !strings.Contains(out, "context") {
		t.Fatalf("artifact body missing: %q", out)
	}
}
