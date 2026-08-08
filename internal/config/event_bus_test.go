package config

import (
	"testing"
	"time"
)

func TestEventBus_EmitSubscribe(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe()

	bus.Emit(ConfigEvent{
		Source:      "breed-config",
		Scope:       "domain",
		ChangedKeys: []string{"bianmu"},
		Timestamp:   time.Now(),
	})

	select {
	case evt := <-ch:
		if evt.Source != "breed-config" {
			t.Fatalf("Source = %q, want breed-config", evt.Source)
		}
		if len(evt.ChangedKeys) != 1 || evt.ChangedKeys[0] != "bianmu" {
			t.Fatalf("ChangedKeys = %v, want [bianmu]", evt.ChangedKeys)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewEventBus()
	ch1 := bus.Subscribe()
	ch2 := bus.Subscribe()

	bus.Emit(ConfigEvent{Source: "test"})

	for i, ch := range []<-chan ConfigEvent{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d timed out", i)
		}
	}
}
