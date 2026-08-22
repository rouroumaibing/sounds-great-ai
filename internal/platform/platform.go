package platform

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/redis/go-redis/v9"
	"sounds-great-ai/internal/a2a"
	a2aadapter "sounds-great-ai/internal/adapter/a2a"
	"sounds-great-ai/internal/adapter/claude"
	"sounds-great-ai/internal/adapter/codex"
	"sounds-great-ai/internal/adapter/gemini"
	"sounds-great-ai/internal/adapter/kimi"
	"sounds-great-ai/internal/adapter/opencode"
	"sounds-great-ai/internal/adapter/pool"
	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/dossier"
	"sounds-great-ai/internal/hooks"
	"sounds-great-ai/internal/mcp"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/prompt"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/internal/settings"

	routingPorts "sounds-great-ai/internal/domains/routing/ports"
	routingServices "sounds-great-ai/internal/domains/routing/services"
	routingStores "sounds-great-ai/internal/domains/routing/stores"
	threadPorts "sounds-great-ai/internal/domains/threads/ports"
	threadStores "sounds-great-ai/internal/domains/threads/stores"
	"sounds-great-ai/internal/skills"
	"sounds-great-ai/internal/sop"
	"sounds-great-ai/internal/threadstore"

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
	// CarrierRegistry is the R1 multi-tier carrier registry (R2/R3/R6). Adapters
	// route Execute through it; health-based fallback and the warm-pool
	// transport hook in here. Nil-safe: when nil, adapters fall back to a direct
	// one-shot pm.Spawn — behavior identical to pre-R2.
	CarrierRegistry *unified.Registry
	// carrierHealth backs the registry's R6 degradation state (default in-memory).
	carrierHealth unified.CarrierHealth
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
	// MCPStore is the persistent, operator-managed registry of MCP servers.
	// It owns persistence (mcp-servers.json) and keeps MCP in sync.
	MCPStore *mcp.FileStore
	A2AHub   routingPorts.IA2AHub
	SOP      sopPorts.IA2AGuardian
	Memory   *memory.MemoryStore

	// Prompt Hooks
	HookRegistry   *hooks.Registry
	HookPipeline   *hooks.Pipeline
	HookTraceStore *hooks.TraceStore

	// Dossier is the capability-dossier loader (FT-DS-001). It reads
	// docs/team/dog-dossier.md (repo doc, git-versioned) and feeds the prompt
	// builder's roster/identity projections; the distillation API shares the
	// same loader for apply-time cache invalidation.
	Dossier *dossier.Loader
	// DossierService orchestrates observations, checkpoint opportunities, and
	// the distillation proposal pipeline (FT-DS-001). Nil only in minimal
	// test wiring.
	DossierService *dossier.Service

	// Stores (port/factory pattern)
	ThreadStore   threadPorts.IThreadStore
	SettingsStore settings.SettingsStore
	EvidenceStore memory.EvidenceStore
	// SharedMemory is the typed-lane registry (Persistent Identity layer).
	// Lane entries are submitted as pending candidates at session close (P2),
	// disposed by a human (P3), and the approved subset is recalled into dog
	// prompts (P4). SQLite-persisted
	// via NewLaneRegistryAt so typed memory survives restarts.
	SharedMemory *memory.LaneRegistry
	// LaneSupply detects typed deltas at session close and submits them as
	// pending lane candidates. Deterministic pattern matching — no LLM (VISION §3).
	LaneSupply *memory.DeltaProducer
	// LaneDispositions records human dispositions on lane candidates and applies
	// them to the registry (approve/reject/modify -> lane status transitions).
	LaneDispositions *memory.DispositionRecorder
	// LaneRecall records memory-recall events (injection observability). Backs
	// the frontend RecallFeed/RecallLedger so the operator can see what memory
	// was surfaced.
	LaneRecall *memory.RecallStore
	// Profiles persists relationship capsules (Persistent Identity P1). Kept
	// separate from SettingsStore on purpose:
	// capsules live in their own directory, never in dog-catalog.json.
	Profiles *settings.ProfileRepository
	// Continuity persists the last-session digest per breed (Persistent
	// Identity P3). Separate
	// directory, never in dog-catalog.json.
	Continuity *settings.ContinuityStore
	// PeopleMemory persists owner-private third-party people & relationship
	// memory (Persistent Identity). Multi-operator:
	// every operator's data is partitioned by operatorID. File-backed by
	// default (zero-dependency); when SG_REDIS_URL is set it is Redis-backed
	// (operator-keyed keyspace + Lua-guarded deferred-receipt lifecycle).
	PeopleMemory settings.PeopleMemoryStore
	// PeopleMemoryHub is the in-process event bus powering the people-memory
	// SSE endpoint for cross-tab live sync. Nil only in tests that skip wiring.
	PeopleMemoryHub *settings.PeopleMemoryEventHub

	// SessionBreed maps an active session id to the breed (dog) running it.
	// It lets the autonomous-distill endpoint derive the distiller from the
	// CURRENT session (the dog distills its own primer),
	// instead of a hardcoded default. Populated best-effort on each spawn and
	// read on distill. Guarded by SessionBreedMu.
	SessionBreed   map[string]string
	SessionBreedMu sync.RWMutex

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
	RedisURL     string // empty = in-memory carrier health; set to enable RedisHealth
}

// maskRedisURL hides credentials in a Redis URL before logging.
func maskRedisURL(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if j := strings.Index(rest, "@"); j >= 0 {
			return u[:i+3] + "***" + rest[j:]
		}
	}
	return u
}

// New initializes the full platform layer.
func New(cfg Config) (*Platform, error) {
	pm := unified.NewProcessManager()

	// Initialize adapters
	//
	// §4.7: the A2A protocol client is a sibling carrier of the CLI adapters.
	// It is keyed by client_id="a2a" (or a more specific "a2a-<name>") and
	// calls an EXTERNAL agent via Google A2A Protocol tasks/send. Endpoints are
	// wired from breed variant.a2a_url after the catalog merges (see below).
	a2aAdapter := a2aadapter.New(pm, "a2a")
	adapters := map[string]unified.AgentExecutor{
		"claude":   claude.New(pm),
		"codex":    codex.New(pm),
		"gemini":   gemini.New(pm),
		"kimi":     kimi.New(pm),
		"opencode": opencode.New(pm),
		"a2a":      a2aAdapter,
	}

	// R1/R2/R3/R6 carrier registry. The default carrier chain is per-provider:
	// claude/codex/gemini lead with bg_daemon (long-session warm-pool tier) and
	// fall back to print_sdk; opencode/kimi stay one-shot (print_sdk).
	//
	// 2026-08-17: the warm-pool (R2) tier is now DEFAULT-ON. pty is compiled in
	// unconditionally (no -tags pty build needed) and p.WireWarmPools() below
	// wires the per-provider warm pools + PtyRunner, so the bg_daemon tier in the
	// chain is actually live (not a transparent one-shot fallback). R3 PTY stays
	// reserved/opt-in (only needed for CLIs requiring a real TTY). R6: RedisHealth
	// is compiled in by default but only activated when a Redis URL is configured;
	// otherwise we keep the zero-dependency in-memory store.
	var carrierHealth unified.CarrierHealth = unified.NewMemoryHealth()
	var rclient *redis.Client
	if cfg.RedisURL != "" {
		rclient = redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
		carrierHealth = unified.NewRedisHealth(rclient)
		log.Printf("carrier health: Redis-backed (url=%s)", maskRedisURL(cfg.RedisURL))
	} else {
		log.Printf("carrier health: in-memory (set SG_REDIS_URL to enable Redis)")
	}
	registry := unified.NewRegistry(carrierHealth)
	registry.RegisterTransport(unified.NewProcessTransport(pm))
	registry.RegisterTransport(unified.NewPtyTransport()) // R3 reserved; not in default chain
	for name, a := range adapters {
		switch v := a.(type) {
		case *claude.Adapter:
			v.SetRegistry(registry, name)
		case *codex.Adapter:
			v.SetRegistry(registry, name)
		case *gemini.Adapter:
			v.SetRegistry(registry, name)
		case *kimi.Adapter:
			v.SetRegistry(registry, name)
		case *opencode.Adapter:
			v.SetRegistry(registry, name)
		}
	}
	// Per-provider default carrier chain (claude-first). claude/codex/gemini
	// PREFER the bg_daemon (warm-pool) long-session tier; opencode/kimi stay
	// one-shot. The bg_daemon tier is wired by p.WireWarmPools() below (called
	// unconditionally since pty is now always compiled); without that call the
	// registry would transparently fall back to print_sdk.
	for name := range adapters {
		chain := []unified.TransportKind{unified.TransportPrintSDK}
		// Per-provider long session: claude/codex/gemini lead with bg_daemon
		// (warm pool), falling back to one-shot print_sdk. opencode/kimi stay
		// one-shot.
		switch name {
		case "claude", "codex", "gemini":
			chain = []unified.TransportKind{unified.TransportBgDaemon, unified.TransportPrintSDK}
		}
		registry.RegisterCarrier(&unified.Carrier{
			ID:         name,
			Provider:   name,
			Transports: chain,
		})
	}

	// 2026-08-17: warm-pool (R2 bg_daemon) is now DEFAULT-ON. pty is compiled in
	// unconditionally, so WireWarmPools wires the per-provider warm pools +
	// PtyRunner and the bg_daemon tier above becomes live (claude/codex/gemini
	// reuse long-lived CLI processes). Without this call the bg_daemon tier is
	// never wired and those CLIs transparently fall back to one-shot. The pools
	// are lazy: no process is spawned until the first turn Acquires a lease.
	// (The actual WireWarmPools() call is made after `pl` is constructed below,
	// because it needs pl.CarrierRegistry + pl.WorkspaceDir.)

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

	// §4.7: wire A2A protocol client endpoints from breed variant config.
	// A variant with client_id="a2a" (or "a2a-<name>") and a non-empty
	// a2a_url routes to the A2A client adapter. Specific client_ids beyond the
	// base "a2a" are registered on demand so distinct external agents can be
	// addressed independently.
	for _, b := range breeds {
		for _, v := range b.Variants {
			if v.ClientID == "" || v.A2AURL == "" {
				continue
			}
			a2aAdapter.SetEndpoint(v.ClientID, v.A2AURL, "")
			if v.ClientID != "a2a" {
				adapters[v.ClientID] = a2aAdapter
			}
		}
	}

	// Initialize platform services
	// 技能意图分两层：home（全局基线 + 权威禁用）与 workspace（工作区覆盖），
	// 合并后驱动注入与级联挂载（详见 internal/skills）。
	homeDir, _ := os.UserHomeDir()
	homeCfg := filepath.Join(homeDir, ".sounds-great-ai", "skills-config.json")
	projCfg := filepath.Join(cfg.WorkspaceDir, ".sounds-great-ai", "skills-config.json")
	skillMgr := skills.NewManagerWithConfig(homeCfg, projCfg, map[string]string{cfg.SkillsDir: "packs"})
	_ = skillMgr.Config().Load() // best effort
	_ = skillMgr.Scan()          // best effort
	// G1：接线上 skills-config.json 热加载——外部进程编辑后自动刷新内存态
	// （Watch 内部 3s 轮询 + 30s 防抖；ReloadAll 重载 global+project 两层并重扫源）。
	skillMgr.Config().Watch(func() {
		if err := skillMgr.ReloadAll(); err != nil {
			log.Printf("Warning: skills reload after hot-reload failed: %v", err)
		}
	})

	mcpReg := mcp.NewRegistry()
	mcpStore := mcp.NewFileStore(settings.ConfigRoot(cfg.WorkspaceDir), mcpReg)

	a2aHub := routingStores.NewA2AHubAdapter(a2a.NewHub(nil))

	maxDepth := cfg.MaxA2ADepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	sopGuardian := sop.NewGuardian(nil, maxDepth)

	// Persistent Identity (P0): experience
	// memory survives restarts. Stored next to dog-catalog.json under the same
	// ConfigRoot so a single directory holds all durable identity state.
	memStore := memory.NewMemoryStoreAt(
		filepath.Join(settings.ConfigRoot(cfg.WorkspaceDir), "memory.json"),
	)

	// Initialize stores (port/factory pattern)
	threadStore, err := threadstore.NewThreadStore(threadstore.StoreConfig{
		SQLitePath: cfg.SQLitePath,
	})
	if err != nil {
		return nil, fmt.Errorf("create thread store: %w", err)
	}
	threadStorePort := threadStores.NewThreadStoreAdapter(threadStore)
	evidenceStore := memory.NewEvidenceStoreAt(
		filepath.Join(settings.ConfigRoot(cfg.WorkspaceDir), "evidence.json"),
	)

	// Persistent Identity (Shared Memory): typed-lane registry. Lane entries are
	// submitted as pending candidates at session close (P2) and disposed by a
	// human (P3); the approved subset is recalled into dog prompts (P4). Stored
	// next to memory.json/evidence.json under the same ConfigRoot so a single
	// directory holds all durable identity state. SQLite-persisted so typed
	// memory survives restarts.
	sharedMemory := memory.NewLaneRegistryAt(
		filepath.Join(settings.ConfigRoot(cfg.WorkspaceDir), "lanes.json"),
	)
	// Recall-event store (injection observability). Persisted as
	// recall-events.jsonl under ConfigRoot.
	laneRecall := memory.NewRecallStore(filepath.Join(settings.ConfigRoot(cfg.WorkspaceDir), "lanes.json"))

	promptBuilder := prompt.NewBuilder(breeds, skillMgr)
	contextAssembler := prompt.NewContextAssembler()

	// FT-DS-001: capability dossier (docs/team/dog-dossier.md). Identity
	// 擅长 line and roster strengths/routing columns prefer dossier
	// projections over config fallbacks; full profiles stay on-demand reads.
	dossierLoader := dossier.NewLoader()
	promptBuilder.SetDossier(dossier.NewReader(dossierLoader, cfg.WorkspaceDir))

	// FT-DS-001: distillation pipeline stores. Observations and proposals are
	// durable team state (main SQLite DB, factory falls back to in-memory when
	// SQLitePath is empty); opportunities are transient (in-memory by design).
	dossierObservations, err := dossier.NewObservationStoreAt(cfg.SQLitePath)
	if err != nil {
		return nil, fmt.Errorf("create dossier observation store: %w", err)
	}
	dossierProposals, err := dossier.NewProposalStoreAt(cfg.SQLitePath)
	if err != nil {
		return nil, fmt.Errorf("create dossier proposal store: %w", err)
	}
	dossierOpportunities := dossier.NewInMemoryOpportunityStore()
	dossierCheckpoint := dossier.NewCheckpoint(dossierOpportunities, func(format string, args ...any) {
		log.Printf(format, args...)
	})
	dossierService := dossier.NewService(dossierProposals, dossierObservations, dossierOpportunities, dossierCheckpoint, dossierLoader, cfg.WorkspaceDir)

	// FT-DS-001 AC-C2: wire the review-complete checkpoint into the SOP
	// guardian's write-back path (best-effort — a review completion creates
	// a distillation opportunity for the reviewed author).
	sopService := sopServices.NewSOPGuardianService(sopGuardian)
	sopService.SetReviewCompleteListener(func(prov sop.ReviewProvenance) {
		dossierCheckpoint.OnReviewComplete(dossier.ReviewCompleteContext{
			ThreadID:      prov.ReviewerThreadID,
			ReviewerDogID: prov.ReviewerDogID,
			AuthorDogID:   prov.AuthorDogID,
			CommitSHA:     prov.ReviewSHA,
		})
	})

	// Initialize prompt hooks (session-init: S1-S4)
	hooksDir := filepath.Join(cfg.WorkspaceDir, "packs/default/hooks")
	hookReg := hooks.NewRegistry(hooksDir)
	if err := hookReg.Scan(); err != nil {
		log.Printf("Warning: hooks scan failed: %v", err)
	}
	// 8.4：把技能管理器注入 d11 skill-trigger resolver，使其能按当前查询动态
	// 匹配已启用的 skill（动态选择，非硬编码 DAG，对齐 §4.2）。
	resolvers := hooks.DefaultResolvers()
	resolvers["SkillTriggerResolver"] = &hooks.SkillTriggerResolver{Skills: skillMgr}
	hookPipeline := hooks.NewPipeline(hookReg, resolvers)

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
	// Wire the live default-breed resolver so the member-management "全局默认犬"
	// selector (persisted via /api/config/default-breed) actually drives which
	// dog executes an un-@mentioned conversation. Mirrors GetDefaultBreed:
	// the DEFAULT_BREED_ID env override wins, otherwise the persisted config.
	mentionRouter.SetDefaultBreedProvider(func() string {
		if id := os.Getenv("DEFAULT_BREED_ID"); id != "" {
			return id
		}
		if configs, err := settingsStore.ListConfig(); err == nil {
			for _, c := range configs {
				if c.Key == "default_breed" {
					return c.Value
				}
			}
		}
		return ""
	})
	worklist := routingServices.NewWorklistRegistry()

	// Project archive source (G8): file-backed repo trajectory store + git-ref
	// collector. repo_url defaults empty → the collector is inert until the
	// operator configures a code-repo URL (平台能力清单 §6 new capability).
	repoTrajectoryStore := custodyStores.NewRepoTrajectoryStore(
		filepath.Join(settings.ConfigRoot(cfg.WorkspaceDir), settings.RepoTrajectoryFileName),
	)
	gitRefCollector := custodyServices.NewGitRefCollector(repoTrajectoryStore, nil)

	// Initialize leader config, loading any persisted value from the catalog.
	leaderCfg := pack.DefaultLeaderConfig()
	if stored, err := settingsStore.GetLeader(); err == nil && stored != nil {
		leaderCfg = *stored
	}

	// Persistent Identity (P1): relationship capsules persist in their own
	// directory (NOT dog-catalog.json), honoring the cell-boundary discipline
	// (relationship ≠ breed config). Single-operator
	// form: the operator namespace is the leader name — the one human operator
	// in SG. The repository is injected into the prompt builder so each spawn
	// re-binds the dog to its long-term relationship with the operator.
	operator := leaderCfg.Name
	if operator == "" {
		operator = "operator"
	}
	profiles := settings.NewProfileRepository(settings.ConfigRoot(cfg.WorkspaceDir), operator)
	promptBuilder.SetProfiles(profiles)

	// Persistent Identity (P3): a per-breed last-session digest survives restarts
	// and separate one-shot spawns. The
	// prompt builder injects it as a "续接上下文" section so the dog resumes
	// awareness of what it was last doing (continuity bootstrap). In one-shot
	// mode the rebuilt identity block already covers "who am I"; this adds
	// "what I was working on".
	continuity := settings.NewContinuityStore(settings.ConfigRoot(cfg.WorkspaceDir))
	promptBuilder.SetContinuity(continuity)

	// Persistent Identity (Shared Memory): recall approved lane truth into the
	// dog's system prompt. Only human-approved entries are injected (M5
	// submission boundary); pending candidates never
	// enter the prompt.
	promptBuilder.SetLaneTruth(sharedMemory, operator)
	// Gap4 cue-plane: wire the relevance-ranked truth reader so the builder
	// injects opportunity-scored truth instead of a flat dump. *memory.LaneRegistry
	// satisfies LaneCueReader (CueMemory), so sharedMemory is passed directly.
	promptBuilder.SetLaneCue(sharedMemory)

	// Persistent Identity: owner-private
	// third-party people & relationship memory. Multi-operator: every operator's
	// data is partitioned by operatorID. File-backed by default (zero-dependency);
	// when SG_REDIS_URL is set we use the Redis-backed store (operator-keyed
	// keyspace + Lua-guarded deferred-receipt lifecycle), reusing the same rclient.
	var peopleMemory settings.PeopleMemoryStore
	// In-process event hub for cross-tab live sync (SSE). Shared between the
	// broadcasting store decorator and the HTTP SSE handler.
	pmHub := settings.NewPeopleMemoryEventHub()
	if rclient != nil {
		peopleMemory = settings.NewRedisPeopleMemoryStore(rclient)
		log.Printf("people-memory: Redis-backed (operator-partitioned)")
	} else {
		peopleMemory = settings.NewFilePeopleMemoryStore(settings.ConfigRoot(cfg.WorkspaceDir))
		log.Printf("people-memory: file-backed (operator-partitioned, set SG_REDIS_URL for Redis)")
	}
	// Decorating here means EVERY mutation path broadcasts (handler calls AND
	// the daily clerk), so any open people-memory tab refreshes on a change.
	peopleMemory = settings.NewBroadcastingPeopleMemoryStore(peopleMemory, pmHub)

	pl := &Platform{
		ProcessManager:  pm,
		CarrierRegistry: registry,
		carrierHealth:   carrierHealth,
		AgentExecutor:   agentsServices.NewAgentExecutor(adapters),
		Breeds:          breeds,
		Loader:          loader,
		Leader:          &leaderCfg,
		Skills:          skillMgr,
		MCP:             mcpReg,
		MCPStore:        mcpStore,
		A2AHub:          a2aHub,
		SOP:             sopService,
		Memory:          memStore,

		ThreadStore:      threadStorePort,
		SettingsStore:    settingsStore,
		EvidenceStore:    evidenceStore,
		SharedMemory:     sharedMemory,
		LaneSupply:       memory.NewDeltaProducer(),
		LaneDispositions: memory.NewDispositionRecorder(),
		LaneRecall:       laneRecall,
		Profiles:         profiles,
		Continuity:       continuity,
		PeopleMemory:     peopleMemory,
		PeopleMemoryHub:  pmHub,

		WorkspaceDir: cfg.WorkspaceDir,

		PromptBuilder:    promptBuilder,
		ContextAssembler: contextAssembler,
		Dossier:          dossierLoader,
		DossierService:   dossierService,

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
	}

	// 2026-08-17: warm-pool (R2 bg_daemon) is now DEFAULT-ON. pl.WireWarmPools
	// wires the per-provider warm pools + PtyRunner so the bg_daemon tier in the
	// carrier chain becomes live (claude/codex/gemini reuse long-lived CLI
	// processes). pty is compiled in unconditionally, so this call is always
	// effective. Pools are lazy: no process is spawned until the first turn
	// Acquires a lease.
	pl.WireWarmPools()

	// Start the daily deferred-receipt clerk:
	// aligned to "30 4 * * *" (04:30 local), it promotes ready deferred receipts
	// into rejectable candidates. The goroutine is cancelled when the process
	// exits (context.Background is fine for a long-lived daemon; it never
	// silently materializes truth).
	settings.StartPeopleMemoryClerk(context.Background(), pl.PeopleMemory, pl.peopleMemoryClerkDeps())

	return pl, nil
}

// GetAdapter returns the CLI adapter for a given CLI name. It delegates to the
// agents domain port (D4-3) and is retained for eval + tests that need the
// concrete unified.AgentExecutor.
func (p *Platform) GetAdapter(cliName string) (unified.AgentExecutor, error) {
	return p.AgentExecutor.Get(cliName)
}

// SetHealthBroadcaster wires a client-facing health broadcaster (the WebSocket
// hub) into the carrier registry so CARRIER_HEALTH events reach the frontend
// (T25 / R6). Safe to call with nil (defaults to a no-op broadcaster).
func (p *Platform) SetHealthBroadcaster(b unified.HealthBroadcaster) {
	if p.CarrierRegistry != nil && b != nil {
		p.CarrierRegistry.SetBroadcaster(b)
	}
}

// RegisterWarmPool enables the R2 warm-pool transport for claude only
// (per-provider claude-first). It registers the BgDaemonTransport backed by
// the given warm pool + claude-specific runner, and ensures claude's carrier
// chain leads with bg_daemon. Other providers stay one-shot regardless — the
// warm pool's PtyWarmSpawnFunc is claude-specific, so wiring it for codex/
// gemini/etc would spawn the wrong CLI. Called from WireClaudeWarmPool
// (compiled only under -tags pty). When bg_daemon is not wired (default build,
// or this is never called), claude's default chain still references bg_daemon
// but the registry finds no bg_daemon transport and falls back to print_sdk.
func (p *Platform) RegisterWarmPool(wp *pool.WarmPool, runner unified.WarmRunner) {
	if p.CarrierRegistry == nil || wp == nil || runner == nil {
		return
	}
	p.CarrierRegistry.RegisterTransport(unified.NewBgDaemonTransport(wp, runner, p.carrierHealth))
	const claudeProvider = "claude"
	p.CarrierRegistry.RegisterCarrier(&unified.Carrier{
		ID:         claudeProvider,
		Provider:   claudeProvider,
		Transports: []unified.TransportKind{unified.TransportBgDaemon, unified.TransportPrintSDK},
	})
}

// RegisterWarmPoolForProviders enables the R2 warm-pool (bg_daemon) transport
// for a set of providers (claude/codex/gemini long-session tiers), each with
// its own warm pool (per-provider spawn func). It registers a single
// BgDaemonTransport that routes to the correct per-provider pool by provider id
// and sets each provider's carrier chain to lead with bg_daemon, falling back
// to print_sdk. This is the "claude/codex/gemini 都能长会话"
// wiring; providers not in the map (opencode/kimi) stay one-shot. Called from
// WireWarmPools (compiled only under -tags pty). Safe to call when bg_daemon is
// not wired: the carriers still reference bg_daemon but the registry finds no
// transport and falls back to one-shot — zero new dependency.
func (p *Platform) RegisterWarmPoolForProviders(providers []string, pools map[string]*pool.WarmPool, runner unified.WarmRunner) {
	if p.CarrierRegistry == nil || runner == nil || len(pools) == 0 {
		return
	}
	p.CarrierRegistry.RegisterTransport(unified.NewBgDaemonTransportMulti(pools, runner, p.carrierHealth))
	for _, prov := range providers {
		if _, ok := pools[prov]; !ok {
			continue
		}
		p.CarrierRegistry.RegisterCarrier(&unified.Carrier{
			ID:         prov,
			Provider:   prov,
			Transports: []unified.TransportKind{unified.TransportBgDaemon, unified.TransportPrintSDK},
		})
	}
}

// GetBreed returns a breed config by ID.
func (p *Platform) GetBreed(id string) *pack.BreedConfig {
	return p.Breeds[id]
}

// RecordSessionBreed notes which breed (dog) is running a session. Best-effort:
// a missed record only degrades distill's session-derived resolution (it falls
// back to requiring an explicit ?client_id). The map is lazily allocated.
func (p *Platform) RecordSessionBreed(sessionID, breedID string) {
	if sessionID == "" || breedID == "" {
		return
	}
	p.SessionBreedMu.Lock()
	if p.SessionBreed == nil {
		p.SessionBreed = make(map[string]string)
	}
	p.SessionBreed[sessionID] = breedID
	p.SessionBreedMu.Unlock()
}

// BreedForSession returns the breed running a session, if known.
func (p *Platform) BreedForSession(sessionID string) (string, bool) {
	if sessionID == "" {
		return "", false
	}
	p.SessionBreedMu.RLock()
	defer p.SessionBreedMu.RUnlock()
	b, ok := p.SessionBreed[sessionID]
	return b, ok
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
		entry := unified.MCPServer{
			Name:    s.Name,
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
		}
		if s.URL != "" {
			// Remote server: emit url + optional headers and a transport type.
			// Streamable HTTP is the modern default; an "sse://" scheme (or
			// ?transport=sse) selects SSE.
			entry.Type = "http"
			if strings.HasPrefix(s.URL, "sse://") || strings.Contains(s.URL, "transport=sse") {
				entry.Type = "sse"
			}
			entry.URL = s.URL
			entry.Headers = s.Headers
			entry.Command = ""
			entry.Args = nil
			entry.Env = nil
		}
		result.Servers = append(result.Servers, entry)
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
// the projected custody state never hangs in "active" forever. Mirrors the
// zombie-reconciliation sweep. Call once at process startup (main.go).
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
