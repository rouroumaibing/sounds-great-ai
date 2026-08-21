package governance

import (
	"encoding/json"
	"fmt"
	"os"
)

// Attestation pins the committed baseline digest to an authorized bootstrap
// reference. The baseline cannot be silently rotated without also rotating the
// attestation that references it. The digest is computed over the baseline, so
// any tool-surface change forces a conscious re-attestation.
type Attestation struct {
	SchemaVersion    int    `json:"schemaVersion"`
	FeatureID        string `json:"featureId"`
	TargetRepository string `json:"targetRepository"`
	BaselineDigest   string `json:"baselineDigest"`
	Owner            string `json:"owner"`
	AuthorizationRef string `json:"authorizationRef"`
}

// GenerateAttestation builds an attestation that references the given baseline.
func GenerateAttestation(baseline Baseline, targetRepo, owner, authRef string) Attestation {
	return Attestation{
		SchemaVersion:    BaselineSchemaVersion,
		FeatureID:        BaselineFeatureID,
		TargetRepository: targetRepo,
		BaselineDigest:   baseline.Digest,
		Owner:            owner,
		AuthorizationRef: authRef,
	}
}

// WriteAttestation writes an attestation file (0o600).
func WriteAttestation(path string, a Attestation) error {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ValidateAttestation enforces the three-way invariant:
//   live catalog digest == committed baseline digest == attestation baselineDigest.
//
// Any mismatch fails closed, forbidding the baseline from drifting from what
// was authorized.
func ValidateAttestation(attestationPath, baselinePath string, catalog []ToolDefinition) error {
	araw, err := os.ReadFile(attestationPath)
	if err != nil {
		return fmt.Errorf("mcp surface attestation missing (%s): %w", attestationPath, err)
	}
	var a Attestation
	if err := json.Unmarshal(araw, &a); err != nil {
		return fmt.Errorf("mcp surface attestation corrupt: %w", err)
	}
	if a.SchemaVersion != BaselineSchemaVersion || a.FeatureID != BaselineFeatureID {
		return fmt.Errorf("mcp surface attestation has unexpected schema/featureId")
	}

	braw, err := os.ReadFile(baselinePath)
	if err != nil {
		return fmt.Errorf("mcp surface baseline missing (%s): %w", baselinePath, err)
	}
	var b Baseline
	if err := json.Unmarshal(braw, &b); err != nil {
		return fmt.Errorf("mcp surface baseline corrupt: %w", err)
	}

	live := ComputeBaseline(catalog)
	if live.Digest != b.Digest {
		return fmt.Errorf("mcp surface drift: live digest %s != committed baseline %s", live.Digest, b.Digest)
	}
	if a.BaselineDigest != b.Digest {
		return fmt.Errorf("mcp surface attestation references %s but baseline is %s", a.BaselineDigest, b.Digest)
	}
	return nil
}
