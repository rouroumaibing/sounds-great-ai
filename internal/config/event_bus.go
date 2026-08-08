package config

import (
	"sync"
	"time"
)

// ConfigEvent represents a configuration change event.
type ConfigEvent struct {
	Source      string    // "breed-config" | "account-config" | "system-config"
	Scope       string    // "domain" | "key"
	ChangedKeys []string  // breed IDs, account IDs, config keys
	Timestamp   time.Time
}

// ConfigEventBus is a pub/sub bus for configuration change events.
type ConfigEventBus struct {
	mu          sync.RWMutex
	subscribers []chan ConfigEvent
}

// NewEventBus creates a new ConfigEventBus.
func NewEventBus() *ConfigEventBus {
	return &ConfigEventBus{}
}

// Subscribe returns a channel that receives all future events.
func (b *ConfigEventBus) Subscribe() <-chan ConfigEvent {
	ch := make(chan ConfigEvent, 16)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers = append(b.subscribers, ch)
	return ch
}

// Emit sends an event to all subscribers (non-blocking, drops on full buffer).
func (b *ConfigEventBus) Emit(event ConfigEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default: // drop if buffer full
		}
	}
}
