package eval

import "testing"

func TestResultStore_SaveAndGetVerdict(t *testing.T) {
	dir := t.TempDir()
	s := NewResultStore(dir)
	v := &VerdictHandoffPacket{ID: "v1", DomainID: "d1", Verdict: "keep", Phenomenon: "test"}
	if err := s.SaveVerdict(v); err != nil {
		t.Fatalf("SaveVerdict: %v", err)
	}
	got, err := s.GetVerdict("v1")
	if err != nil {
		t.Fatalf("GetVerdict: %v", err)
	}
	if got.ID != "v1" {
		t.Errorf("ID = %q, want %q", got.ID, "v1")
	}
}

func TestResultStore_ListVerdicts(t *testing.T) {
	dir := t.TempDir()
	s := NewResultStore(dir)
	_ = s.SaveVerdict(&VerdictHandoffPacket{ID: "v1", DomainID: "d1", Verdict: "keep"})
	_ = s.SaveVerdict(&VerdictHandoffPacket{ID: "v2", DomainID: "d2", Verdict: "fix"})
	all, err := s.ListVerdicts("")
	if err != nil {
		t.Fatalf("ListVerdicts: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d verdicts, want 2", len(all))
	}
	d1, err := s.ListVerdicts("d1")
	if err != nil {
		t.Fatalf("ListVerdicts d1: %v", err)
	}
	if len(d1) != 1 {
		t.Fatalf("got %d verdicts for d1, want 1", len(d1))
	}
}

func TestResultStore_ListVerdicts_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	s := NewResultStore(dir)
	result, err := s.ListVerdicts("")
	if err != nil {
		t.Fatalf("ListVerdicts: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("got %d verdicts, want 0", len(result))
	}
}

func TestResultStore_GetVerdict_NotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewResultStore(dir)
	_, err := s.GetVerdict("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent verdict")
	}
}
