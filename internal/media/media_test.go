package media

import (
	"errors"
	"testing"
)

func TestStore_PutGetDelete(t *testing.T) {
	s := NewInMemoryStore()
	r := Resource{ID: "r1", Kind: KindImage, Path: "/uploads/r1", Size: 100, Source: "generation"}
	if err := s.Put(r); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("r1")
	if err != nil || got.Size != 100 {
		t.Fatalf("get: %v %+v", err, got)
	}
	if _, err := s.Get("missing"); !errors.Is(err, ErrResourceNotFound) {
		t.Fatal("missing must error")
	}
	if err := s.Delete("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("r1"); !errors.Is(err, ErrResourceNotFound) {
		t.Fatal("deleted must be gone")
	}
}

func TestStore_ListByKind(t *testing.T) {
	s := NewInMemoryStore()
	_ = s.Put(Resource{ID: "a", Kind: KindImage})
	_ = s.Put(Resource{ID: "b", Kind: KindAudio})
	_ = s.Put(Resource{ID: "c", Kind: KindImage})
	if imgs := s.List(KindImage); len(imgs) != 2 {
		t.Fatalf("image count = %d", len(imgs))
	}
	if all := s.List(""); len(all) != 3 {
		t.Fatalf("all count = %d", len(all))
	}
}

func TestArchive_RichBlocks(t *testing.T) {
	a := NewArchive()
	a.Append(RichBlock{ID: "b1", Type: BlockText, Text: "hello"})
	a.Append(RichBlock{ID: "b2", Type: BlockImage, ResourceID: "r1"})
	if len(a.List()) != 2 {
		t.Fatal("archive length")
	}
	b, ok := a.Get("b2")
	if !ok || b.Type != BlockImage || b.ResourceID != "r1" {
		t.Fatalf("block mismatch: %+v", b)
	}
}

func TestPromoter_PromotesToUploads(t *testing.T) {
	s := NewInMemoryStore()
	p := NewPromoter(s)
	art := GeneratedArtifact{ID: "gen1", Kind: KindImage, StagingPath: "/tmp/gen1.png", Size: 2048, Source: "generation:image"}
	res, err := p.Promote(art)
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != "/uploads/gen1" {
		t.Fatalf("canonical path = %q", res.Path)
	}
	// now readable from the source of truth
	got, gerr := s.Get("gen1")
	if gerr != nil || got.Size != 2048 {
		t.Fatalf("promoted resource not readable: %v %+v", gerr, got)
	}
}

func TestPromoter_FailClosed(t *testing.T) {
	var nilStore Store
	p := &Promoter{store: nilStore}
	if _, err := p.Promote(GeneratedArtifact{ID: "x"}); err == nil {
		t.Fatal("nil store must error")
	}
	p2 := NewPromoter(NewInMemoryStore())
	if _, err := p2.Promote(GeneratedArtifact{}); err == nil {
		t.Fatal("empty id must error")
	}
}
