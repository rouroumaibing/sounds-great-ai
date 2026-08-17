package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"sounds-great-ai/internal/agent"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/internal/mcp"
	"sounds-great-ai/internal/ops"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/sop"
	"sounds-great-ai/internal/telemetry"
	"sounds-great-ai/internal/transport"
)

func main() {
	ctx := context.Background()
	startTime := time.Now()
	cfg := LoadConfig()

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
	skillDir := GetenvDefault("SKILL_DIR", "internal/agent/skills")
	if err := sm.Load(skillDir); err != nil {
		log.Printf("Warning: skill load failed: %v", err)
	}

	workspaceDir := GetenvDefault("WORKSPACE_DIR", "")
	if workspaceDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workspaceDir = wd
		}
	}
	breedsDir := GetenvDefault("BREEDS_DIR", "packs/default/breeds")
	skillsDir := GetenvDefault("SKILLS_DIR", "packs/default/skills")
	sqlitePath := GetenvDefault("SQLITE_PATH", "data/sounds-great.db")
	redisURL := GetenvDefault("SG_REDIS_URL", "")

	pl, err := platform.New(platform.Config{
		BreedsDir: breedsDir, SkillsDir: skillsDir, WorkspaceDir: workspaceDir, SQLitePath: sqlitePath, RedisURL: redisURL,
	})
	if err != nil {
		log.Printf("Warning: platform init failed (server runs in legacy mode): %v", err)
	}

	// Per-provider long session: under -tags pty this wires warm pools
	// (bg_daemon) for claude/codex/gemini; under the default build it is a
	// no-op stub and all three transparently fall back to one-shot print_sdk.
	if pl != nil {
		pl.WireWarmPools()
	}

	p, registry, embedder, cleaner, _ := SetupPack()
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

	evalScheduler, evalHandler := SetupEval(ctx, pl, workspaceDir)

	// QC auto-runner (server-side auto-trigger): periodically runs the QC loop
	// and exposes status via /api/qc/*. Mirrors the eval scheduler pattern
	// (SetupEval -> scheduler.Start) so QC is no longer only a dev-run `make qc`.
	qcSkipHeavy := GetenvDefault("QC_AUTO_SKIP_HEAVY", "true") != "false"
	qcRunner := sop.NewAutoRunner(workspaceDir, qcSkipHeavy)

	mux := BuildMuxWithHandler(wsHandler, p, pl, registry, embedder, workspaceDir, startTime, evalHandler, logBuf, qcRunner)

	port := GetenvDefault("PORT", "8080")
	srv := &http.Server{Addr: ":" + port, Handler: mux}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Printf("Server starting on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Ball-custody zombie reconciler (P1): heals dangling invocations that
	// never wrote a terminal event (crashed CLI agent / leaked fiber).
	if pl != nil {
		go pl.StartReconciler(ctx)
	}

	burnRateMonitor, monitorCancel := setupBurnRateMonitor(wsHandler)

	// QC auto-runner periodic sweep. A separate cancelable context (not the
	// shared ctx used by SetupEval) keeps shutdown isolated: cancelling it stops
	// the ticker goroutine without touching the eval scheduler.
	if interval := parseQCInterval(); interval > 0 {
		qcCtx, qcCancel := context.WithCancel(ctx)
		go qcRunner.Start(qcCtx, interval)
		defer qcCancel()
	}

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

// parseQCInterval reads the QC auto-runner period from QC_AUTO_INTERVAL.
// Recognised values: a Go duration (e.g. "30m", "1h"), or "off"/"0"/"false" to
// disable the periodic sweep (QC then runs only on demand via POST /api/qc/run).
// Defaults to 30m.
func parseQCInterval() time.Duration {
	v := GetenvDefault("QC_AUTO_INTERVAL", "30m")
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "off", "0", "false", "":
		return 0
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return 30 * time.Minute
}
