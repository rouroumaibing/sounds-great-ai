package platform

import (
	"fmt"

	"sounds-great-ai/internal/adapter/claude"
	"sounds-great-ai/internal/adapter/codex"
	"sounds-great-ai/internal/adapter/gemini"
	"sounds-great-ai/internal/adapter/opencode"
	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/mcp"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/router"
	"sounds-great-ai/internal/skills"
	"sounds-great-ai/internal/sop"
)

// Platform wires together all platform-layer components.
// This is the integration point — the server initializes this struct
// and uses it to handle tasks.
type Platform struct {
	// CLI Adapters
	ProcessManager *unified.ProcessManager
	Adapters       map[string]unified.AgentExecutor // keyed by CLI name

	// Config
	Breeds  map[string]*config.BreedConfig
	Loader  *config.Loader

	// Platform Services
	Skills   *skills.SkillManager
	MCP      *mcp.MCPRegistry
	Router   *router.RoutingEngine
	A2AHub   *a2a.A2AHub
	SOP      *sop.SOPGuardian
	Memory   *memory.MemoryStore

	// Compressor for A2A handoffs
	Compressor *a2a.ContextCompressor
}

// Config holds initialization parameters.
type Config struct {
	BreedsDir   string
	SkillsDir   string
	MaxA2ADepth int
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

	// Load breed configs
	loader := config.NewLoader()
	breeds, err := loader.LoadFromDir(cfg.BreedsDir)
	if err != nil {
		return nil, fmt.Errorf("load breeds: %w", err)
	}

	// Initialize platform services
	skillMgr := skills.NewManager(cfg.SkillsDir)
	_ = skillMgr.LoadFromDir() // best effort

	mcpReg := mcp.NewRegistry()

	routingRules := []config.RoutingRule{} // loaded from pack config
	routingEngine := router.NewEngine(routingRules, breeds)

	a2aHub := a2a.NewHub(nil)

	maxDepth := cfg.MaxA2ADepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	sopGuardian := sop.NewGuardian(nil, maxDepth)

	memStore := memory.NewMemoryStore()

	compressor := a2a.NewContextCompressor()

	return &Platform{
		ProcessManager: pm,
		Adapters:       adapters,
		Breeds:         breeds,
		Loader:         loader,
		Skills:         skillMgr,
		MCP:            mcpReg,
		Router:         routingEngine,
		A2AHub:         a2aHub,
		SOP:            sopGuardian,
		Memory:         memStore,
		Compressor:     compressor,
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
func (p *Platform) GetBreed(id string) *config.BreedConfig {
	return p.Breeds[id]
}
