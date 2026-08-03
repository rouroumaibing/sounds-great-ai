package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"sounds-great-ai/pkg/pack"
)

// FormattedOutput is the structured output of format_output.
type FormattedOutput struct {
	Sections map[string]string `json:"sections"` // by stepID
	Combined string            `json:"combined"`
}

type FormatOutput struct{}

func NewFormatOutput() *FormatOutput { return &FormatOutput{} }

func (f *FormatOutput) Name() string    { return "format_output" }
func (f *FormatOutput) Version() string { return "v1" }

func (f *FormatOutput) Init(ctx context.Context) error { return nil }
func (f *FormatOutput) Health() error                  { return nil }
func (f *FormatOutput) Close() error                   { return nil }

func (f *FormatOutput) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	sections := make(map[string]string)
	if input.Previous != nil {
		// Sort stepIDs for deterministic Combined output
		ids := make([]string, 0, len(input.Previous))
		for id := range input.Previous {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		var combined string
		for _, id := range ids {
			out := input.Previous[id]
			if out == nil {
				continue
			}
			text := formatOneOutput(id, out)
			sections[id] = text
			combined += fmt.Sprintf("## %s\n%s\n\n", id, text)
		}
		return &pack.TaskOutput{
			Data: map[string]any{
				"formatted": FormattedOutput{Sections: sections, Combined: combined},
			},
		}, nil
	}

	return &pack.TaskOutput{
		Data: map[string]any{
			"formatted": FormattedOutput{Sections: sections},
		},
	}, nil
}

// formatOneOutput renders a single TaskOutput as readable text.
// Recognizes known structures; falls back to JSON pretty.
func formatOneOutput(stepID string, out *pack.TaskOutput) string {
	if out == nil {
		return "(nil)"
	}
	var sb string
	if out.Reason != "" {
		sb = fmt.Sprintf("Reason: %s\n", out.Reason)
	}
	if len(out.Data) > 0 {
		b, _ := json.MarshalIndent(out.Data, "", "  ")
		sb += string(b)
	} else if len(out.Results) > 0 {
		b, _ := json.MarshalIndent(out.Results, "", "  ")
		sb += string(b)
	}
	return sb
}
