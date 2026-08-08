package hooks

import "testing"

func TestLeaderRefResolverSkip(t *testing.T) {
	r := &LeaderRefResolver{}
	result := r.Resolve(&AssemblerInput{})
	if result.Status != "skipped" {
		t.Errorf("Status = %q, want %q", result.Status, "skipped")
	}
}

func TestLeaderRefResolverFired(t *testing.T) {
	r := &LeaderRefResolver{}
	result := r.Resolve(&AssemblerInput{
		LeaderName:         "You",
		LeaderHandles:      "`@leader` / `@owner`",
		LeaderFirstMention: "@leader",
	})
	if result.Status != "fired" {
		t.Errorf("Status = %q, want %q", result.Status, "fired")
	}
	if result.Vars["LeaderName"] != "You" {
		t.Errorf("LeaderName = %q, want You", result.Vars["LeaderName"])
	}
	if result.Vars["LeaderFirstMention"] != "@leader" {
		t.Errorf("LeaderFirstMention = %q, want @leader", result.Vars["LeaderFirstMention"])
	}
}

func TestLeaderRefResolverDefaultMention(t *testing.T) {
	r := &LeaderRefResolver{}
	result := r.Resolve(&AssemblerInput{
		LeaderName: "You",
	})
	if result.Vars["LeaderFirstMention"] != "@leader" {
		t.Errorf("LeaderFirstMention = %q, want @leader", result.Vars["LeaderFirstMention"])
	}
}
