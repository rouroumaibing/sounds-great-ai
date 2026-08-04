package a2a

type ContextCompressor struct{}

func NewContextCompressor() *ContextCompressor {
	return &ContextCompressor{}
}

func (c *ContextCompressor) CompressHandoffFallback(thread *Thread, n int) []Message {
	history := thread.History
	if len(history) <= n {
		return history
	}
	return history[len(history)-n:]
}
