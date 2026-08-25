package collective

import "testing"

func TestCollective_JoinLeaveChannelPrune(t *testing.T) {
	c := New("col-1", "pack")
	c.Join(Member{AgentID: "bianmu", Role: "lead"})
	c.Join(Member{AgentID: "jinmao", Role: "retriever"})
	ch := c.CreateChannel("general", "bianmu", "jinmao")
	if len(c.Members()) != 2 {
		t.Fatalf("expected 2 members, got %d", len(c.Members()))
	}
	if len(ch.Members) != 2 {
		t.Fatalf("channel should have 2 members, got %d", len(ch.Members))
	}
	// Leaving prunes from the channel.
	c.Leave("jinmao")
	if len(c.Members()) != 1 {
		t.Fatalf("expected 1 member after leave, got %d", len(c.Members()))
	}
	for _, id := range ch.Members {
		if id == "jinmao" {
			t.Fatal("left member must be pruned from channel")
		}
	}
}
