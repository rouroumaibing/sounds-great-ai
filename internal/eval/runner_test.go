package eval

import "testing"

func TestEvalRunner_Domains(t *testing.T) {
	domains := []EvalDomain{{DomainID: "d1", DisplayName: "Test", Description: "desc"}}
	r := NewEvalRunner(nil, NewResultStore(t.TempDir()), domains)
	got := r.Domains()
	if len(got) != 1 {
		t.Fatalf("got %d domains, want 1", len(got))
	}
	if got[0].DomainID != "d1" {
		t.Errorf("DomainID = %q, want %q", got[0].DomainID, "d1")
	}
}
