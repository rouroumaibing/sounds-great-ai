package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"sounds-great-ai/internal/agent"
	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/internal/packapi"
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
	p := setupPack()
	defer p.Close()

	wsHandler := transport.NewWSHandler(p)
	mux := buildMuxWithHandler(wsHandler, p)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func setupPack() *pack.Pack {
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
	// Load breed configs from JSON files
	breedsDir := os.Getenv("BREEDS_DIR")
	if breedsDir == "" {
		breedsDir = "packs/default/breeds"
	}
	if err := p.LoadFromDir(breedsDir, pack.LoadPolicyFailFast); err != nil {
		log.Printf("Warning: breed load failed: %v", err)
	}
	return p
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

func buildMux() *http.ServeMux {
	return buildMuxWithHandler(transport.NewWSHandler(pack.New("default")), pack.New("default"))
}

func buildMuxWithHandler(wsHandler *transport.WSHandler, p *pack.Pack) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/ws", wsHandler.HandleWS)

	// Mount Pack API routes
	packAPI := packapi.NewHandler(p, "packs/default/breeds")
	mux.Handle("/api/breeds/", packAPI.Routes())

	return mux
}
