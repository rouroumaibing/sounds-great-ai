package eval

import (
	"fmt"
	"hash/fnv"
)

// MeasurementBundle is a bundle of evaluation measurements with a birth
// certificate (F267: measurement bundle 出生证). The birth cert freezes the
// inputs + judge version so a measurement is reproducible and auditable.
type MeasurementBundle struct {
	ID           string
	InputsHash   string // birth cert: hash of the inputs the bundle was created from
	JudgeVersion string // versioned judge used to score the bundle
}

// BirthCert computes the birth-cert hash from the raw inputs + judge version.
func BirthCert(inputs, judgeVersion string) string {
	h := fnv.New64a()
	h.Write([]byte(inputs))
	h.Write([]byte("|"))
	h.Write([]byte(judgeVersion))
	return fmt.Sprintf("%016x", h.Sum64())
}

// NewMeasurementBundle creates a bundle with a frozen birth cert (F267).
func NewMeasurementBundle(id, inputs, judgeVersion string) *MeasurementBundle {
	return &MeasurementBundle{
		ID:           id,
		InputsHash:   BirthCert(inputs, judgeVersion),
		JudgeVersion: judgeVersion,
	}
}

// VersionedJudge scores a bundle under a specific judge version. It fails closed
// when the requested judge version does not match the bundle's frozen version
// (reproducibility guarantee).
type VersionedJudge struct {
	Version string
}

// Score scores the bundle. Returns an error if the judge version mismatches the
// bundle's birth-cert version (fail-closed: never score under the wrong judge).
func (j VersionedJudge) Score(b *MeasurementBundle, score float64) (float64, error) {
	if b.JudgeVersion != j.Version {
		return 0, fmt.Errorf("eval: judge version mismatch (bundle=%s judge=%s)", b.JudgeVersion, j.Version)
	}
	return score, nil
}
