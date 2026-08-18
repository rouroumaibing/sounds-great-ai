package capability

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"sounds-great-ai/internal/component"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/pkg/pack"
)

// MemoryReflect is the LLM abstractive-summary capability for the typed-lane
// Shared Memory subsystem. It is a SANCTIONED platform synthesis service under
// irreversible-decisions §4.8 (and §4.4 "平台合成走 Eino"), homologous to
// clowder's ReflectionService / AbstractiveSummaryClient.
//
// It does NOT perform agent reasoning (VISION §3): it only synthesizes an
// abstractive reflection over already-approved truth. The output never
// auto-becomes truth — callers must route it through human disposition
// (M5 提交权). The model is opt-in and configured via env
// (SG_REFLECT_API_KEY + SG_REFLECT_MODEL, or SG_REFLECT_CLI).
type MemoryReflect struct {
	chat model.BaseChatModel
}

// NewMemoryReflect builds a reflection capability over an injected chat model.
// The model is injected (not constructed here) so the capability stays
// testable with a stub and decoupled from config sourcing.
func NewMemoryReflect(chat model.BaseChatModel) *MemoryReflect {
	return &MemoryReflect{chat: chat}
}

// Name / Version / Init / Health / Close implement the capability adapter
// contract (consistent with ContextAssemble / AgentDispatch).
func (r *MemoryReflect) Name() string    { return "memory_reflect" }
func (r *MemoryReflect) Version() string { return "v1" }
func (r *MemoryReflect) Init(ctx context.Context) error { return nil }
func (r *MemoryReflect) Health() error {
	if r.chat == nil {
		return fmt.Errorf("memory_reflect: no chat model configured")
	}
	return nil
}
func (r *MemoryReflect) Close() error { return nil }

// ReflectOptions tunes the reflection synthesis.
type ReflectOptions struct {
	// MaxChars caps the requested reflection length (0 = default 2000).
	MaxChars int
	// Focus is an optional free-text directive ("focus on collaboration style").
	Focus string
}

// Reflect synthesizes an abstractive reflection over the supplied approved
// truth entries. Entries are already human-approved; the model only
// summarizes / surfaces cross-entry patterns — it authors no new truth.
func (r *MemoryReflect) Reflect(ctx context.Context, entries []*memory.LaneEntry, opts ReflectOptions) (string, error) {
	if r.chat == nil {
		return "", fmt.Errorf("memory_reflect: no chat model configured")
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("memory_reflect: no entries to reflect on")
	}
	maxChars := opts.MaxChars
	if maxChars <= 0 {
		maxChars = 2000
	}
	prompt := buildReflectPrompt(entries, opts.Focus, maxChars)
	resp, err := r.chat.Generate(ctx, []*schema.Message{
		schema.SystemMessage(systemReflectPrompt),
		schema.UserMessage(prompt),
	})
	if err != nil {
		return "", fmt.Errorf("memory_reflect: model generate failed: %w", err)
	}
	return strings.TrimSpace(resp.Content), nil
}

// Run adapts Reflect to the capability TaskInput/TaskOutput contract. Entries
// flow through the pipeline like every other capability: an upstream step's
// TaskOutput.Data["entries"] ([]*memory.LaneEntry) is collected; the reflection
// is returned in Data["reflection"] and is never auto-written to memory. The
// HTTP endpoint (LanesHandler.Reflect) does NOT use Run — it gathers approved
// truth from the registry and calls Reflect directly.
func (r *MemoryReflect) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	entries := collectEntriesFromPrevious(input)
	if len(entries) == 0 {
		return nil, fmt.Errorf("memory_reflect: no upstream entries (invoke via Reflect directly)")
	}
	reflection, err := r.Reflect(ctx, entries, ReflectOptions{
		MaxChars: getIntConfig(input.CapabilityConfig, "max_chars", 2000),
		Focus:    focusFromConfig(input),
	})
	if err != nil {
		return nil, err
	}
	return &pack.TaskOutput{
		Approved: true,
		Reason:   "reflected on memory truth",
		Data:     map[string]any{"reflection": reflection},
	}, nil
}

// collectEntriesFromPrevious gathers []*memory.LaneEntry from any upstream
// TaskOutput.Data["entries"], following the capability data-flow convention.
func collectEntriesFromPrevious(input *pack.TaskInput) []*memory.LaneEntry {
	var entries []*memory.LaneEntry
	if input == nil || input.Previous == nil {
		return entries
	}
	for _, prev := range input.Previous {
		if prev == nil || prev.Data == nil {
			continue
		}
		if raw, ok := prev.Data["entries"]; ok {
			if es, ok := raw.([]*memory.LaneEntry); ok {
				entries = append(entries, es...)
			}
		}
	}
	return entries
}

// focusFromConfig reads an optional "focus" directive from a TaskInput's
// CapabilityConfig (string) so the capability adapter honors the same knob the
// direct Reflect call does.
func focusFromConfig(input *pack.TaskInput) string {
	if input == nil || input.CapabilityConfig == nil {
		return ""
	}
	if f, ok := input.CapabilityConfig["focus"].(string); ok {
		return f
	}
	return ""
}

const systemReflectPrompt = `你是犬队协作平台的记忆反省合成器。你会收到人类已批准的记忆 truth 条目（决策 / 经验 / 偏好 / 身份 / 实体 / 事件）。
任务：产出一段简洁的抽象反省，凸显跨条目的模式、张力与可长期复用的要点。
严格约束：
- 不编造事实，只基于给定条目综合。
- 这是"合成"不是"决策"：不要替人类做任何动作或判断。
- 输出纯文本，语言与输入一致（默认中文）。`

func buildReflectPrompt(entries []*memory.LaneEntry, focus string, maxChars int) string {
	var sb strings.Builder
	sb.WriteString("以下为已批准的记忆 truth 条目，请产出抽象反省：\n\n")
	for i, e := range entries {
		fmt.Fprintf(&sb, "[%d] (%s) %s\n", i+1, e.Type, e.Content)
	}
	sb.WriteString("\n")
	if focus != "" {
		fmt.Fprintf(&sb, "反省聚焦点：%s\n", focus)
	}
	fmt.Fprintf(&sb, "请控制在约 %d 字符以内。\n", maxChars)
	return sb.String()
}

// NewReflectModelFromEnv builds a chat model for memory reflection from env.
// Either an OpenAI-compatible API key (SG_REFLECT_API_KEY + SG_REFLECT_MODEL,
// optional SG_REFLECT_BASE_URL) or a CLI model (SG_REFLECT_CLI, optional
// SG_REFLECT_CLI_ARGS) must be configured. Returns an error when neither is
// set — callers treat that as "reflection unavailable" (platform stays
// deterministic).
func NewReflectModelFromEnv(ctx context.Context) (model.BaseChatModel, error) {
	if cli := os.Getenv("SG_REFLECT_CLI"); cli != "" {
		return component.NewChatModel(ctx, &component.ModelConfig{
			Type:    component.ProviderTypeCLI,
			CLIPath: cli,
			CLIArgs: strings.Fields(os.Getenv("SG_REFLECT_CLI_ARGS")),
		})
	}
	apiKey := os.Getenv("SG_REFLECT_API_KEY")
	modelName := os.Getenv("SG_REFLECT_MODEL")
	if apiKey == "" || modelName == "" {
		return nil, fmt.Errorf("memory_reflect: configure SG_REFLECT_API_KEY + SG_REFLECT_MODEL (api_key) or SG_REFLECT_CLI (cli)")
	}
	return component.NewChatModel(ctx, &component.ModelConfig{
		Type:      component.ProviderTypeAPIKey,
		APIKey:    apiKey,
		ModelName: modelName,
		BaseURL:   os.Getenv("SG_REFLECT_BASE_URL"),
	})
}
