package platform

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/cloudwego/eino/components/embedding"
	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/adapter/claude"
	"sounds-great-ai/internal/adapter/codex"
	"sounds-great-ai/internal/adapter/gemini"
	"sounds-great-ai/internal/adapter/opencode"
	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/hooks"
	"sounds-great-ai/internal/mcp"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/prompt"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/internal/router"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/skills"
	"sounds-great-ai/internal/sop"
	"sounds-great-ai/internal/threadstore"

	"sounds-great-ai/pkg/pack"
)

// Platform wires together all platform-layer components.
// This is the integration point — the server initializes this struct
// and uses it to handle tasks.
type Platform struct {
	// CLI Adapters
	ProcessManager *unified.ProcessManager
	Adapters       map[string]unified.AgentExecutor // keyed by CLI name

	// Config
	Breeds map[string]*pack.BreedConfig
	Loader *pack.Loader
	Leader *pack.Leader

	// Platform Services
	Skills *skills.SkillManager
	MCP    *mcp.MCPRegistry
	Router *router.RoutingEngine
	A2AHub *a2a.A2AHub
	SOP    *sop.SOPGuardian
	Memory *memory.MemoryStore

	// Prompt Hooks
	HookRegistry   *hooks.Registry
	HookPipeline   *hooks.Pipeline
	HookTraceStore *hooks.TraceStore

	// Compressor for A2A handoffs
	Compressor *a2a.ContextCompressor

	// Stores (port/factory pattern)
	ThreadStore   threadstore.ThreadStore
	SettingsStore settings.SettingsStore
	EvidenceStore memory.EvidenceStore

	// Infrastructure
	WorkspaceDir string
	RAGRegistry  *ragstore.StoreRegistry
	Embedder     embedding.Embedder

	// Prompt
	PromptBuilder    *prompt.Builder
	ContextAssembler *prompt.ContextAssembler

	// Messages
	MessageStore threadstore.MessageStore

	// Routing
	MentionRouter *Router
}

// Config holds initialization parameters.
type Config struct {
	BreedsDir    string
	SkillsDir    string
	MaxA2ADepth  int
	WorkspaceDir string
	SQLitePath   string // empty = in-memory
}

// New initializes the full platform layer.
func New(cfg Config) (*Platform, error) {
	pm := unified.NewProcessManager()

	// Initialize adapters
	adapters := map[string]unified.AgentExecutor{
		"claude":   claude.New(pm),
		"codex":    codex.New(pm),
		"gemini":   gemini.New(pm),
		"opencode": opencode.New(pm),
	}

	// Initialize stores (port/factory pattern) — created before the breed
	// merge so the runtime catalog can be seeded/merged into the registry.
	settingsStore := settings.NewFileSettingsStore(
		filepath.Join(settings.ConfigRoot(cfg.WorkspaceDir), settings.AccountsFileName),
		filepath.Join(settings.ConfigRoot(cfg.WorkspaceDir), settings.CatalogFileName),
		true,
	)

	// Load breed configs (template seeds), then merge the runtime catalog.
	loader := &pack.Loader{Policy: pack.LoadPolicySkipInvalid}
	breeds, err := loader.LoadFromFile(filepath.Join(cfg.BreedsDir, "dog-template.json"))
	if err != nil {
		return nil, fmt.Errorf("load breeds: %w", err)
	}
	// Catalog is the single runtime truth; the template is a seed (copied into
	// the catalog on first init). Merged breeds drive routing, prompts, and the
	// platform registry. See plan 1.2 / decision D2.
	if merged, merr := MergedBreeds(breeds, settingsStore); merr != nil {
		log.Printf("Warning: breed catalog merge failed: %v", merr)
	} else {
		breeds = merged
	}

	// Initialize platform services
	skillMgr := skills.NewManager(cfg.SkillsDir)
	_ = skillMgr.LoadFromDir() // best effort

	mcpReg := mcp.NewRegistry()

	routingRules := []pack.RoutingRule{} // loaded from pack config
	routingEngine := router.NewEngine(routingRules, breeds)

	a2aHub := a2a.NewHub(nil)

	maxDepth := cfg.MaxA2ADepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	sopGuardian := sop.NewGuardian(nil, maxDepth)

	memStore := memory.NewMemoryStore()

	compressor := a2a.NewContextCompressor()

	// Initialize stores (port/factory pattern)
	threadStore, err := threadstore.NewThreadStore(threadstore.StoreConfig{
		SQLitePath: cfg.SQLitePath,
	})
	if err != nil {
		return nil, fmt.Errorf("create thread store: %w", err)
	}
	evidenceStore := memory.NewEvidenceStore()

	promptBuilder := prompt.NewBuilder(breeds, skillMgr)
	contextAssembler := prompt.NewContextAssembler()

	// Initialize prompt hooks (session-init: S1-S4)
	hooksDir := filepath.Join(cfg.WorkspaceDir, "packs/default/hooks")
	hookReg := hooks.NewRegistry(hooksDir)
	if err := hookReg.Scan(); err != nil {
		log.Printf("Warning: hooks scan failed: %v", err)
	}
	hookPipeline := hooks.NewPipeline(hookReg, hooks.DefaultResolvers())

	// Initialize hook trace store (graceful degradation on failure)
	var hookTraceStore *hooks.TraceStore
	traceDBPath := filepath.Join(cfg.WorkspaceDir, "hooks_trace.db")
	hookTraceStore, err = hooks.NewTraceStore(traceDBPath)
	if err != nil {
		log.Printf("Warning: hook trace store init failed: %v", err)
		hookTraceStore = nil // continue without tracing
	}
	messageStore, err := threadstore.NewMessageStore(threadstore.StoreConfig{
		SQLitePath: cfg.SQLitePath,
	})
	if err != nil {
		return nil, fmt.Errorf("create message store: %w", err)
	}
	router := NewRouter(breeds)

	// Initialize leader config, loading any persisted value from the catalog.
	leaderCfg := pack.DefaultLeaderConfig()
	if stored, err := settingsStore.GetLeader(); err == nil && stored != nil {
		leaderCfg = *stored
	}

	return &Platform{
		ProcessManager: pm,
		Adapters:       adapters,
		Breeds:         breeds,
		Loader:         loader,
		Leader:         &leaderCfg,
		Skills:         skillMgr,
		MCP:            mcpReg,
		Router:         routingEngine,
		A2AHub:         a2aHub,
		SOP:            sopGuardian,
		Memory:         memStore,
		Compressor:     compressor,

		ThreadStore:   threadStore,
		SettingsStore: settingsStore,
		EvidenceStore: evidenceStore,

		WorkspaceDir: cfg.WorkspaceDir,

		PromptBuilder:    promptBuilder,
		ContextAssembler: contextAssembler,

		HookRegistry:   hookReg,
		HookPipeline:   hookPipeline,
		HookTraceStore: hookTraceStore,

		MessageStore: messageStore,

		MentionRouter: router,
	}, nil
}

// GetAdapter returns the CLI adapter for a given CLI name.
func (p *Platform) GetAdapter(cliName string) (unified.AgentExecutor, error) {
	adapter, ok := p.Adapters[cliName]
	if !ok {
		return nil, fmt.Errorf("unknown CLI: %s", cliName)
	}
	return adapter, nil
}

// GetBreed returns a breed config by ID.
func (p *Platform) GetBreed(id string) *pack.BreedConfig {
	return p.Breeds[id]
}

// BuildMCPConfig returns MCP server configurations for CLI agents.
// This is global (not per-breed) — all MCP-supporting agents get the same config.
func (p *Platform) BuildMCPConfig() *unified.MCPConfig {
	if p.MCP == nil {
		return nil
	}
	servers := p.MCP.ForBreed(nil, "")
	if len(servers) == 0 {
		return nil
	}
	result := &unified.MCPConfig{Servers: make([]unified.MCPServer, 0, len(servers))}
	for _, s := range servers {
		result.Servers = append(result.Servers, unified.MCPServer{
			Name:    s.Name,
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
		})
	}
	return result
}

// Close releases platform resources. Called during graceful shutdown.
func (p *Platform) Close() error {
	// ProcessManager processes are cleaned up via context cancellation
	// in their respective goroutines. Nothing explicit needed here.
	return nil
}

// Ready checks if the platform is ready to serve requests.
func (p *Platform) Ready() bool {
	return len(p.Adapters) > 0 && len(p.Breeds) > 0
}
