// internal/ragstore/cleaner.go
package ragstore

import (
	"context"
	"log"
	"sync"
	"time"
)

type RetiredCleaner struct {
	registry *StoreRegistry
	interval time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once
}

func NewRetiredCleaner(registry *StoreRegistry, interval time.Duration) *RetiredCleaner {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &RetiredCleaner{
		registry: registry,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (c *RetiredCleaner) Start() {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.runOnce(context.Background())
			case <-c.stopCh:
				return
			}
		}
	}()
}

func (c *RetiredCleaner) runOnce(ctx context.Context) {
	now := time.Now()
	retirees := c.registry.Retirees()
	for _, info := range retirees {
		if info.RetireAt.After(now) {
			continue
		}
		oldStore, err := c.registry.GetRetired(info.Backend)
		if err != nil {
			continue
		}
		if err := oldStore.DropAll(ctx); err != nil {
			log.Printf("cleaner: drop all failed for %s: %v", info.Backend, err)
			continue
		}
		c.registry.RemoveRetired(info.Backend)
	}
}

func (c *RetiredCleaner) Stop() {
	c.stopOnce.Do(func() { close(c.stopCh) })
}
