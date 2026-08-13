package platform

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

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
	"sounds-great-ai/internal/settings"

	routingPorts "sounds-great-ai/internal/domains/routing/ports"
	routingServices "sounds-great-ai/internal/domains/routing/services"
	routingStores "sounds-great-ai/internal/domains/routing/stores"
	"sounds-great-ai/internal/skills"
	"sounds-great-ai/internal/sop"
	"sounds-great-ai/internal/threadstore"
	threadPorts "sounds-great-ai/internal/domains/threads/ports"
	threadStores "sounds-great-ai/internal/domains/threads/stores"

	agentsPorts "sounds-great-ai/internal/domains/agents/ports"
	agentsServices "sounds-great-ai/internal/domains/agents/services"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	custodyServices "sounds-great-ai/internal/domains/custody/services"
	custodyStores "sounds-great-ai/internal/domains/custody/stores"

	sopPorts "sounds-great-ai/internal/domains/sop/ports"
	sopServices "sounds-great-ai/internal/domains/sop/services"

	"sounds-great-ai/pkg/pack"
)

// Platform wires together all platform-layer components.
// This is the integration point — the server initializes this struct
// and uses it to handle tasks.
type Platform struct {
	// CLI Adapters
	ProcessManager *unified.ProcessManager
	// AgentExecutor is the agents domain port wrapping the 4 CLI adapters
	// (claude/codex/gemini/opencode). execution.go consumes this port instead
	// of reaching into internal/adapter directly (D4-3).
	AgentExecutor agentsPorts.IAgentExecutor

	// Config
	Breeds map[string]*pack.BreedConfig
	Loader *pack.Loader
	Leader *pack.Leader

	// Platform Services
	Skills *skills.SkillManager
	MCP    *mcp.MCPRegistry
	A2AHub routingPorts.IA2AHub
	SOP    sopPorts.IA2AGuardian
	Memory *memory.MemoryStore

	// Prompt Hooks
	HookRegistry   *hooks.Registry
	HookPipeline   *hooks.Pipeline
	HookTraceStore *hooks.TraceStore

	// Compressor for A2A handoffs
	Compressor *a2a.ContextCompressor

	// Stores (port/factory pattern)
	ThreadStore   threadPorts.IThreadStore
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
	MessageStore threadPorts.IMessageStore

	// Ball custody ledger (orchestration ball-custody event ledger)
	BallLedger custodyPorts.IBallLedger

	// HoldScheduler manages parked holds (P2 hold_ball: manual/webhook wake).
	HoldScheduler custodyPorts.IHoldScheduler

	// Routing
	MentionRouter routingPorts.IMentionRouter

	// Worklist enforces per-invocation A2A depth + ping-pong breaking (G2).
	Worklist routingPorts.IWorklist

	// RepoTrajectoryStore + GitRefCollector power the project archive source (G8).
	// Both are nil-safe: when repo_url is empty the collector never runs.
	RepoTrajectoryStore *custodyStores.RepoTrajectoryStore
	GitRefCollector     *custodyServices.GitRefCollector
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

	a2aHub := routingStores.NewA2AHubAdapter(a2a.NewHub(nil))

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
	threadStorePort := threadStores.NewThreadStoreAdapter(threadStore)
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
	messageStorePort := threadStores.NewMessageStoreAdapter(messageStore)

	ballStore := custodyStores.NewMemoryBallLedgerStore()
	ballLedger := custodyServices.NewBallLedger(ballStore)
	holdScheduler := custodyServices.NewHoldScheduler(ballLedger, pm)
	mentionRouter := routingServices.NewMentionRouterService(breeds)
	worklist := routingServices.NewWorklistRegistry()

	// Project archive source (G8): file-backed repo trajectory store + git-ref
	// collector. repo_url defaults empty → the collector is inert until the
	// operator configures a code-repo URL (VISION §6 new capability).
	repoTrajectoryStore := custodyStores.NewRepoTrajectoryStore(
		filepath.Join(settings.ConfigRoot(cfg.WorkspaceDir), settings.RepoTrajectoryFileName),
	)
	gitRefCollector := custodyServices.NewGitRefCollector(repoTrajectoryStore, nil)

	// Initialize leader config, loading any persisted value from the catalog.
	leaderCfg := pack.DefaultLeaderConfig()
	if stored, err := settingsStore.GetLeader(); err == nil && stored != nil {
		leaderCfg = *stored
	}

	return &Platform{
		ProcessManager: pm,
		AgentExecutor:  agentsServices.NewAgentExecutor(adapters),
		Breeds:         breeds,
		Loader:         loader,
		Leader:         &leaderCfg,
		Skills:         skillMgr,
		MCP:            mcpReg,
		A2AHub:         a2aHub,
		SOP:            sopServices.NewSOPGuardianService(sopGuardian),
		Memory:         memStore,
		Compressor:     compressor,

		ThreadStore:   threadStorePort,
		SettingsStore: settingsStore,
		EvidenceStore: evidenceStore,

		WorkspaceDir: cfg.WorkspaceDir,

		PromptBuilder:    promptBuilder,
		ContextAssembler: contextAssembler,

		HookRegistry:   hookReg,
		HookPipeline:   hookPipeline,
		HookTraceStore: hookTraceStore,

		MessageStore: messageStorePort,

		BallLedger: ballLedger,

		HoldScheduler: holdScheduler,

		MentionRouter: mentionRouter,

		Worklist: worklist,

		RepoTrajectoryStore: repoTrajectoryStore,
		GitRefCollector:     gitRefCollector,
	}, nil
}

// GetAdapter returns the CLI adapter for a given CLI name. It delegates to the
// agents domain port (D4-3) and is retained for eval + tests that need the
// concrete unified.AgentExecutor.
func (p *Platform) GetAdapter(cliName string) (unified.AgentExecutor, error) {
	return p.AgentExecutor.Get(cliName)
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

// defaultReconcileInterval is how often the ball-custody zombie reconciler runs.
const defaultReconcileInterval = 60 * time.Second

// defaultReconcileTimeout is how long an invocation may go without a heartbeat
// (or start event, if no heartbeat was emitted) before it is healed as died.
const defaultReconcileTimeout = 5 * time.Minute

// repoCollectInterval is how often the git-ref collector snapshots branch heads
// (G8). It is decoupled from the reconcile interval so collection runs on its
// own cadence regardless of reconcile activity.
const repoCollectInterval = 5 * time.Minute

// StartReconciler runs the ball-custody zombie sweep until ctx is cancelled.
// It heals dangling invocations (started but never ended) into died/zombie so
// the projected custody state never hangs in "active" forever. Mirrors
// clowder-ai's reconcileZombies. Call once at process startup (main.go).
func (p *Platform) StartReconciler(ctx context.Context) {
	if p.BallLedger == nil {
		return
	}
	ticker := time.NewTicker(defaultReconcileInterval)
	defer ticker.Stop()
	var lastRepoCollect int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := p.BallLedger.ReconcileZombies(ctx, defaultReconcileTimeout)
			if err != nil {
				log.Printf("ball-custody reconciler error: %v", err)
			} else if n > 0 {
				log.Printf("ball-custody reconciler healed %d dangling invocation(s)", n)
			}
			// G5: advance timed/command holds (auto-wake or expire).
			if p.HoldScheduler != nil {
				if terr := p.HoldScheduler.Tick(time.Now().Unix()); terr != nil {
					log.Printf("hold scheduler tick error: %v", terr)
				}
			}
			// G8: periodic git-ref collection (only when repo_url is set).
			now := time.Now().Unix()
			if p.GitRefCollector != nil && now-lastRepoCollect >= int64(repoCollectInterval.Seconds()) {
				if repoURL := p.GetRepoURL(ctx); repoURL != "" {
					if collected, cerr := p.GitRefCollector.Collect(ctx, repoURL); cerr != nil {
						log.Printf("repo-trajectory collect error: %v", cerr)
					} else if collected > 0 {
						log.Printf("repo-trajectory collected %d branch event(s)", collected)
					}
					lastRepoCollect = now
				}
			}
		}
	}
}

// Close releases platform resources. Called during graceful shutdown.
func (p *Platform) Close() error {
	// ProcessManager processes are cleaned up via context cancellation
	// in their respective goroutines. Nothing explicit needed here.
	return nil
}

// HoldBall parks a thread, writing ball.held and registering an active hold.
// resumeMsg is the context handed back to the holder when the hold is released.
func (p *Platform) HoldBall(ctx context.Context, threadID, holder string, cond custodyPorts.WakeCondition, resumeMsg string) error {
	if p.HoldScheduler == nil {
		return fmt.Errorf("hold scheduler not initialized")
	}
	return p.HoldScheduler.Hold(ctx, threadID, holder, cond, resumeMsg)
}

// WakeHold releases a parked hold (validating wake kind/token) and returns the
// record so the caller can resume the holder breed.
func (p *Platform) WakeHold(ctx context.Context, threadID string, kind custodyPorts.WakeKind, token string) (*custodyPorts.HoldRecord, error) {
	if p.HoldScheduler == nil {
		return nil, fmt.Errorf("hold scheduler not initialized")
	}
	return p.HoldScheduler.Wake(ctx, threadID, kind, token)
}

// GetHold returns the active hold for a thread, if any.
func (p *Platform) GetHold(ctx context.Context, threadID string) (*custodyPorts.HoldRecord, bool) {
	if p.HoldScheduler == nil {
		return nil, false
	}
	return p.HoldScheduler.GetHold(ctx, threadID)
}

// Ready checks if the platform is ready to serve requests.
func (p *Platform) Ready() bool {
	return p.AgentExecutor != nil && p.AgentExecutor.Count() > 0 && len(p.Breeds) > 0
}

// GetRepoURL returns the configured code-repo URL (empty string when unset).
// Reads live from the settings store so a PUT /api/config/repo takes effect
// without a restart.
func (p *Platform) GetRepoURL(ctx context.Context) string {
	if p.SettingsStore == nil {
		return ""
	}
	configs, err := p.SettingsStore.ListConfig()
	if err != nil {
		return ""
	}
	for _, c := range configs {
		if c.Key == "repo_url" {
			return c.Value
		}
	}
	return ""
}
