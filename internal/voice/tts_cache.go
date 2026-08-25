// Package voice implements the audio layer: TTS segment caching with resume
// position and STT provider access (roadmap P1-B). The TTS cache lets a
// long-generation be resumed from the last cached segment rather than
// re-synthesizing from the start, and supports bounded cache cleanup.
package voice

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// Segment is one cached unit of synthesized speech.
type Segment struct {
	// ID is the stable segment id (usually the text hash).
	ID string
	// Text is the source utterance.
	Text string
	// AudioRef is a reference to the audio bytes (path or store key). The cache
	// is agnostic to the backing store.
	AudioRef string
	// Position is the monotonic offset used for resume (sequence order).
	Position int64
}

// TTSCache is a goroutine-safe, bounded cache of TTS segments keyed by text
// hash. It records resume positions so a client can ask "where did we stop?"
type TTSCache struct {
	mu          sync.Mutex
	segments    map[string]*Segment
	order       []string // insertion order for LRU eviction
	maxSegments int
}

// NewTTSCache creates a cache holding at most maxSegments (0 => default 256).
func NewTTSCache(maxSegments int) *TTSCache {
	if maxSegments <= 0 {
		maxSegments = 256
	}
	return &TTSCache{
		segments:    make(map[string]*Segment),
		maxSegments: maxSegments,
	}
}

// key derives a stable segment id from text.
func key(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Put caches a segment (id derived from text if empty). Newer puts move to the
// most-recent position.
func (c *TTSCache) Put(seg Segment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if seg.ID == "" {
		seg.ID = key(seg.Text)
	}
	if _, ok := c.segments[seg.ID]; ok {
		// refresh recency
		c.removeOrder(seg.ID)
	} else if len(c.segments) >= c.maxSegments {
		c.evictLocked()
	}
	c.segments[seg.ID] = &seg
	c.order = append(c.order, seg.ID)
}

// Get returns a cached segment by text.
func (c *TTSCache) Get(text string) (Segment, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.segments[key(text)]
	if !ok {
		return Segment{}, false
	}
	return *s, true
}

// ResumePosition returns the Position of the last cached segment for text,
// allowing a client to resume synthesis after that offset. ok=false means the
// text was never cached (resume from 0).
func (c *TTSCache) ResumePosition(text string) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.segments[key(text)]
	if !ok {
		return 0, false
	}
	return s.Position, true
}

// Len returns the number of cached segments.
func (c *TTSCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.segments)
}

func (c *TTSCache) removeOrder(id string) {
	for i, v := range c.order {
		if v == id {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

// evictLocked drops the least-recently-used segment. Caller holds mu.
func (c *TTSCache) evictLocked() {
	if len(c.order) == 0 {
		return
	}
	oldest := c.order[0]
	delete(c.segments, oldest)
	c.order = c.order[1:]
}

// Clean evicts segments beyond the cap (defensive; Put already bounds).
func (c *TTSCache) Clean() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.segments) > c.maxSegments {
		c.evictLocked()
	}
}
