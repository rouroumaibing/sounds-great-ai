package eval

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Scorer parses breed output into a VerdictHandoffPacket.
type Scorer struct{}

// ParseVerdict extracts the JSON verdict block from breed output and validates it.
// Breed output is expected to contain a ```json-verdict ... ``` fenced block.
func (s *Scorer) ParseVerdict(output string) (*VerdictHandoffPacket, error) {
	jsonStr, err := extractVerdictBlock(output)
	if err != nil {
		return nil, err
	}
	var v VerdictHandoffPacket
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return nil, fmt.Errorf("unmarshal verdict: %w", err)
	}
	if err := ValidateVerdict(&v); err != nil {
		return nil, fmt.Errorf("validate verdict: %w", err)
	}
	return &v, nil
}

// extractVerdictBlock pulls the content between ```json-verdict and ``` fences.
func extractVerdictBlock(output string) (string, error) {
	const open = "```json-verdict"
	const close = "```"
	start := strings.Index(output, open)
	if start == -1 {
		return "", fmt.Errorf("no json-verdict block found in breed output")
	}
	start += len(open)
	end := strings.Index(output[start:], close)
	if end == -1 {
		return "", fmt.Errorf("json-verdict block not closed")
	}
	return strings.TrimSpace(output[start : start+end]), nil
}
