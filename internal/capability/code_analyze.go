package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"sounds-great-ai/pkg/pack"
)

// AnalysisResult is the structured output of code_analyze.
type AnalysisResult struct {
	Summary  string    `json:"summary"`
	Findings []Finding `json:"findings"`
}

// Finding is a single code analysis finding.
type Finding struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

const codeAnalyzeSystemSuffix = `
You are a code analysis expert. Based on search results, analyze dependencies,
call chains, potential issues. Output JSON:
{"summary": "...", "findings": [{"type": "...", "file": "...", "line": 0, "severity": "low|medium|high", "description": "..."}]}
`

// CodeAnalyze is the code_analyze:v1 capability.
type CodeAnalyze struct{}

func NewCodeAnalyze() *CodeAnalyze { return &CodeAnalyze{} }

func (c *CodeAnalyze) Name() string    { return "code_analyze" }
func (c *CodeAnalyze) Version() string { return "v1" }

func (c *CodeAnalyze) Init(ctx context.Context) error { return nil }
func (c *CodeAnalyze) Health() error                  { return nil }
func (c *CodeAnalyze) Close() error                   { return nil }

// Run reads Previous["search"].Data["matches"], formats them, calls LLM
// (with fallback to wrapping each match as a Finding).
func (c *CodeAnalyze) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	var matches []SearchMatch
	if input.Previous != nil {
		if searchOut, ok := input.Previous["search"]; ok && searchOut != nil {
			if m, ok := searchOut.Data["matches"]; ok {
				_ = decodeData(m, &matches) // tolerate decode failure → empty matches
			}
		}
	}

	if len(matches) == 0 {
		return &pack.TaskOutput{
			Data: map[string]any{"analysis": AnalysisResult{}},
		}, nil
	}

	userContent := formatMatchesForLLM(matches)

	parsed, _ := callLLMWithFallback(ctx, llmCallSpec{
		Breed:         input.Breed,
		SystemSuffix:  codeAnalyzeSystemSuffix,
		UserContent:   userContent,
		MaxInputChars: 12000,
		Parse: func(b []byte) (any, error) {
			var ar AnalysisResult
			if err := json.Unmarshal(b, &ar); err != nil {
				return nil, err
			}
			return ar, nil
		},
		Fallback: func() any {
			// Wrap each match as a finding
			findings := make([]Finding, 0, len(matches))
			for _, m := range matches {
				findings = append(findings, Finding{
					Type:        "search_match",
					File:        m.File,
					Line:        m.Line,
					Severity:    "info",
					Description: m.Content,
				})
			}
			return AnalysisResult{
				Summary:  fmt.Sprintf("LLM unavailable; %d raw matches", len(matches)),
				Findings: findings,
			}
		},
	})

	ar, _ := parsed.(AnalysisResult)
	return &pack.TaskOutput{
		Data: map[string]any{"analysis": ar},
	}, nil
}

// formatMatchesForLLM renders matches as "file:line: content" text.
func formatMatchesForLLM(matches []SearchMatch) string {
	var sb strings.Builder
	for _, m := range matches {
		fmt.Fprintf(&sb, "%s:%d: %s\n", m.File, m.Line, m.Content)
	}
	return sb.String()
}
