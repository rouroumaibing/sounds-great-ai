package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/internal/eval"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/internal/telemetry"
	"sounds-great-ai/internal/transport"
	"sounds-great-ai/pkg/pack"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/redis/go-redis/v9"
)

func LoadConfig() *component.ModelConfig {
	return &component.ModelConfig{
		Type:      component.ProviderType(os.Getenv("MODEL_TYPE")),
		BaseURL:   os.Getenv("MODEL_BASE_URL"),
		APIKey:    os.Getenv("MODEL_API_KEY"),
		ModelName: os.Getenv("MODEL_NAME"),
		CLIPath:   os.Getenv("MODEL_CLI_PATH"),
		CLIArgs:   []string{},
	}
}

func GetenvDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func GetenvDefaultInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return defaultVal
}

func SetupPack() (*pack.Pack, *ragstore.StoreRegistry, embedding.Embedder, *ragstore.RetiredCleaner, string) {
	p := pack.New("default")
	if err := p.RegisterCapability(capability.NewCommandCheck()); err != nil {
		log.Printf("Warning: CommandCheck registration failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewPathValidate()); err != nil {
		log.Printf("Warning: PathValidate registration failed: %v", err)
	}
	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workspaceDir = wd
		}
	}
	if err := p.RegisterCapability(capability.NewAgentDispatch()); err != nil {
		log.Printf("Warning: AgentDispatch registration failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewDispatchExecute(p)); err != nil {
		log.Printf("Warning: DispatchExecute registration failed: %v", err)
	}
	sf, err := capability.NewSensitiveFilter()
	if err == nil {
		_ = p.RegisterCapability(sf)
	} else {
		log.Printf("Warning: SensitiveFilter construction failed: %v", err)
	}
	ctx := context.Background()
	registry, _, cleaner, embedder, err := SetupRAG(ctx, workspaceDir)
	if err != nil {
		log.Printf("Warning: RAG init failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewContextAssemble()); err != nil {
		log.Printf("Warning: ContextAssemble registration failed: %v", err)
	}
	if registry != nil && embedder != nil {
		if err := p.RegisterCapability(capability.NewRetrieveCapability(registry, embedder)); err != nil {
			log.Printf("Warning: RetrieveCapability registration failed: %v", err)
		}
	}
	breedsDir := os.Getenv("BREEDS_DIR")
	if breedsDir == "" {
		breedsDir = "packs/default/breeds"
	}
	breedsFile := filepath.Join(breedsDir, "dog-template.json")
	if err := p.LoadFromFile(breedsFile, pack.LoadPolicySkipInvalid); err != nil {
		log.Printf("Warning: breed load failed: %v", err)
	}
	go WatchBreedsFile(p, breedsFile)
	return p, registry, embedder, cleaner, workspaceDir
}

// WatchBreedsFile polls the single consolidated breeds file and reloads the pack
// when it changes.
func WatchBreedsFile(p *pack.Pack, file string) {
	var lastMod int64
	if info, err := os.Stat(file); err == nil {
		lastMod = info.ModTime().UnixNano()
	}
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		mtime := info.ModTime().UnixNano()
		if mtime != lastMod {
			lastMod = mtime
			if err := p.ReloadFromFile(file, pack.LoadPolicySkipInvalid); err != nil {
				log.Printf("Warning: breed reload failed: %v", err)
			} else {
				log.Printf("Breeds reloaded from %s", file)
			}
		}
	}
}

func SetupRAG(ctx context.Context, workspaceDir string) (*ragstore.StoreRegistry, *ragstore.Migrator, *ragstore.RetiredCleaner, embedding.Embedder, error) {
	embedder, err := component.NewOpenAIEmbedder(ctx, component.EmbedConfig{
		APIKey:  os.Getenv("MODEL_API_KEY"),
		BaseURL: os.Getenv("MODEL_BASE_URL"),
		Model:   GetenvDefault("EMBEDDING_MODEL", "text-embedding-3-small"),
		Dim:     GetenvDefaultInt("EMBEDDING_DIM", 1536),
	})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("embedder init: %w", err)
	}

	backend := ragstore.BackendType(os.Getenv("RAG_STORE_BACKEND"))
	if backend == "" {
		backend = ragstore.BackendMemory
	}

	cfg := ragstore.StoreConfig{
		Backend:     backend,
		Embedder:    embedder,
		PersistPath: filepath.Join(workspaceDir, "rag_index.json"),
		SQLitePath:  filepath.Join(workspaceDir, "rag_index.db"),
	}
	initialStore, err := ragstore.NewStore(cfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("initial store: %w", err)
	}

	registry := ragstore.NewStoreRegistry(initialStore, backend)
	registry.SetActiveConfig(cfg)

	migrationDB := filepath.Join(workspaceDir, "migration.db")
	migrator, err := ragstore.NewMigrator(registry, migrationDB)
	if err != nil {
		log.Printf("Warning: migrator init failed: %v", err)
	} else {
		registry.SetMigrator(migrator)
		registry.SetDB(migrator.DB())
		registry.SetEmbedder(embedder)
		if err := registry.LoadRetirees(ctx, migrator.DB()); err != nil {
			log.Printf("Warning: load retirees failed: %v", err)
		}
	}

	cleaner := ragstore.NewRetiredCleaner(registry, 24*time.Hour)
	cleaner.Start()

	if os.Getenv("RAG_AUTO_INDEX") == "true" {
		docsDir := filepath.Join(workspaceDir, "docs")
		indexer := ragstore.NewIndexBuilder(initialStore, embedder, docsDir)
		go func() {
			log.Printf("RAG auto-index starting for %s", docsDir)
			if err := indexer.Rebuild(); err != nil {
				log.Printf("Warning: RAG auto-index failed: %v", err)
			} else {
				log.Printf("RAG auto-index completed")
			}
		}()
	}

	return registry, migrator, cleaner, embedder, nil
}

func SetupEval(ctx context.Context, pl *platform.Platform, workspaceDir string) (*eval.Scheduler, *transport.EvalHandler) {
	if pl == nil {
		return nil, nil
	}
	evalsDir := filepath.Join(workspaceDir, "packs/default/evals")
	evalDomains, err := eval.LoadDomains(evalsDir)
	if err != nil {
		log.Printf("Warning: eval domain load failed: %v", err)
	}
	if len(evalDomains) == 0 {
		return nil, nil
	}
	resultStore := eval.NewResultStore(filepath.Join(workspaceDir, "docs/eval-results"))
	var rdb *redis.Client
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("Warning: REDIS_URL parse failed: %v", err)
		} else {
			rdb = redis.NewClient(opts)
		}
	}
	evalRunner := eval.NewEvalRunner(pl, resultStore, evalDomains)
	closureSvc := eval.NewClosureService(rdb)
	scheduler := eval.NewScheduler(evalRunner, evalDomains, rdb)
	handler := transport.NewEvalHandler(evalRunner, resultStore, closureSvc, scheduler)
	go scheduler.Start(ctx)
	log.Printf("Eval subsystem initialized: %d domains", len(evalDomains))
	return scheduler, handler
}

func setupBurnRateMonitor(wsHandler *transport.WSHandler) (*telemetry.BurnRateMonitor, context.CancelFunc) {
	cfg := telemetry.DefaultBurnRateConfig()
	if v := os.Getenv("BURN_RATE_ERROR_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.ErrorRateThreshold = f
		}
	}
	if v := os.Getenv("BURN_RATE_P95_THRESHOLD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.P95LatencyThreshold = d
		}
	}
	cfg.ActiveInvocationsMax = GetenvDefaultInt("BURN_RATE_ACTIVE_MAX", cfg.ActiveInvocationsMax)
	if v := os.Getenv("BURN_RATE_CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CheckInterval = d
		}
	}
	cfg.ConsecutiveBreaches = GetenvDefaultInt("BURN_RATE_CONSECUTIVE_BREACHES", cfg.ConsecutiveBreaches)
	if v := os.Getenv("PROMETHEUS_URL"); v != "" {
		cfg.PrometheusURL = v
	}
	monitor := telemetry.NewBurnRateMonitor(cfg, func(alert telemetry.BurnRateAlert) {
		severity := "warning"
		title := "Burn-Rate 告警"
		switch alert.Type {
		case "error_rate":
			severity, title = "critical", "错误率超阈值"
		case "p95_latency":
			severity, title = "critical", "P95 延迟超阈值"
		case "active_invocations":
			severity, title = "warning", "活跃调用数超阈值"
		case "recovery":
			severity, title = "info", "指标恢复"
		}
		wsHandler.SendSystemNotice(severity, title, alert.Message)
	})
	monitorCtx, cancel := context.WithCancel(context.Background())
	monitor.Start(monitorCtx)
	log.Printf("Burn-Rate Monitor started (interval=%s, prometheus=%s)", cfg.CheckInterval, cfg.PrometheusURL)
	return monitor, cancel
}
