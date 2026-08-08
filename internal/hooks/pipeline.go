package hooks

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
)

type PromptPatch struct {
	HookID  string
	Content string
	Order   int
}

type TraceEvent struct {
	HookID        string
	Status        string
	ContentHash   string
	TokenEstimate int
	Timestamp     time.Time
	ReasonCode    string
}

type PipelineResult struct {
	Patches []PromptPatch
	Events  []TraceEvent
}

type Pipeline struct {
	Registry  *Registry
	Resolvers map[string]Resolver
}

func NewPipeline(reg *Registry, resolvers map[string]Resolver) *Pipeline {
	return &Pipeline{Registry: reg, Resolvers: resolvers}
}

func (p *Pipeline) ExecuteStage(stage string, input *AssemblerInput) *PipelineResult {
	hooks := p.Registry.GetStageHooks(stage)
	result := &PipelineResult{}

	for _, h := range hooks {
		if !h.Manifest.Enabled {
			result.Events = append(result.Events, TraceEvent{
				HookID:    h.Manifest.ID,
				Status:    "disabled",
				Timestamp: time.Now(),
			})
			continue
		}

		var resolveResult ResolveResult
		if h.Manifest.Resolver == "" {
			resolveResult = ResolveResult{Status: "fired"}
		} else if resolver, ok := p.Resolvers[h.Manifest.Resolver]; ok {
			resolveResult = resolver.Resolve(input)
		} else {
			resolveResult = ResolveResult{Status: "fired"}
		}

		if resolveResult.Status == "skipped" {
			result.Events = append(result.Events, TraceEvent{
				HookID:     h.Manifest.ID,
				Status:     "skipped",
				ReasonCode: resolveResult.Reason,
				Timestamp:  time.Now(),
			})
			continue
		}

		content := renderTemplate(h.Template, resolveResult.Vars)

		result.Patches = append(result.Patches, PromptPatch{
			HookID:  h.Manifest.ID,
			Content: content,
			Order:   h.Manifest.Order,
		})
		result.Events = append(result.Events, TraceEvent{
			HookID:        h.Manifest.ID,
			Status:        "fired",
			ContentHash:   hashContent(content),
			TokenEstimate: estimateTokens(content),
			Timestamp:     time.Now(),
		})
	}

	return result
}

func AssemblePatches(patches []PromptPatch) string {
	var parts []string
	for _, p := range patches {
		parts = append(parts, p.Content)
	}
	return strings.Join(parts, "\n\n")
}

func renderTemplate(tmpl string, vars map[string]string) string {
	result := tmpl
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{."+k+"}}", v)
	}
	return result
}

func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}

func estimateTokens(content string) int {
	return len(content) / 4
}
