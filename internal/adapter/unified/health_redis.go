package unified

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisHealth is the cross-instance CarrierHealth implementation, mirroring
// the CarrierHealthStore (Redis-backed). It is compiled into the
// default binary; it is only activated at runtime when a Redis URL is
// configured (platform.Config.RedisURL / SG_REDIS_URL). MemoryHealth remains
// the default (zero-dependency) fallback when no URL is set.
type RedisHealth struct {
	client *redis.Client
	prefix string
	now    func() time.Time
}

// NewRedisHealth constructs a Redis-backed carrier health store.
func NewRedisHealth(client *redis.Client) *RedisHealth {
	return &RedisHealth{
		client: client,
		prefix: "sg:carrier-health:",
		now:    time.Now,
	}
}

type redisEntry struct {
	Level       string          `json:"level"`
	Reason      ErrorReasonCode `json:"reason"`
	Until       int64           `json:"until"` // unix ms
	Consecutive int             `json:"consecutive"`
	Escalated   bool            `json:"escalated"`
}

func (h *RedisHealth) key(carrier string) string { return h.prefix + carrier }

func (h *RedisHealth) get(_ context.Context, carrier string) (*redisEntry, error) {
	raw, err := h.client.Get(context.Background(), h.key(carrier)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var e redisEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func (h *RedisHealth) set(_ context.Context, carrier string, e *redisEntry, ttl time.Duration) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return h.client.Set(context.Background(), h.key(carrier), raw, ttl).Err()
}

// RecordFailure classifies reason and persists the degrade state with a TTL
// equal to the cooldown, so Redis auto-expires recovered carriers.
func (h *RedisHealth) RecordFailure(_ context.Context, carrier string, reason ErrorReasonCode) {
	e, _ := h.get(context.Background(), carrier)
	if e == nil {
		e = &redisEntry{}
	}

	tier, ok := reasonTier[reason]
	if !ok {
		tier = TierTransient
	}

	now := h.now()
	var ttl time.Duration
	switch tier {
	case TierQuota:
		e.Level = "offline"
		e.Reason = reason
		e.Until = now.Add(quotaCooldown).UnixMilli()
		e.Consecutive = 0
		e.Escalated = true
		ttl = quotaCooldown
	case TierStructural:
		e.Level = "degraded"
		e.Reason = reason
		e.Until = now.Add(structCooldown).UnixMilli()
		e.Consecutive = 0
		e.Escalated = true
		ttl = structCooldown
	case TierTransient:
		e.Consecutive++
		if e.Consecutive >= transientStrikes {
			e.Level = "degraded"
			e.Reason = reason
			e.Until = now.Add(structCooldown).UnixMilli()
			e.Escalated = true
			ttl = structCooldown
		} else {
			e.Level = "degraded"
			e.Reason = reason
			e.Until = now.Add(transientCooldown).UnixMilli()
			e.Escalated = false
			ttl = transientCooldown
		}
	}
	_ = h.set(context.Background(), carrier, e, ttl)
}

// RecordSuccess clears non-escalated transient degradation; quota/escalated
// persist until TTL (handled by Redis expiry).
func (h *RedisHealth) RecordSuccess(_ context.Context, carrier string) {
	e, err := h.get(context.Background(), carrier)
	if err != nil || e == nil {
		return
	}
	if e.Level == "offline" || e.Escalated {
		return
	}
	_ = h.client.Del(context.Background(), h.key(carrier)).Err()
}

// Level returns the current health level string.
func (h *RedisHealth) Level(ctx context.Context, carrier string) string {
	return h.Info(ctx, carrier).Level
}

// Info returns detailed degrade info, treating missing/expired keys as online.
func (h *RedisHealth) Info(_ context.Context, carrier string) DegradeInfo {
	e, err := h.get(context.Background(), carrier)
	if err != nil || e == nil {
		return DegradeInfo{Level: "online"}
	}
	now := h.now()
	until := time.UnixMilli(e.Until)
	if now.After(until) {
		_ = h.client.Del(context.Background(), h.key(carrier)).Err()
		return DegradeInfo{Level: "online"}
	}
	level := e.Level
	if level == "" {
		level = "degraded"
	}
	return DegradeInfo{
		Level:       level,
		Reason:      e.Reason,
		Until:       until,
		Remaining:   until.Sub(now),
		Consecutive: e.Consecutive,
	}
}
