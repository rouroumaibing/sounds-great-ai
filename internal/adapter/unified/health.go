package unified

import (
	"context"
	"sync"
	"time"
)

// HealthTier classifies a failure's severity for cooldown purposes, mirroring
// the CarrierHealthStore (quota 4h / structural 30min / transient 3-strikes).
type HealthTier int

const (
	// TierTransient: network/overload/timeout. Short cooldown; escalates to
	// structural after N consecutive strikes. Cleared by a later success.
	TierTransient HealthTier = iota
	// TierStructural: config/model/auth. Medium fixed cooldown (TTL-governed).
	TierStructural
	// TierQuota: billing. Long cooldown; never cleared by success (TTL-governed).
	TierQuota
)

// DegradeInfo describes the current health state of a carrier.
type DegradeInfo struct {
	Level       string          // "online" | "degraded" | "offline"
	Reason      ErrorReasonCode // last classified reason (empty when online)
	Until       time.Time       // when the cooldown expires
	Remaining   time.Duration   // time left until recovery (0 when online)
	Consecutive int             // consecutive transient strikes (diagnostics only)
}

// CarrierHealth records per-carrier failure state and answers degrade queries.
// It is the R6 building block: callers RecordFailure with a classified
// ErrorReasonCode; Level/Info drive the R1 multi-carrier fallback chain.
type CarrierHealth interface {
	// RecordFailure notes a failure for carrier with the given classified reason.
	RecordFailure(ctx context.Context, carrier string, reason ErrorReasonCode)
	// RecordSuccess clears transient/structural degradation for carrier.
	// Quota (billing) degradation is TTL-governed and NOT cleared by success.
	RecordSuccess(ctx context.Context, carrier string)
	// Level returns the current health level: "online", "degraded", or "offline".
	Level(ctx context.Context, carrier string) string
	// Info returns detailed degrade info (online with zero remaining when healthy).
	Info(ctx context.Context, carrier string) DegradeInfo
}

const (
	quotaCooldown     = 4 * time.Hour
	structCooldown    = 30 * time.Minute
	transientCooldown = 3 * time.Minute
	transientStrikes  = 3 // consecutive transient failures → behave like structural
)

// reasonTier maps classified reasons to their cooldown tier. Unknown reasons
// degrade gracefully to transient (short, self-healing).
var reasonTier = map[ErrorReasonCode]HealthTier{
	ReasonQuotaExceeded:         TierQuota,
	ReasonAuthFailed:            TierStructural,
	ReasonInvalidConfig:         TierStructural,
	ReasonModelNotFound:         TierStructural,
	ReasonNetworkError:          TierTransient,
	ReasonServerOverloaded:      TierTransient,
	ReasonResponseTimeout:       TierTransient,
	ReasonStallTimeout:          TierTransient,
	ReasonUpstreamPolicyReject:  TierTransient,
}

type healthEntry struct {
	level       string
	reason      ErrorReasonCode
	until       time.Time
	consecutive int
	escalated   bool // transient strikes reached threshold → structural TTL
}

// MemoryHealth is the default, dependency-free CarrierHealth implementation.
// It is safe for concurrent use and sufficient for single-instance deployments.
// Production can swap in RedisHealth (build tag redis_health) for cross-instance
// shared health state, per ADR-002.
type MemoryHealth struct {
	mu      sync.Mutex
	entries map[string]*healthEntry
	now     func() time.Time // injectable clock for tests
}

// NewMemoryHealth constructs an in-memory carrier health store.
func NewMemoryHealth() *MemoryHealth {
	return &MemoryHealth{
		entries: make(map[string]*healthEntry),
		now:     time.Now,
	}
}

// NewMemoryHealthWithClock is like NewMemoryHealth but uses a fixed clock (tests).
func NewMemoryHealthWithClock(now func() time.Time) *MemoryHealth {
	return &MemoryHealth{
		entries: make(map[string]*healthEntry),
		now:     now,
	}
}

// RecordFailure classifies reason, updates the carrier's degrade tier, and
// escalates transient failures after transientStrikes consecutive hits.
func (h *MemoryHealth) RecordFailure(_ context.Context, carrier string, reason ErrorReasonCode) {
	h.mu.Lock()
	defer h.mu.Unlock()

	e, ok := h.entries[carrier]
	if !ok {
		e = &healthEntry{}
		h.entries[carrier] = e
	}

	tier, ok := reasonTier[reason]
	if !ok {
		tier = TierTransient
	}

	switch tier {
	case TierQuota:
		e.level = "offline"
		e.reason = reason
		e.until = h.now().Add(quotaCooldown)
		e.consecutive = 0
		e.escalated = true
	case TierStructural:
		e.level = "degraded"
		e.reason = reason
		e.until = h.now().Add(structCooldown)
		e.consecutive = 0
		e.escalated = true
	case TierTransient:
		e.consecutive++
		if e.consecutive >= transientStrikes {
			e.level = "degraded"
			e.reason = reason
			e.until = h.now().Add(structCooldown)
			e.escalated = true
		} else {
			e.level = "degraded"
			e.reason = reason
			e.until = h.now().Add(transientCooldown)
			e.escalated = false
		}
	}
}

// RecordSuccess clears transient degradation (non-escalated). Quota and
// escalated-structural degradation are TTL-governed and persist.
func (h *MemoryHealth) RecordSuccess(_ context.Context, carrier string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	e, ok := h.entries[carrier]
	if !ok {
		return
	}
	if e.level == "offline" || e.escalated {
		return
	}
	delete(h.entries, carrier)
}

// Level returns the current health level string.
func (h *MemoryHealth) Level(ctx context.Context, carrier string) string {
	return h.Info(ctx, carrier).Level
}

// Info returns detailed degrade info, automatically recovering expired entries.
func (h *MemoryHealth) Info(_ context.Context, carrier string) DegradeInfo {
	h.mu.Lock()
	defer h.mu.Unlock()

	e, ok := h.entries[carrier]
	if !ok {
		return DegradeInfo{Level: "online"}
	}
	now := h.now()
	if now.After(e.until) {
		delete(h.entries, carrier)
		return DegradeInfo{Level: "online"}
	}
	level := e.level
	if level == "" {
		level = "degraded"
	}
	return DegradeInfo{
		Level:       level,
		Reason:      e.reason,
		Until:       e.until,
		Remaining:   e.until.Sub(now),
		Consecutive: e.consecutive,
	}
}
