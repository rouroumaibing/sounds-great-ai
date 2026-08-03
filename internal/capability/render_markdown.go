package capability

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"sounds-great-ai/pkg/pack"
)

type RenderMarkdown struct{}

func NewRenderMarkdown() *RenderMarkdown { return &RenderMarkdown{} }

func (r *RenderMarkdown) Name() string    { return "render_markdown" }
func (r *RenderMarkdown) Version() string { return "v1" }

func (r *RenderMarkdown) Init(ctx context.Context) error { return nil }
func (r *RenderMarkdown) Health() error                  { return nil }
func (r *RenderMarkdown) Close() error                   { return nil }

func (r *RenderMarkdown) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	var fo FormattedOutput
	if input.Previous != nil {
		if fOut, ok := input.Previous["format"]; ok && fOut != nil {
			if v, ok := fOut.Data["formatted"]; ok {
				_ = decodeData(v, &fo)
			}
		}
	}

	if len(fo.Sections) == 0 {
		return &pack.TaskOutput{
			Data: map[string]any{"markdown": "_(no content to render)_"},
		}, nil
	}

	ids := make([]string, 0, len(fo.Sections))
	for id := range fo.Sections {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var sb strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&sb, "## %s\n\n%s\n\n", id, fo.Sections[id])
	}
	return &pack.TaskOutput{
		Data: map[string]any{"markdown": sb.String()},
	}, nil
}
