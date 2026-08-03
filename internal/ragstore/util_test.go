// internal/ragstore/util_test.go
package ragstore

import (
	"math"
	"testing"
)

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{0, 1}
	if got := cosineSimilarity(a, b); got != 0.0 {
		t.Fatalf("orthogonal vectors: want 0, got %v", got)
	}
}

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float64{1, 2, 3}
	if got := cosineSimilarity(a, a); math.Abs(got-1.0) > 1e-9 {
		t.Fatalf("identical vectors: want 1, got %v", got)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{1, 2, 3}
	// Zero vector must return 0.0, not NaN (NaN breaks sort.Slice)
	got := cosineSimilarity(a, b)
	if got != 0.0 {
		t.Fatalf("zero vector: want 0, got %v", got)
	}
	if math.IsNaN(got) {
		t.Fatal("zero vector returned NaN")
	}
}

func TestCosineSimilarity_LengthMismatch(t *testing.T) {
	a := []float64{1, 2}
	b := []float64{1, 2, 3}
	if got := cosineSimilarity(a, b); got != 0.0 {
		t.Fatalf("length mismatch: want 0, got %v", got)
	}
}

func TestEncodeDecodeFloat64Slice_RoundTrip(t *testing.T) {
	original := []float64{1.5, -2.3, 3.14159, 0, -0.0}
	encoded := encodeFloat64Slice(original)
	decoded := decodeFloat64Slice(encoded)
	if len(decoded) != len(original) {
		t.Fatalf("length: want %d, got %d", len(original), len(decoded))
	}
	for i := range original {
		if math.Abs(decoded[i]-original[i]) > 1e-12 {
			t.Fatalf("idx %d: want %v, got %v", i, original[i], decoded[i])
		}
	}
}

func TestDecodeFloat64Slice_Empty(t *testing.T) {
	got := decodeFloat64Slice(nil)
	if len(got) != 0 {
		t.Fatalf("empty: want 0, got %d", len(got))
	}
}
