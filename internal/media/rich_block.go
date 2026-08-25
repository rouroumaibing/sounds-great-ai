package media

import (
	"sync"
	"time"
)

// BlockType classifies a rich block.
type BlockType string

const (
	// BlockText is plain/rich text.
	BlockText BlockType = "text"
	// BlockImage references an uploaded image resource.
	BlockImage BlockType = "image"
	// BlockAudio references an uploaded audio resource.
	BlockAudio BlockType = "audio"
	// BlockVideo references an uploaded video resource.
	BlockVideo BlockType = "video"
)

// RichBlock is a single structured content unit archived alongside messages
// (roadmap P1-B rich_block 归档). A media block points at a Resource by id.
type RichBlock struct {
	ID         string
	Type       BlockType
	Text       string // populated for BlockText
	ResourceID string // populated for media blocks
	CreatedAt  time.Time
}

// Archive stores rich blocks in creation order. Goroutine-safe.
type Archive struct {
	mu     sync.Mutex
	blocks map[string]RichBlock
	order  []string
}

// NewArchive creates an empty archive.
func NewArchive() *Archive {
	return &Archive{blocks: make(map[string]RichBlock)}
}

// Append adds a block (id required).
func (a *Archive) Append(b RichBlock) {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.blocks[b.ID]; !ok {
		a.order = append(a.order, b.ID)
	}
	a.blocks[b.ID] = b
}

// Get returns a block by id (zero, false if absent).
func (a *Archive) Get(id string) (RichBlock, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := a.blocks[id]
	return b, ok
}

// List returns blocks in insertion order.
func (a *Archive) List() []RichBlock {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]RichBlock, 0, len(a.order))
	for _, id := range a.order {
		out = append(out, a.blocks[id])
	}
	return out
}
