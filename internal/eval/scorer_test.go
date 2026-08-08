package eval

import "testing"

func TestScorer_ParseVerdict(t *testing.T) {
	s := &Scorer{}
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid verdict block",
			input: "```json-verdict\n" +
				`{"id":"v1","domainId":"d1","verdict":"fix","phenomenon":"test","evidence":{"snapshotRefs":["s1"]}}` + "\n```",
			wantErr: false,
		},
		{name: "no verdict block", input: "no block here", wantErr: true},
		{name: "unclosed block", input: "```json-verdict\n{...}", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.ParseVerdict(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVerdict() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractVerdictBlock(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "```json-verdict\n{}\n```", wantErr: false},
		{name: "missing open", input: "no open fence", wantErr: true},
		{name: "missing close", input: "```json-verdict\n{...}", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractVerdictBlock(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractVerdictBlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
