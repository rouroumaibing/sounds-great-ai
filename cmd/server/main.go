package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sounds-great-ai/internal/agent"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/internal/mcp"
	"sounds-great-ai/internal/ops"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/telemetry"
	"sounds-great-ai/internal/transport"
)

func main() {
	ctx := context.Background()
	startTime := time.Now()
	cfg := loadConfig()

	logBuf := ops.NewLogBuffer(1000)
	log.SetOutput(ops.NewLogWriter(os.Stderr, logBuf))

	telemetryCleanup, telemetryErr := telemetry.Init()
	if telemetryErr != nil {
		log.Printf("Warning: telemetry init failed: %v", telemetryErr)
	}
	defer telemetryCleanup()
	defer telemetry.Shutdown()

	_, err := component.NewChatModel(ctx, cfg)
	if err != nil {
		log.Printf("Warning: model init failed (server still starts): %v", err)
	}
	sm := &agent.SkillManager{}
	skillDir := getenvDefault("SKILL_DIR", "internal/agent/skills")
	if err := sm.Load(skillDir); err != nil {
		log.Printf("Warning: skill load failed: %v", err)
	}

	workspaceDir := getenvDefault("WORKSPACE_DIR", "")
	if workspaceDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workspaceDir = wd
		}
	}
	breedsDir := getenvDefault("BREEDS_DIR", "packs/default/breeds")
	skillsDir := getenvDefault("SKILLS_DIR", "packs/default/skills")
	sqlitePath := getenvDefault("SQLITE_PATH", "data/sounds-great.db")

	pl, err := platform.New(platform.Config{
		BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: workspaceDir, SQLitePath: sqlitePath,
	})
	if err != nil {
		log.Printf("Warning: platform init failed (server runs in legacy mode): %v", err)
	}

	p, registry, embedder, cleaner, _ := setupPack()
	defer p.Close()
	if cleaner != nil {
		defer cleaner.Stop()
	}

	if registry != nil && pl != nil {
		pl.RAGRegistry = registry
		pl.Embedder = embedder
		mcpServerPath := filepath.Join(workspaceDir, "bin", "sounds-great-mcp-server")
		ragDBPath := filepath.Join(workspaceDir, "rag_index.db")
		pl.MCP.Register("knowledge", &mcp.MCPServerConfig{
			Name: "knowledge", Command: mcpServerPath, Args: []string{"--db", ragDBPath}, Enabled: true,
		})
	}

	wsHandler := transport.NewWSHandler(p)
	if pl != nil {
		wsHandler = transport.NewWSHandlerWithPlatform(p, pl)
	}

	evalScheduler, evalHandler := setupEval(ctx, pl, workspaceDir)

	mux := buildMuxWithHandler(wsHandler, p, pl, registry, embedder, workspaceDir, startTime, evalHandler, logBuf)

	port := getenvDefault("PORT", "8080")
	srv := &http.Server{Addr: ":" + port, Handler: mux}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Printf("Server starting on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	burnRateMonitor, monitorCancel := setupBurnRateMonitor(wsHandler)

	sig := <-sigCh
	log.Printf("Received signal %s, shutting down...", sig)

	monitorCancel()
	burnRateMonitor.Stop()
	log.Printf("Burn-Rate Monitor stopped")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
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
