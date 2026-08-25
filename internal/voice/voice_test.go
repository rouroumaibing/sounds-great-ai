package voice

import (
	"errors"
	"testing"
)

func TestTTSCache_PutGetResume(t *testing.T) {
	c := NewTTSCache(0)
	c.Put(Segment{Text: "hello world", AudioRef: "a1", Position: 3})
	seg, ok := c.Get("hello world")
	if !ok || seg.AudioRef != "a1" || seg.Position != 3 {
		t.Fatalf("get mismatch: %+v ok=%v", seg, ok)
	}
	pos, ok := c.ResumePosition("hello world")
	if !ok || pos != 3 {
		t.Fatalf("resume pos = %d ok=%v", pos, ok)
	}
	// uncached text -> resume from 0
	if _, ok := c.ResumePosition("never seen"); ok {
		t.Fatal("uncached text should not have a resume position")
	}
}

func TestTTSCache_DerivesID(t *testing.T) {
	c := NewTTSCache(0)
	c.Put(Segment{Text: "x", AudioRef: "r", Position: 1})
	seg, _ := c.Get("x")
	if seg.ID == "" {
		t.Fatal("id should be derived from text")
	}
}

func TestTTSCache_LRUClean(t *testing.T) {
	c := NewTTSCache(2)
	c.Put(Segment{Text: "a", AudioRef: "ra", Position: 1})
	c.Put(Segment{Text: "b", AudioRef: "rb", Position: 2})
	c.Put(Segment{Text: "c", AudioRef: "rc", Position: 3}) // evicts "a"
	if c.Len() != 2 {
		t.Fatalf("len = %d, want 2", c.Len())
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("oldest segment should be evicted")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("newest segment should remain")
	}
}

// fakeSTT is a stub provider for tests.
type fakeSTT struct {
	name string
	text string
	err  error
}

func (f *fakeSTT) Name() string { return f.name }
func (f *fakeSTT) Transcribe(ref string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.text, nil
}

func TestSTTRegistry(t *testing.T) {
	r := NewSTTRegistry()
	r.Register(&fakeSTT{name: "whisper", text: "hi"})
	if _, err := r.Get("missing"); err == nil {
		t.Fatal("missing provider must error")
	}
	got, err := r.Transcribe("whisper", "audio1")
	if err != nil || got != "hi" {
		t.Fatalf("transcribe: %v %q", err, got)
	}
	// error propagates
	r.Register(&fakeSTT{name: "boom", err: errors.New("asr down")})
	if _, err := r.Transcribe("boom", "x"); err == nil {
		t.Fatal("provider error must propagate")
	}
	if len(r.List()) != 2 {
		t.Fatal("list length")
	}
}
