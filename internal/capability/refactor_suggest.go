package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"sounds-great-ai/pkg/pack"
)

// RefactorResult is the structured output of refactor_suggest.
type RefactorResult struct {
	Summary     string       `json:"summary"`
	Suggestions []Suggestion `json:"suggestions"`
}

// Suggestion is one refactoring suggestion.
type Suggestion struct {
	File       string `json:"file"`
	OldPattern string `json:"old_pattern"`
	NewPattern string `json:"new_pattern"`
	Reason     string `json:"reason"`
	Risk       string `json:"risk"` // low|medium|high
}

const refactorSuggestSystemSuffix = `
You are a refactoring expert. Based on the analysis, propose concrete
executable suggestions. Output JSON:
{"suggestions": [{"file": "...", "old_pattern": "...", "new_pattern": "...", "reason": "...", "risk": "low|medium|high"}], "summary": "..."}
`

type RefactorSuggest struct{}

func NewRefactorSuggest() *RefactorSuggest { return &RefactorSuggest{} }

func (r *RefactorSuggest) Name() string    { return "refactor_suggest" }
func (r *RefactorSuggest) Version() string { return "v1" }

func (r *RefactorSuggest) Init(ctx context.Context) error { return nil }
func (r *RefactorSuggest) Health() error                  { return nil }
func (r *RefactorSuggest) Close() error                   { return nil }

func (r *RefactorSuggest) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	var ar AnalysisResult
	if input.Previous != nil {
		if analyzeOut, ok := input.Previous["analyze"]; ok && analyzeOut != nil {
			if a, ok := analyzeOut.Data["analysis"]; ok {
				_ = decodeData(a, &ar)
			}
		}
	}

	if len(ar.Findings) == 0 {
		return &pack.TaskOutput{
			Data: map[string]any{"refactor": RefactorResult{Summary: "no refactor needed"}},
		}, nil
	}

	userContent := formatAnalysisForLLM(ar)

	parsed, _ := callLLMWithFallback(ctx, llmCallSpec{
		Breed:         input.Breed,
		SystemSuffix:  refactorSuggestSystemSuffix,
		UserContent:   userContent,
		MaxInputChars: 8000,
		Parse: func(b []byte) (any, error) {
			var rr RefactorResult
			if err := json.Unmarshal(b, &rr); err != nil {
				return nil, err
			}
			return rr, nil
		},
		Fallback: func() any {
			suggestions := make([]Suggestion, 0, len(ar.Findings))
			for _, f := range ar.Findings {
				suggestions = append(suggestions, Suggestion{
					File:       f.File,
					OldPattern: f.Description,
					NewPattern: "",
					Reason:     "derived from finding: " + f.Description,
					Risk:       "medium",
				})
			}
			return RefactorResult{
				Summary:     fmt.Sprintf("LLM unavailable; %d raw suggestions", len(suggestions)),
				Suggestions: suggestions,
			}
		},
	})

	rr, _ := parsed.(RefactorResult)
	return &pack.TaskOutput{
		Data: map[string]any{"refactor": rr},
	}, nil
}

func formatAnalysisForLLM(ar AnalysisResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Summary: %s\n", ar.Summary)
	for i, f := range ar.Findings {
		fmt.Fprintf(&sb, "%d. [%s] %s:%d — %s\n", i+1, f.Severity, f.File, f.Line, f.Description)
	}
	return sb.String()
}
