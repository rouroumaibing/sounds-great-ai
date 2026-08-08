package pack

import (
	"context"
	"testing"
)

func TestBarkBreedNotFound(t *testing.T) {
	p := New("test")
	_, err := p.Bark(context.Background(), "nonexistent", &TaskInput{})
	if err == nil {
		t.Error("expected error for nonexistent breed, got nil")
	}
}
