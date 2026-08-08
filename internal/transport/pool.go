package transport

import (
	"sync"

	"sounds-great-ai/pkg/protocol"
)

// bufferPool reuses []byte slices for JSON marshaling to reduce GC pressure.
var bufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 512)
		return &b
	},
}

// GetBuffer retrieves a []byte from the pool (capacity 512, length 0).
func GetBuffer() *[]byte {
	return bufferPool.Get().(*[]byte)
}

// PutBuffer returns a []byte to the pool after resetting its length.
func PutBuffer(b *[]byte) {
	*b = (*b)[:0]
	bufferPool.Put(b)
}

// eventPool reuses protocol.Event structs to reduce allocations.
var eventPool = sync.Pool{
	New: func() interface{} {
		return &protocol.Event{}
	},
}

// GetEvent retrieves a *protocol.Event from the pool.
func GetEvent() *protocol.Event {
	return eventPool.Get().(*protocol.Event)
}

// PutEvent returns a *protocol.Event to the pool after zeroing it.
func PutEvent(e *protocol.Event) {
	*e = protocol.Event{}
	eventPool.Put(e)
}

// PoolStats returns basic pool statistics for diagnostics.
type PoolStats struct {
	// Note: sync.Pool doesn't expose counts directly.
	// These are placeholder fields for the diagnostics endpoint.
	BufferPoolNew int
	EventPoolNew  int
}
