package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"sounds-great-ai/internal/agent"
	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/eval"
	"sounds-great-ai/internal/hooks"
	"sounds-great-ai/internal/mcp"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/ops"
	"sounds-great-ai/internal/packapi"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/telemetry"
	"sounds-great-ai/internal/threadstore"
	"sounds-great-ai/internal/transport"
	"sounds-great-ai/pkg/pack"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()
	startTime := time.Now()
	cfg := loadConfig()

	// Initialize LogBuffer and wire it into the standard log package
	// so all log.Printf calls also write to the in-memory ring buffer.
	logBuf := ops.NewLogBuffer(1000)
	log.SetOutput(ops.NewLogWriter(os.Stderr, logBuf))

	// telemetry init (graceful degradation: failure does not block startup)
	telemetryCleanup, _ := telemetry.Init()
	defer telemetryCleanup()
	defer telemetry.Shutdown()

	_, err := component.NewChatModel(ctx, cfg)
	if err != nil {
		log.Printf("Warning: model init failed (server still starts): %v", err)
	}
	sm := &agent.SkillManager{}
	skillDir := os.Getenv("SKILL_DIR")
	if skillDir == "" {
		skillDir = "internal/agent/skills"
	}
	if err := sm.Load(skillDir); err != nil {
		log.Printf("Warning: skill load failed: %v", err)
	}

	// Initialize new platform layer
	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workspaceDir = wd
		}
	}
	breedsDir := os.Getenv("BREEDS_DIR")
	if breedsDir == "" {
		breedsDir = "packs/default/breeds"
	}
	skillsDir := os.Getenv("SKILLS_DIR")
	if skillsDir == "" {
		skillsDir = "packs/default/skills"
	}

	sqlitePath := os.Getenv("SQLITE_PATH")
	if sqlitePath == "" {
		sqlitePath = "data/sounds-great.db"
	}

	pl, err := platform.New(platform.Config{
		BreedsDir:    breedsDir,
		SkillsDir:    skillsDir,
		WorkspaceDir: workspaceDir,
		SQLitePath:   sqlitePath,
	})
	if err != nil {
		log.Printf("Warning: platform init failed (server runs in legacy mode): %v", err)
	}

	// Initialize legacy Pack system (backward compat for WS handler)
	p, registry, embedder, cleaner, _ := setupPack()
	defer p.Close()
	if cleaner != nil {
		defer cleaner.Stop()
	}

	// Wire RAG into platform if available
	if registry != nil && pl != nil {
		pl.RAGRegistry = registry
		pl.Embedder = embedder

		// Register RAG MCP server (shared knowledge service, not bound to any breed)
		mcpServerPath := filepath.Join(workspaceDir, "bin", "sounds-great-mcp-server")
		ragDBPath := filepath.Join(workspaceDir, "rag_index.db")
		pl.MCP.Register("knowledge", &mcp.MCPServerConfig{
			Name:    "knowledge",
			Command: mcpServerPath,
			Args:    []string{"--db", ragDBPath},
			Enabled: true,
		})
	}

	wsHandler := transport.NewWSHandler(p)
	if pl != nil {
		wsHandler = transport.NewWSHandlerWithPlatform(p, pl)
	}

	// Initialize eval subsystem
	var evalScheduler *eval.Scheduler
	var evalHandler *transport.EvalHandler
	if pl != nil {
		evalsDir := filepath.Join(workspaceDir, "packs/default/evals")
		evalDomains, err := eval.LoadDomains(evalsDir)
		if err != nil {
			log.Printf("Warning: eval domain load failed: %v", err)
		}
		if len(evalDomains) > 0 {
			resultStore := eval.NewResultStore(filepath.Join(workspaceDir, "docs/eval-results"))

			// Redis client (nil if REDIS_URL not set → memory fallback)
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
			evalScheduler = eval.NewScheduler(evalRunner, evalDomains, rdb)
			evalHandler = transport.NewEvalHandler(evalRunner, resultStore, closureSvc, evalScheduler)

			// Start scheduler in background
			go evalScheduler.Start(ctx)

			log.Printf("Eval subsystem initialized: %d domains", len(evalDomains))
		}
	}

	mux := buildMuxWithHandler(wsHandler, p, pl, registry, embedder, workspaceDir, startTime, evalHandler, logBuf)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Graceful shutdown (following clowder-ai pattern)
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Printf("Server starting on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Initialize Burn-Rate Monitor (Phase 7 Polish extension)
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	burnRateCfg := telemetry.DefaultBurnRateConfig()
	if v := os.Getenv("BURN_RATE_ERROR_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			burnRateCfg.ErrorRateThreshold = f
		}
	}
	if v := os.Getenv("BURN_RATE_P95_THRESHOLD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			burnRateCfg.P95LatencyThreshold = d
		}
	}
	burnRateCfg.ActiveInvocationsMax = getenvDefaultInt("BURN_RATE_ACTIVE_MAX", burnRateCfg.ActiveInvocationsMax)
	if v := os.Getenv("BURN_RATE_CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			burnRateCfg.CheckInterval = d
		}
	}
	burnRateCfg.ConsecutiveBreaches = getenvDefaultInt("BURN_RATE_CONSECUTIVE_BREACHES", burnRateCfg.ConsecutiveBreaches)
	if v := os.Getenv("PROMETHEUS_URL"); v != "" {
		burnRateCfg.PrometheusURL = v
	}

	burnRateMonitor := telemetry.NewBurnRateMonitor(burnRateCfg, func(alert telemetry.BurnRateAlert) {
		severity := "warning"
		title := "Burn-Rate 告警"
		switch alert.Type {
		case "error_rate":
			severity = "critical"
			title = "错误率超阈值"
		case "p95_latency":
			severity = "critical"
			title = "P95 延迟超阈值"
		case "active_invocations":
			severity = "warning"
			title = "活跃调用数超阈值"
		case "recovery":
			severity = "info"
			title = "指标恢复"
		}
		wsHandler.SendSystemNotice(severity, title, alert.Message)
	})
	burnRateMonitor.Start(monitorCtx)
	log.Printf("Burn-Rate Monitor started (interval=%s, prometheus=%s)", burnRateCfg.CheckInterval, burnRateCfg.PrometheusURL)

	sig := <-sigCh
	log.Printf("Received signal %s, shutting down...", sig)

	monitorCancel()
	burnRateMonitor.Stop()
	log.Printf("Burn-Rate Monitor stopped")

	shutdownCtx5s, cancel5s := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel5s()
	if err := srv.Shutdown(shutdownCtx5s); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	if pl != nil {
		if err := pl.Close(); err != nil {
			log.Printf("Platform close error: %v", err)
		}
	}
	if evalScheduler != nil {
		evalScheduler.Stop()
	}
	log.Printf("Server stopped")
}

func setupPack() (*pack.Pack, *ragstore.StoreRegistry, embedding.Embedder, *ragstore.RetiredCleaner, string) {
	p := pack.New("default")
	// Register capabilities (fixed Go code, once at startup)
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
	// Dispatch executor (framework capability)
	if err := p.RegisterCapability(capability.NewDispatchExecute(p)); err != nil {
		log.Printf("Warning: DispatchExecute registration failed: %v", err)
	}
	// zhonghuatianyuanquan
	sf, err := capability.NewSensitiveFilter()
	if err == nil {
		_ = p.RegisterCapability(sf)
	} else {
		log.Printf("Warning: SensitiveFilter construction failed: %v", err)
	}
	// RAG store (jinmao) — registry for platform use
	ctx := context.Background()
	registry, _, cleaner, embedder, err := setupRAG(ctx, workspaceDir)
	if err != nil {
		log.Printf("Warning: RAG init failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewContextAssemble()); err != nil {
		log.Printf("Warning: ContextAssemble registration failed: %v", err)
	}
	// Retrieve capability (jinmao) — requires RAG registry + embedder
	if registry != nil && embedder != nil {
		if err := p.RegisterCapability(capability.NewRetrieveCapability(registry, embedder)); err != nil {
			log.Printf("Warning: RetrieveCapability registration failed: %v", err)
		}
	}
	// Load breed configs from JSON files
	breedsDir := os.Getenv("BREEDS_DIR")
	if breedsDir == "" {
		breedsDir = "packs/default/breeds"
	}
	if err := p.LoadFromDir(breedsDir, pack.LoadPolicyFailFast); err != nil {
		log.Printf("Warning: breed load failed: %v", err)
	}
	return p, registry, embedder, cleaner, workspaceDir
}

func loadConfig() *component.ModelConfig {
	return &component.ModelConfig{
		Type:      component.ProviderType(os.Getenv("MODEL_TYPE")),
		BaseURL:   os.Getenv("MODEL_BASE_URL"),
		APIKey:    os.Getenv("MODEL_API_KEY"),
		ModelName: os.Getenv("MODEL_NAME"),
		CLIPath:   os.Getenv("MODEL_CLI_PATH"),
		CLIArgs:   []string{},
	}
}

func getenvDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getenvDefaultInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return defaultVal
}

func setupRAG(ctx context.Context, workspaceDir string) (*ragstore.StoreRegistry, *ragstore.Migrator, *ragstore.RetiredCleaner, embedding.Embedder, error) {
	embedder, err := component.NewOpenAIEmbedder(ctx, component.EmbedConfig{
		APIKey:  os.Getenv("MODEL_API_KEY"),
		BaseURL: os.Getenv("MODEL_BASE_URL"),
		Model:   getenvDefault("EMBEDDING_MODEL", "text-embedding-3-small"),
		Dim:     getenvDefaultInt("EMBEDDING_DIM", 1536),
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

	// Migrator: persists migration log + retired_stores metadata to migration.db.
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

	// RetiredCleaner: drops retired backends after their 30-day retain window.
	cleaner := ragstore.NewRetiredCleaner(registry, 24*time.Hour)
	cleaner.Start()

	// Auto-index docs into RAG store if RAG_AUTO_INDEX=true (default false)
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

func buildMux() http.Handler {
	p := pack.New("default")
	return buildMuxWithHandler(transport.NewWSHandler(p), p, nil, nil, nil, "", time.Now(), nil, ops.NewLogBuffer(1000))
}

func buildMuxWithHandler(wsHandler *transport.WSHandler, p *pack.Pack, pl *platform.Platform, registry *ragstore.StoreRegistry, embedder embedding.Embedder, workspaceDir string, startTime time.Time, evalHandler *transport.EvalHandler, logBuf *ops.LogBuffer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if pl != nil && pl.Ready() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ready","adapters":` + fmt.Sprintf("%d", len(pl.Adapters)) + `,"breeds":` + fmt.Sprintf("%d", len(pl.Breeds)) + `}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not ready"}`))
		}
	})
	mux.HandleFunc("/ws", wsHandler.HandleWS)

	// Create shared components
	eventBus := config.NewEventBus()
	breedsDir := os.Getenv("BREEDS_DIR")
	if breedsDir == "" {
		breedsDir = "packs/default/breeds"
	}

	// Mount Pack API routes
	packAPI := packapi.NewHandler(p, breedsDir)
	packAPI.SetEventBus(eventBus)
	mux.Handle("/api/breeds/", packAPI.Routes())

	// Mount RAG API routes (backend inspect/switch/sync) when registry is available.
	if registry != nil && embedder != nil {
		ragHandler := transport.NewRAGHandler(registry, embedder, workspaceDir)
		mux.Handle("/api/rag/", ragHandler.Routes())
	}

	// Mount Thread + Session API routes (use platform stores if available, else create standalone)
	var threadStore threadstore.ThreadStore
	if pl != nil {
		threadStore = pl.ThreadStore
	} else {
		threadStore, _ = threadstore.NewThreadStore(threadstore.StoreConfig{})
	}
	var threadHandler *transport.ThreadHandler
	if pl != nil && pl.MessageStore != nil {
		threadHandler = transport.NewThreadHandlerWithMessages(threadStore, pl.MessageStore)
	} else {
		threadHandler = transport.NewThreadHandler(threadStore)
	}
	mux.Handle("/api/threads/", threadHandler.Routes())
	mux.Handle("/api/sessions/", threadHandler.Routes())

	// Mount Settings API routes
	var settingsStore settings.SettingsStore
	if pl != nil {
		settingsStore = pl.SettingsStore
	} else {
		settingsStore = settings.NewSettingsStore()
	}
	credStore := settings.NewMemoryCredentialStore()

	settingsHandler := transport.NewSettingsHandlerWithCredentials(settingsStore, credStore, eventBus)
	mux.Handle("/api/settings/", settingsHandler.Routes())

	// Mount Config API routes
	breedLoader := config.NewLoader()
	envPath := filepath.Join(workspaceDir, ".env")
	configHandler := transport.NewConfigHandler(breedLoader, breedsDir, settingsStore, envPath)
	if pl != nil && pl.Leader != nil {
		configHandler.SetLeader(pl.Leader)
	}
	mux.Handle("/api/config/", configHandler.Routes())

	// Mount Rules + Prompt Injection API routes
	var hookReg *hooks.Registry
	if pl != nil {
		hookReg = pl.HookRegistry
	}
	rulesHandler := transport.NewRulesHandler(hookReg, breedLoader, breedsDir, "AGENTS.md")
	mux.Handle("/api/rules", rulesHandler.Routes())
	mux.Handle("/api/prompt-injection/", rulesHandler.Routes())

	// Mount Memory Evidence API routes
	var evidenceStore memory.EvidenceStore
	if pl != nil {
		evidenceStore = pl.EvidenceStore
	} else {
		evidenceStore = memory.NewEvidenceStore()
	}
	memoryHandler := transport.NewMemoryHandler(evidenceStore)
	mux.Handle("/api/memory/", memoryHandler.Routes())

	// Mount Notifications API routes (in-memory store)
	notificationsHandler := transport.NewNotificationsHandler()
	mux.Handle("/api/notifications", notificationsHandler.Routes())
	mux.Handle("/api/notifications/", notificationsHandler.Routes())

	// Mount Files API routes (project file tree)
	filesHandler := transport.NewFilesHandler(workspaceDir)
	mux.Handle("/api/files/", filesHandler.Routes())

	// Mount Panels API routes (concierge, voice, plugins, marketplace, IM)
	panelsHandler := transport.NewPanelsHandler()
	mux.Handle("/api/config/concierge", panelsHandler.Routes())
	mux.Handle("/api/config/voice", panelsHandler.Routes())
	mux.Handle("/api/config/connectors", panelsHandler.Routes())
	mux.Handle("/api/plugins", panelsHandler.Routes())
	mux.Handle("/api/plugins/", panelsHandler.Routes())
	mux.Handle("/api/marketplace", panelsHandler.Routes())

	// Mount Eval API routes
	if evalHandler != nil {
		mux.Handle("/api/evals", evalHandler.Routes())
	}

	// Skills API — list loaded skills
	mux.HandleFunc("/api/skills", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		type skillItem struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		}

		if pl == nil || pl.Skills == nil {
			json.NewEncoder(w).Encode([]skillItem{})
			return
		}
		all := pl.Skills.All()
		items := make([]skillItem, 0, len(all))
		for _, s := range all {
			items = append(items, skillItem{
				Name:   s.ID + ".md",
				Source: "packs/default/skills",
			})
		}
		json.NewEncoder(w).Encode(items)
	})

	// MCP Servers API — list registered MCP servers
	mux.HandleFunc("/api/mcp/servers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		type mcpItem struct {
			Name    string   `json:"name"`
			Tools   []string `json:"tools"`
			Enabled bool     `json:"enabled"`
		}

		if pl == nil || pl.MCP == nil {
			json.NewEncoder(w).Encode([]mcpItem{})
			return
		}
		servers := pl.MCP.All()
		items := make([]mcpItem, 0, len(servers))
		for _, s := range servers {
			items = append(items, mcpItem{
				Name:    s.Name,
				Tools:   []string{},
				Enabled: s.Enabled,
			})
		}
		json.NewEncoder(w).Encode(items)
	})

	// Ops Health API — uptime, goroutines, memory stats, active threads/processes
	mux.HandleFunc("/api/ops/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		otelStatus := "disabled"
		if telemetry.IsInitialized() {
			otelStatus = "ok"
		}

		json.NewEncoder(w).Encode(map[string]any{
			"uptime":           time.Since(startTime).String(),
			"goroutines":       runtime.NumGoroutine(),
			"mem_alloc":        mem.Alloc,
			"mem_total_alloc":  mem.TotalAlloc,
			"mem_sys":          mem.Sys,
			"mem_heap_alloc":   mem.HeapAlloc,
			"mem_heap_sys":     mem.HeapSys,
			"mem_heap_objects": mem.HeapObjects,
			"mem_num_gc":       mem.NumGC,
			"active_threads":   0,
			"active_processes": 0,
			"status":           "ok",
			"otel": map[string]any{
				"status":         otelStatus,
				"tracesEnabled":  telemetry.IsInitialized(),
				"metricsEnabled": telemetry.IsInitialized(),
			},
		})
	})

	// Ops Logs API — last 100 log entries from LogBuffer
	mux.HandleFunc("/api/ops/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		entries := logBuf.Recent(100)
		if entries == nil {
			entries = []ops.LogEntry{}
		}
		json.NewEncoder(w).Encode(entries)
	})

	// Diagnostics Pool API — goroutines, memory, rate monitor, GC stats
	mux.HandleFunc("/api/diagnostics/pool", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		json.NewEncoder(w).Encode(map[string]any{
			"goroutines": runtime.NumGoroutine(),
			"memory": map[string]any{
				"alloc":        mem.Alloc,
				"total_alloc":  mem.TotalAlloc,
				"sys":          mem.Sys,
				"heap_alloc":   mem.HeapAlloc,
				"heap_sys":     mem.HeapSys,
				"heap_objects": mem.HeapObjects,
				"num_gc":       mem.NumGC,
				"gc_pause_ns":  mem.PauseTotalNs,
			},
			"rate_monitor": map[string]any{
				"tracked_sessions": wsHandler.SessionCount(),
			},
			"log_buffer": map[string]any{
				"entries": logBuf.Len(),
			},
			"rag_cache": map[string]any{
				"available": registry != nil,
			},
		})
	})

	// Ops Git API — branch, ahead/behind, dirty/untracked/modified
	mux.HandleFunc("/api/ops/git", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		// branch
		branch := ""
		if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
			branch = strings.TrimSpace(string(out))
		}

		// ahead/behind (relative to upstream; zero on failure)
		ahead, behind := 0, 0
		if out, err := exec.Command("git", "rev-list", "--left-right", "--count", "HEAD...@{u}").Output(); err == nil {
			parts := strings.Fields(strings.TrimSpace(string(out)))
			if len(parts) >= 2 {
				fmt.Sscanf(parts[0], "%d", &ahead)
				fmt.Sscanf(parts[1], "%d", &behind)
			}
		}

		// dirty/untracked/modified via porcelain status
		untracked, modified := 0, 0
		if out, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "??") {
					untracked++
				} else {
					modified++
				}
			}
		}
		dirty := modified > 0 || untracked > 0

		json.NewEncoder(w).Encode(map[string]any{
			"branch":    branch,
			"ahead":     ahead,
			"behind":    behind,
			"dirty":     dirty,
			"untracked": untracked,
			"modified":  modified,
		})
	})

	// Telemetry ops endpoints — metrics, metrics/history, traces
	opsHandler := transport.NewOpsHandler()
	opsHandler.RegisterRoutes(mux)

	// Upgrade info endpoint — returns installation mode
	mux.HandleFunc("/api/upgrade/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		mode := "release"
		if _, err := os.Stat(".git"); err == nil {
			mode = "source"
		}

		version := "v0.1.0"
		if data, err := os.ReadFile("VERSION"); err == nil {
			version = strings.TrimSpace(string(data))
		}

		json.NewEncoder(w).Encode(map[string]any{
			"mode":    mode,
			"version": version,
			"repo":    "https://github.com/rouroumaibing/sounds-great-ai",
		})
	})

	// Upgrade endpoint — detects mode and runs appropriate path
	mux.HandleFunc("/api/upgrade", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Pull bool `json:"pull"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		var logs []string

		// Detect installation mode
		isSource := false
		if _, err := os.Stat(".git"); err == nil {
			isSource = true
		}

		if isSource {
			// Source mode: git pull + rebuild
			if req.Pull {
				out, err := exec.Command("git", "pull").CombinedOutput()
				logs = append(logs, string(out))
				if err != nil {
					json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "git pull failed", "logs": logs})
					return
				}
			}

			out, err := exec.Command("make", "install").CombinedOutput()
			logs = append(logs, string(out))
			if err != nil {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "make install failed", "logs": logs})
				return
			}

			out, err = exec.Command("make", "build").CombinedOutput()
			logs = append(logs, string(out))
			if err != nil {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "make build failed", "logs": logs})
				return
			}

			out, err = exec.Command("go", "build", "-o", "bin/server", "cmd/server/main.go").CombinedOutput()
			logs = append(logs, string(out))
			if err != nil {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "go build failed", "logs": logs})
				return
			}

			json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Upgrade complete (source mode). Restart the server to apply.", "logs": logs})
		} else {
			// Release mode: download latest release binary from GitHub
			platformKey := runtime.GOOS + "-" + runtime.GOARCH
			apiURL := "https://api.github.com/repos/rouroumaibing/sounds-great-ai/releases/latest"

			resp, err := http.Get(apiURL)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to fetch release info: " + err.Error(), "logs": logs})
				return
			}
			defer resp.Body.Close()

			var release struct {
				TagName string `json:"tag_name"`
				Assets  []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				} `json:"assets"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to parse release info: " + err.Error(), "logs": logs})
				return
			}

			logs = append(logs, fmt.Sprintf("Latest release: %s", release.TagName))

			// Find matching asset for current platform
			var downloadURL string
			for _, asset := range release.Assets {
				if strings.Contains(asset.Name, platformKey) {
					downloadURL = asset.BrowserDownloadURL
					break
				}
			}
			if downloadURL == "" {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": fmt.Sprintf("no release asset found for platform %s", platformKey), "logs": logs})
				return
			}

			// Download and replace binary
			dlResp, err := http.Get(downloadURL)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to download release: " + err.Error(), "logs": logs})
				return
			}
			defer dlResp.Body.Close()

			if err := os.MkdirAll("bin", 0755); err != nil {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to create bin dir: " + err.Error(), "logs": logs})
				return
			}

			outFile, err := os.Create("bin/server")
			if err != nil {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to create binary file: " + err.Error(), "logs": logs})
				return
			}
			if _, err := io.Copy(outFile, dlResp.Body); err != nil {
				outFile.Close()
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to write binary: " + err.Error(), "logs": logs})
				return
			}
			outFile.Close()
			os.Chmod("bin/server", 0755)

			logs = append(logs, fmt.Sprintf("Downloaded %s to bin/server", downloadURL))
			json.NewEncoder(w).Encode(map[string]any{"success": true, "message": fmt.Sprintf("Upgrade complete (release mode, %s). Restart the server to apply.", release.TagName), "logs": logs})
		}
	})

	// Serve frontend static files in production (SPA fallback)
	distDir := filepath.Join(workspaceDir, "web", "dist")
	if _, err := os.Stat(distDir); err == nil {
		mux.Handle("/", spaHandler(distDir))
	}

	return telemetry.TraceMiddleware(mux)
}

func spaHandler(distDir string) http.Handler {
	fs := http.FileServer(http.Dir(distDir))
	cleanDistDir := filepath.Clean(distDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		cleanPath := filepath.Join(distDir, filepath.Clean(r.URL.Path))
		if !strings.HasPrefix(cleanPath, cleanDistDir+string(filepath.Separator)) && cleanPath != cleanDistDir {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(cleanPath); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fs.ServeHTTP(w, r)
	})
}
