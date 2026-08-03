package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"sounds-great-ai/internal/agent"
	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/internal/packapi"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/internal/transport"
	"sounds-great-ai/pkg/pack"
)

func main() {
	ctx := context.Background()
	cfg := loadConfig()
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

	// Initialize Pack system
	p, registry, embedder, cleaner, workspaceDir := setupPack()
	defer p.Close()
	if cleaner != nil {
		defer cleaner.Stop()
	}

	wsHandler := transport.NewWSHandler(p)
	mux := buildMuxWithHandler(wsHandler, p, registry, embedder, workspaceDir)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
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
	if err := p.RegisterCapability(capability.NewLLMChat()); err != nil {
		log.Printf("Warning: LLMChat registration failed: %v", err)
	}
	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workspaceDir = wd
		}
	}
	if err := p.RegisterCapability(capability.NewCodeSearch(workspaceDir)); err != nil {
		log.Printf("Warning: CodeSearch registration failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewTaskDecompose()); err != nil {
		log.Printf("Warning: TaskDecompose registration failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewAgentDispatch()); err != nil {
		log.Printf("Warning: AgentDispatch registration failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewResultMerge()); err != nil {
		log.Printf("Warning: ResultMerge registration failed: %v", err)
	}
	// Dispatch executor (framework capability)
	if err := p.RegisterCapability(capability.NewDispatchExecute(p)); err != nil {
		log.Printf("Warning: DispatchExecute registration failed: %v", err)
	}
	// xigou chain
	if err := p.RegisterCapability(capability.NewCodeAnalyze()); err != nil {
		log.Printf("Warning: CodeAnalyze registration failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewRefactorSuggest()); err != nil {
		log.Printf("Warning: RefactorSuggest registration failed: %v", err)
	}
	// zangao chain
	if err := p.RegisterCapability(capability.NewFormatOutput()); err != nil {
		log.Printf("Warning: FormatOutput registration failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewRenderMarkdown()); err != nil {
		log.Printf("Warning: RenderMarkdown registration failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewStreamResponse()); err != nil {
		log.Printf("Warning: StreamResponse registration failed: %v", err)
	}
	// demu chain
	if err := p.RegisterCapability(capability.NewLogTrace()); err != nil {
		log.Printf("Warning: LogTrace registration failed: %v", err)
	}
	if err := p.RegisterCapability(capability.NewErrorDiagnose()); err != nil {
		log.Printf("Warning: ErrorDiagnose registration failed: %v", err)
	}
	// zhonghuatianyuanquan
	sf, err := capability.NewSensitiveFilter()
	if err == nil {
		_ = p.RegisterCapability(sf)
	} else {
		log.Printf("Warning: SensitiveFilter construction failed: %v", err)
	}
	// RAG capabilities (jinmao)
	ctx := context.Background()
	registry, _, cleaner, embedder, err := setupRAG(ctx, workspaceDir)
	if err != nil {
		log.Printf("Warning: RAG init failed, rag_* capabilities unavailable: %v", err)
	} else {
		if err := p.RegisterCapability(capability.NewRagSearch(registry)); err != nil {
			log.Printf("Warning: RagSearch registration failed: %v", err)
		}
		if err := p.RegisterCapability(capability.NewRagIndex(registry)); err != nil {
			log.Printf("Warning: RagIndex registration failed: %v", err)
		}
		if err := p.RegisterCapability(capability.NewContextAssemble()); err != nil {
			log.Printf("Warning: ContextAssemble registration failed: %v", err)
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

	return registry, migrator, cleaner, embedder, nil
}

func buildMux() *http.ServeMux {
	return buildMuxWithHandler(transport.NewWSHandler(pack.New("default")), pack.New("default"), nil, nil, "")
}

func buildMuxWithHandler(wsHandler *transport.WSHandler, p *pack.Pack, registry *ragstore.StoreRegistry, embedder embedding.Embedder, workspaceDir string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/ws", wsHandler.HandleWS)

	// Mount Pack API routes
	packAPI := packapi.NewHandler(p, "packs/default/breeds")
	mux.Handle("/api/breeds/", packAPI.Routes())

	// Mount RAG API routes (backend inspect/switch/sync) when registry is available.
	if registry != nil && embedder != nil {
		ragHandler := transport.NewRAGHandler(registry, embedder, workspaceDir)
		mux.Handle("/api/rag/", ragHandler.Routes())
	}

	return mux
}
