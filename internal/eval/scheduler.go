package eval

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Scheduler runs eval domains on a daily/N-day schedule.
type Scheduler struct {
	runner  *EvalRunner
	domains []EvalDomain
	redis   *redis.Client
	stop    chan struct{}
}

// NewScheduler creates a Scheduler.
func NewScheduler(runner *EvalRunner, domains []EvalDomain, rdb *redis.Client) *Scheduler {
	return &Scheduler{
		runner:  runner,
		domains: domains,
		redis:   rdb,
		stop:    make(chan struct{}),
	}
}

// Start begins the daily tick loop.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	// Run once immediately on start
	s.tick(ctx)
	for {
		select {
		case <-ticker.C:
			s.tick(ctx)
		case <-s.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) tick(ctx context.Context) {
	for _, domain := range s.domains {
		if !s.isDue(ctx, domain) {
			continue
		}
		s.runDomain(ctx, domain)
	}
}

// runDomain executes one domain. Only writes last-dispatch on success (先 trigger 后写).
func (s *Scheduler) runDomain(ctx context.Context, domain EvalDomain) {
	effective := s.effectiveBreed(ctx, domain)
	adjustedDomain := domain
	adjustedDomain.EvalBreed = effective

	verdict, err := s.runner.RunDomain(ctx, adjustedDomain)
	if err != nil {
		log.Printf("eval scheduler: domain %s failed: %v", domain.DomainID, err)
		return // 失败不写 last-dispatch，下次重试
	}
	if s.redis != nil {
		key := fmt.Sprintf("eval-nday-last-dispatch:%s", domain.DomainID)
		// best-effort: 写失败不影响 eval 结果
		if err := s.redis.Set(ctx, key, strconv.FormatInt(time.Now().UnixMilli(), 10), 0).Err(); err != nil {
			log.Printf("eval scheduler: write last-dispatch failed for %s: %v", domain.DomainID, err)
		}
	}
	log.Printf("eval scheduler: domain %s completed, verdict %s", domain.DomainID, verdict.ID)
}

// isDue checks if a domain should run. Fail-open: Redis unavailable → always true.
func (s *Scheduler) isDue(ctx context.Context, domain EvalDomain) bool {
	if s.redis == nil {
		return true // fail-open
	}
	key := fmt.Sprintf("eval-nday-last-dispatch:%s", domain.DomainID)
	raw, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return true // Redis 错误 → fail-open
	}
	lastMs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return true // 非数字 → fail-open
	}
	nDaysMs := int64(24 * 60 * 60 * 1000) // daily default
	if domain.Frequency == "weekly" {
		nDaysMs = 7 * nDaysMs
	}
	jitterMs := int64(2 * 60 * 1000) // 2 分钟 jitter
	return time.Now().UnixMilli()-lastMs >= nDaysMs-jitterMs
}

// effectiveBreed returns the runtime override breed, or YAML default (静默降级).
func (s *Scheduler) effectiveBreed(ctx context.Context, domain EvalDomain) string {
	if s.redis == nil {
		return domain.EvalBreed
	}
	key := fmt.Sprintf("eval:cat-override:%s", domain.DomainID)
	override, err := s.redis.HGet(ctx, key, "breedId").Result()
	if err != nil || override == "" {
		return domain.EvalBreed // 静默降级
	}
	return override
}

// TriggerNow manually triggers a domain by ID.
func (s *Scheduler) TriggerNow(ctx context.Context, domainID string) error {
	for _, d := range s.domains {
		if d.DomainID == domainID {
			s.runDomain(ctx, d)
			return nil
		}
	}
	return fmt.Errorf("domain %q not found", domainID)
}
