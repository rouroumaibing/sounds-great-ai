package eval

import "testing"

func TestMeasurementBundle_BirthCert(t *testing.T) {
	b := NewMeasurementBundle("m1", "inputs-blob", "judge-v3")
	if b.InputsHash == "" {
		t.Fatal("birth cert must be set")
	}
	if b.JudgeVersion != "judge-v3" {
		t.Fatalf("judge version wrong: %s", b.JudgeVersion)
	}
	// Same inputs+version => same birth cert (reproducible).
	b2 := NewMeasurementBundle("m1", "inputs-blob", "judge-v3")
	if b.InputsHash != b2.InputsHash {
		t.Fatal("birth cert not reproducible for identical inputs")
	}
}

func TestVersionedJudge_ScoreFailClosed(t *testing.T) {
	b := NewMeasurementBundle("m1", "in", "judge-v3")
	j := VersionedJudge{Version: "judge-v3"}
	if v, err := j.Score(b, 0.9); err != nil || v != 0.9 {
		t.Fatalf("matching judge should score: %v %v", v, err)
	}
	// Mismatched version fails closed.
	j2 := VersionedJudge{Version: "judge-v4"}
	if _, err := j2.Score(b, 0.9); err == nil {
		t.Fatal("mismatched judge version must fail closed")
	}
}
