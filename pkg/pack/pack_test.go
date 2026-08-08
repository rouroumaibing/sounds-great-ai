package pack

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestRegisterSuccess(t *testing.T) {
	p := New("test")
	if err := p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"}); err != nil {
		t.Fatalf("RegisterCapability: %v", err)
	}

	breed := &BreedConfig{
		ID:               "breed1",
		Name:             "breed1",
		DefaultVariantID: "v1",
		Variants: []Variant{{
			ID:           "v1",
			ClientID:     "test",
			DefaultModel: "test-model",
		}},
		Source: BreedSourceUser,
	}
	if err := p.Register(breed); err != nil {
		t.Fatalf("Register: %v", err)
	}

	list := p.List()
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1", len(list))
	}
	if list[0].ID != "breed1" {
		t.Errorf("List[0].ID = %q, want %q", list[0].ID, "breed1")
	}
}

func TestRegisterSystemBreedProtection(t *testing.T) {
	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	systemBreed := &BreedConfig{
		ID:               "sys-breed",
		DefaultVariantID: "v1",
		Variants: []Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           BreedSourceSystem,
	}
	if err := p.Register(systemBreed); err != nil {
		t.Fatalf("Register system: %v", err)
	}

	// user 尝试覆盖 system breed
	userBreed := &BreedConfig{
		ID:               "sys-breed",
		DefaultVariantID: "v1",
		Variants: []Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	}
	err := p.Register(userBreed)
	if err == nil {
		t.Error("expected error overwriting system breed, got nil")
	}
}

func TestHasBreed(t *testing.T) {
	p := New("test")
	p.Register(&BreedConfig{
		ID:               "test-breed",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	})
	if !p.HasBreed("test-breed") {
		t.Error("HasBreed should return true for registered breed")
	}
	if p.HasBreed("nonexistent") {
		t.Error("HasBreed should return false for unregistered breed")
	}
}

func TestUnregisterSystemBreedRejection(t *testing.T) {
	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	systemBreed := &BreedConfig{
		ID:               "sys-breed",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           BreedSourceSystem,
	}
	p.Register(systemBreed)

	err := p.Unregister("sys-breed")
	if err == nil {
		t.Error("expected error deleting system breed, got nil")
	}
}

func TestUnregisterUserBreedSuccess(t *testing.T) {
	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	userBreed := &BreedConfig{
		ID:               "user-breed",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	}
	p.Register(userBreed)

	if err := p.Unregister("user-breed"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if len(p.List()) != 0 {
		t.Error("List should be empty after unregister")
	}
}

func TestUnregisterNotFound(t *testing.T) {
	p := New("test")
	err := p.Unregister("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent breed, got nil")
	}
}

func TestValidateWithoutMutating(t *testing.T) {
	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	breed := &BreedConfig{
		ID:               "breed1",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	}
	if err := p.Validate(breed); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Should not be registered
	if len(p.List()) != 0 {
		t.Error("List should be empty — Validate must not mutate state")
	}
}

func TestRegisterCapabilityDuplicate(t *testing.T) {
	p := New("test")
	if err := p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"}); err != nil {
		t.Fatalf("first RegisterCapability: %v", err)
	}
	err := p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})
	if err == nil {
		t.Error("expected error for duplicate capability, got nil")
	}
}

func TestCloseCallsCapabilityClose(t *testing.T) {
	p := New("test")
	mc := &closeTrackingCapability{name: "cap1", version: "v1"}
	p.RegisterCapability(mc)

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !mc.closed {
		t.Error("capability Close() was not called")
	}
}

func TestConcurrentRegisterAndList(t *testing.T) {
	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	// Pre-register a breed
	p.Register(&BreedConfig{
		ID:               "breed1",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			breed := &BreedConfig{
				ID:               fmt.Sprintf("breed-%d", n),
				DefaultVariantID: "v1",
				Variants:         []Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
				Source:           BreedSourceUser,
			}
			p.Register(breed)
		}(i)
		go func() {
			defer wg.Done()
			p.List()
		}()
	}
	wg.Wait()
}

func TestBarkInjectsBreed(t *testing.T) {
	p := New("test")

	breed := &BreedConfig{
		ID:               "breed1",
		Name:             "breed1",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "test", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	}
	if err := p.Register(breed); err != nil {
		t.Fatalf("Register: %v", err)
	}

	input := &TaskInput{Query: "hello"}
	out, err := p.Bark(context.Background(), "breed1", input)
	if err != nil {
		t.Fatalf("Bark: %v", err)
	}
	if out == nil {
		t.Fatal("Bark returned nil output")
	}

	if input.Breed == nil {
		t.Fatal("Breed was not injected into TaskInput")
	}
	if input.Breed.ID != "breed1" {
		t.Errorf("input.Breed.ID = %q, want %q", input.Breed.ID, "breed1")
	}
}

// breedCapturingCapability captures the Breed from TaskInput during Run
type breedCapturingCapability struct {
	name          string
	version       string
	capturedBreed *BreedConfig
}

func (m *breedCapturingCapability) Name() string                   { return m.name }
func (m *breedCapturingCapability) Version() string                { return m.version }
func (m *breedCapturingCapability) Init(ctx context.Context) error { return nil }
func (m *breedCapturingCapability) Run(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	m.capturedBreed = input.Breed
	return &TaskOutput{Approved: true}, nil
}
func (m *breedCapturingCapability) Health() error { return nil }
func (m *breedCapturingCapability) Close() error  { return nil }

// closeTrackingCapability 跟踪 Close 调用的 mock
type closeTrackingCapability struct {
	name    string
	version string
	closed  bool
}

func (c *closeTrackingCapability) Name() string                   { return c.name }
func (c *closeTrackingCapability) Version() string                { return c.version }
func (c *closeTrackingCapability) Init(ctx context.Context) error { return nil }
func (c *closeTrackingCapability) Run(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	return &TaskOutput{Approved: true}, nil
}
func (c *closeTrackingCapability) Health() error { return nil }
func (c *closeTrackingCapability) Close() error  { c.closed = true; return nil }
