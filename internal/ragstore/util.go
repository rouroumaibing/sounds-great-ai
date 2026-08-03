// internal/ragstore/util.go
package ragstore

import (
	"encoding/binary"
	"math"
)

// cosineSimilarity computes cosine similarity between two vectors.
// Returns 0.0 for zero vectors to avoid NaN (which breaks sort.Slice).
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0.0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// encodeFloat64Slice serializes []float64 to a byte slice (LittleEndian).
func encodeFloat64Slice(v []float64) []byte {
	buf := make([]byte, 8*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(f))
	}
	return buf
}

// decodeFloat64Slice deserializes a byte slice to []float64 (LittleEndian).
func decodeFloat64Slice(b []byte) []float64 {
	n := len(b) / 8
	v := make([]float64, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
	}
	return v
}
