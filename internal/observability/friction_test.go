package observability

import "testing"

func TestFrictionAggregator_Dedup(t *testing.T) {
	a := NewFrictionAggregator()
	if !a.Ingest(FrictionSignal{ID: "1", Channel: "cli", Key: "stall:thread-12", Weight: 1}) {
		t.Fatal("first ingest should be accepted")
	}
	// Duplicate key from a different channel must be deduped.
	if a.Ingest(FrictionSignal{ID: "2", Channel: "ui", Key: "stall:thread-12", Weight: 1}) {
		t.Fatal("duplicate key must be deduped across channels")
	}
	if got := a.Rollup(); len(got) != 1 || got[0].Count != 1 {
		t.Fatalf("rollup wrong after dedup: %+v", got)
	}
}

func TestFrictionAggregator_RollupByChannel(t *testing.T) {
	a := NewFrictionAggregator()
	a.Ingest(FrictionSignal{Channel: "cli", Key: "a", Weight: 2})
	a.Ingest(FrictionSignal{Channel: "ui", Key: "b", Weight: 3})
	a.Ingest(FrictionSignal{Channel: "cli", Key: "c", Weight: 1})
	got := a.Rollup()
	if len(got) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(got))
	}
	// cli: count 2 weight 3; ui: count 1 weight 3
	for _, r := range got {
		switch r.Channel {
		case "cli":
			if r.Count != 2 || r.Weight != 3 {
				t.Fatalf("cli rollup wrong: %+v", r)
			}
		case "ui":
			if r.Count != 1 || r.Weight != 3 {
				t.Fatalf("ui rollup wrong: %+v", r)
			}
		}
	}
}
