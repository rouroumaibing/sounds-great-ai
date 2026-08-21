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

	"sounds-great-ai/internal/component"
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
	port := GetenvDefault("PORT", "8080")

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
		// Seed the builtin RAG "knowledge" server into the persistent store.
		// Operators can add/remove/toggle their own servers via the MCP panel;
		// the builtin entry is owned by the platform and shown read-only.
		if pl.MCPStore != nil {
			pl.MCPStore.SeedKnowledge(mcpServerPath, []string{"--db", ragDBPath})

			// Seed the builtin "platform" MCP server — the platform-as-MCP-server
			// surface that exposes collab/memory/people/roster/breeds capabilities
			// to CLI agents by proxying the SG REST API. It connects back to this
			// server's own loopback address; the auth token (if any) is passed via
			// env so the subprocess can authenticate (dev mode = auth disabled).
			platformPath := filepath.Join(workspaceDir, "bin", "sounds-great-platform-mcp-server")
			port = GetenvDefault("PORT", "8080")
			apiBase := "http://localhost:" + port
			platformEnv := map[string]string{}
			if tok := os.Getenv("AUTH_TOKEN"); tok != "" {
				platformEnv["SG_API_TOKEN"] = tok
			}
			// CallbackURL is the HTTP fallback: when the MCP stdio transport is
			// unavailable, the agent calls the SG REST API directly at this
			// loopback address.
			pl.MCPStore.SeedPlatform(platformPath, []string{"--api-base", apiBase}, platformEnv, nil, apiBase)
		}
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
