// Package collective models the AI-native Collective / Channel object model
// (F290, P1-E long-tail deepening): a Collective is a group of agents sharing
// Channels; members join/leave and channels carry member lists.
package collective

import (
	"fmt"
	"sync"
)

// Member is an agent participating in a collective.
type Member struct {
	AgentID string
	Role    string
}

// Channel is a named communication surface within a collective.
type Channel struct {
	ID      string
	Name    string
	Members []string // agent ids
}

// Collective is the AI-native object model (F290).
type Collective struct {
	ID      string
	Name    string
	members map[string]Member
	channels map[string]*Channel
	mu      sync.RWMutex
}

// New creates an empty collective.
func New(id, name string) *Collective {
	return &Collective{ID: id, Name: name, members: make(map[string]Member), channels: make(map[string]*Channel)}
}

// Join adds a member (idempotent).
func (c *Collective) Join(m Member) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.members[m.AgentID] = m
}

// Leave removes a member and prunes them from all channels.
func (c *Collective) Leave(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.members, agentID)
	for _, ch := range c.channels {
		pruned := ch.Members[:0]
		for _, id := range ch.Members {
			if id != agentID {
				pruned = append(pruned, id)
			}
		}
		ch.Members = pruned
	}
}

// CreateChannel adds a channel with optional initial members.
func (c *Collective) CreateChannel(name string, initialMembers ...string) *Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := &Channel{ID: fmt.Sprintf("ch-%s", name), Name: name, Members: append([]string{}, initialMembers...)}
	c.channels[ch.ID] = ch
	return ch
}

// Members returns the current member list.
func (c *Collective) Members() []Member {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Member, 0, len(c.members))
	for _, m := range c.members {
		out = append(out, m)
	}
	return out
}
