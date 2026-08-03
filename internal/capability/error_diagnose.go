package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"sounds-great-ai/pkg/pack"
)

// DiagnosisResult is the structured output of error_diagnose.
type DiagnosisResult struct {
	RootCause      string   `json:"root_cause"`
	Severity       string   `json:"severity"` // low|medium|high|critical
	FixSuggestions []string `json:"fix_suggestions"`
	RelatedFiles   []string `json:"related_files"`
}

const errorDiagnoseSystemSuffix = `
You are an error diagnosis expert. Based on the log trace, analyze the root
cause and suggest fixes. Output JSON:
{"root_cause": "...", "severity": "low|medium|high|critical", "fix_suggestions": ["..."], "related_files": ["..."]}
`

type ErrorDiagnose struct{}

func NewErrorDiagnose() *ErrorDiagnose { return &ErrorDiagnose{} }

func (e *ErrorDiagnose) Name() string    { return "error_diagnose" }
func (e *ErrorDiagnose) Version() string { return "v1" }

func (e *ErrorDiagnose) Init(ctx context.Context) error { return nil }
func (e *ErrorDiagnose) Health() error                  { return nil }
func (e *ErrorDiagnose) Close() error                   { return nil }

func (e *ErrorDiagnose) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	var tr TraceResult
	if input.Previous != nil {
		if tOut, ok := input.Previous["trace"]; ok && tOut != nil {
			if v, ok := tOut.Data["trace"]; ok {
				_ = decodeData(v, &tr)
			}
		}
	}

	if len(tr.Matches) == 0 {
		return &pack.TaskOutput{
			Data: map[string]any{
				"diagnosis": DiagnosisResult{RootCause: "no errors to diagnose"},
			},
		}, nil
	}

	userContent := formatTraceForLLM(tr)

	parsed, _ := callLLMWithFallback(ctx, llmCallSpec{
		Breed:         input.Breed,
		SystemSuffix:  errorDiagnoseSystemSuffix,
		UserContent:   userContent,
		MaxInputChars: 10000,
		Parse: func(b []byte) (any, error) {
			var dr DiagnosisResult
			if err := json.Unmarshal(b, &dr); err != nil {
				return nil, err
			}
			return dr, nil
		},
		Fallback: func() any {
			var sb strings.Builder
			for i, m := range tr.Matches {
				if i > 0 {
					sb.WriteString("; ")
				}
				fmt.Fprintf(&sb, "%s:%d %s", m.File, m.Line, m.Content)
			}
			return DiagnosisResult{
				RootCause:      "LLM unavailable; raw trace: " + sb.String(),
				Severity:       "unknown",
				FixSuggestions: []string{},
				RelatedFiles:   []string{},
			}
		},
	})

	dr, _ := parsed.(DiagnosisResult)
	return &pack.TaskOutput{
		Data: map[string]any{"diagnosis": dr},
	}, nil
}

func formatTraceForLLM(tr TraceResult) string {
	var sb strings.Builder
	for _, m := range tr.Matches {
		fmt.Fprintf(&sb, "[%s] %s:%d: %s\n", m.Level, m.File, m.Line, m.Content)
		for _, c := range m.Context {
			fmt.Fprintf(&sb, "  > %s\n", c)
		}
	}
	return sb.String()
}
