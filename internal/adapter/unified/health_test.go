package unified

import (
	"context"
	"testing"
	"time"
)

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestMemoryHealth_TransientClearedBySuccess(t *testing.T) {
	base := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	h := NewMemoryHealthWithClock(fixedClock(base))
	ctx := context.Background()

	h.RecordFailure(ctx, "claude", ReasonNetworkError)
	if lvl := h.Level(ctx, "claude"); lvl != "degraded" {
		t.Fatalf("after 1 transient: want degraded, got %s", lvl)
	}
	info := h.Info(ctx, "claude")
	if info.Remaining > transientCooldown || info.Remaining <= 0 {
		t.Fatalf("transient remaining off: %v", info.Remaining)
	}

	// a later success clears non-escalated transient degradation
	h.RecordSuccess(ctx, "claude")
	if lvl := h.Level(ctx, "claude"); lvl != "online" {
		t.Fatalf("after success: want online, got %s", lvl)
	}
}

func TestMemoryHealth_TransientEscalatesAfterStrikes(t *testing.T) {
	base := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	h := NewMemoryHealthWithClock(fixedClock(base))
	ctx := context.Background()

	// 3 consecutive transient failures → escalated to structural (30min)
	h.RecordFailure(ctx, "codex", ReasonServerOverloaded)
	h.RecordFailure(ctx, "codex", ReasonServerOverloaded)
	h.RecordFailure(ctx, "codex", ReasonServerOverloaded)
	info := h.Info(ctx, "codex")
	if info.Remaining > structCooldown || info.Remaining <= transientCooldown {
		t.Fatalf("escalated remaining off: %v", info.Remaining)
	}

	// escalated (structural-like) is NOT cleared by success
	h.RecordSuccess(ctx, "codex")
	if lvl := h.Level(ctx, "codex"); lvl != "degraded" {
		t.Fatalf("escalated should persist: got %s", lvl)
	}
}

func TestMemoryHealth_StructuralTTL(t *testing.T) {
	base := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	h := NewMemoryHealthWithClock(fixedClock(base))
	ctx := context.Background()

	h.RecordFailure(ctx, "gemini", ReasonAuthFailed)
	info := h.Info(ctx, "gemini")
	if info.Level != "degraded" || info.Remaining > structCooldown {
		t.Fatalf("structural off: %+v", info)
	}
	// TTL-governed: success does not clear
	h.RecordSuccess(ctx, "gemini")
	if lvl := h.Level(ctx, "gemini"); lvl != "degraded" {
		t.Fatalf("structural should persist: got %s", lvl)
	}
}

func TestMemoryHealth_QuotaOfflineNotCleared(t *testing.T) {
	base := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	h := NewMemoryHealthWithClock(fixedClock(base))
	ctx := context.Background()

	h.RecordFailure(ctx, "kimi", ReasonQuotaExceeded)
	info := h.Info(ctx, "kimi")
	if info.Level != "offline" || info.Remaining > quotaCooldown {
		t.Fatalf("quota off: %+v", info)
	}
	h.RecordSuccess(ctx, "kimi")
	if lvl := h.Level(ctx, "kimi"); lvl != "offline" {
		t.Fatalf("quota should persist: got %s", lvl)
	}
}

func TestMemoryHealth_RecoversAfterTTL(t *testing.T) {
	base := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	h := NewMemoryHealthWithClock(fixedClock(base))
	ctx := context.Background()

	h.RecordFailure(ctx, "opencode", ReasonNetworkError)
	// advance clock past the transient cooldown
	h.now = fixedClock(base.Add(transientCooldown + time.Second))
	if lvl := h.Level(ctx, "opencode"); lvl != "online" {
		t.Fatalf("should recover after TTL: got %s", lvl)
	}
}

func TestMemoryHealth_UnknownReasonDefaultsTransient(t *testing.T) {
	base := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	h := NewMemoryHealthWithClock(fixedClock(base))
	ctx := context.Background()

	h.RecordFailure(ctx, "x", ErrorReasonCode("totally_unknown"))
	info := h.Info(ctx, "x")
	if info.Level != "degraded" || info.Remaining > transientCooldown {
		t.Fatalf("unknown reason should degrade transiently: %+v", info)
	}
}
