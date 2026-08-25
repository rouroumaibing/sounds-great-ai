package voice

import (
	"errors"
	"sync"
)

// STTProvider transcribes an audio reference to text (roadmap P1-B STT接入).
// Implementations wrap upstream ASR services; the cache/registry here stays
// transport-free.
type STTProvider interface {
	// Name identifies the provider (e.g. "whisper", "azure").
	Name() string
	// Transcribe returns the recognized text for an audio reference.
	Transcribe(audioRef string) (string, error)
}

// ErrProviderNotFound is returned by the registry when a provider is unknown.
var ErrProviderNotFound = errors.New("voice: stt provider not found")

// STTRegistry holds available STT providers keyed by name. Goroutine-safe.
type STTRegistry struct {
	mu        sync.RWMutex
	providers map[string]STTProvider
}

// NewSTTRegistry creates an empty registry.
func NewSTTRegistry() *STTRegistry {
	return &STTRegistry{providers: make(map[string]STTProvider)}
}

// Register adds or replaces a provider.
func (r *STTRegistry) Register(p STTProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Get returns a provider by name (nil, ErrProviderNotFound if absent).
func (r *STTRegistry) Get(name string) (STTProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return p, nil
}

// Transcribe is a convenience that looks up a provider and transcribes.
func (r *STTRegistry) Transcribe(name, audioRef string) (string, error) {
	p, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return p.Transcribe(audioRef)
}

// List returns registered provider names.
func (r *STTRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.providers))
	for n := range r.providers {
		out = append(out, n)
	}
	return out
}
