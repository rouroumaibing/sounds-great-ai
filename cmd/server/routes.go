package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/config"
	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	custodyServices "sounds-great-ai/internal/domains/custody/services"
	"sounds-great-ai/internal/hooks"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/ops"
	"sounds-great-ai/internal/packapi"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/sop"
	"sounds-great-ai/internal/telemetry"
	"sounds-great-ai/internal/threadstore"
	threadPorts "sounds-great-ai/internal/domains/threads/ports"
	threadStores "sounds-great-ai/internal/domains/threads/stores"
	"sounds-great-ai/internal/transport"
	"sounds-great-ai/pkg/pack"

	"github.com/cloudwego/eino/components/embedding"
)

func BuildMux() http.Handler {
	p := pack.New("default")
	return BuildMuxWithHandler(transport.NewWSHandler(p), p, nil, nil, nil, "", time.Now(), nil, ops.NewLogBuffer(1000), nil)
}

func BuildMuxWithHandler(wsHandler *transport.WSHandler, p *pack.Pack, pl *platform.Platform, registry *ragstore.StoreRegistry, embedder embedding.Embedder, workspaceDir string, startTime time.Time, evalHandler *transport.EvalHandler, logBuf *ops.LogBuffer, qcRunner *sop.AutoRunner) http.Handler {
	auth := transport.NewAuthMiddleware()
	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	cors := transport.CORSMiddleware(allowedOrigin)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if pl != nil && pl.Ready() {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status":   "ready",
				"adapters": pl.AgentExecutor.Count(),
				"breeds":   len(pl.Breeds),
			})
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
		}
	})
	mux.HandleFunc("GET /ws", wsHandler.HandleWS)

	settingsDir := settings.ConfigRoot(workspaceDir)
	var settingsStore settings.SettingsStore
	if pl != nil {
		settingsStore = pl.SettingsStore
	} else {
		settingsStore = settings.NewFileSettingsStore(
			filepath.Join(settingsDir, settings.AccountsFileName),
			filepath.Join(settingsDir, settings.CatalogFileName),
			true,
		)
	}

	eventBus := config.NewEventBus()
	breedsDir := os.Getenv("BREEDS_DIR")
	if breedsDir == "" {
		breedsDir = "packs/default/breeds"
	}

	packAPI := packapi.NewHandler(p, breedsDir, settingsStore)
	packAPI.SetEventBus(eventBus)
	mux.Handle("/api/breeds", auth.Wrap(packAPI.Routes()))
	mux.Handle("/api/breeds/", auth.Wrap(packAPI.Routes()))

	if registry != nil && embedder != nil {
		ragHandler := transport.NewRAGHandler(registry, embedder, workspaceDir)
		mux.Handle("/api/rag/", ragHandler.Routes())
	} else {
		mux.HandleFunc("GET /api/rag/backend", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"active":   "none",
				"retirees": []map[string]any{},
				"error":    "RAG not initialized (embedding API key required)",
			})
		})
	}

	var threadStore threadPorts.IThreadStore
	if pl != nil {
		threadStore = pl.ThreadStore
	} else {
		ts, _ := threadstore.NewThreadStore(threadstore.StoreConfig{})
		threadStore = threadStores.NewThreadStoreAdapter(ts)
	}
	var threadHandler *transport.ThreadHandler
	if pl != nil && pl.MessageStore != nil {
		threadHandler = transport.NewThreadHandlerWithMessages(threadStore, pl.MessageStore)
	} else {
		threadHandler = transport.NewThreadHandler(threadStore)
	}
	mux.Handle("/api/threads", threadHandler.Routes())
	mux.Handle("/api/threads/", threadHandler.Routes())
	mux.Handle("/api/sessions/", threadHandler.Routes())

	// Credentials live in the global home directory (CredentialRoot) per the
	// customer-safety layout, independent of the project-local config root.
	credStore := settings.NewFileCredentialStore(filepath.Join(settings.CredentialRoot(), settings.CredentialsFileName), true)
	settingsHandler := transport.NewSettingsHandlerWithCredentials(settingsStore, credStore, eventBus)
	mux.Handle("/api/settings/", auth.Wrap(settingsHandler.Routes()))

	breedLoader := pack.NewLoader()
	envPath := filepath.Join(workspaceDir, ".env")
	configHandler := transport.NewConfigHandler(breedLoader, breedsDir, settingsStore, envPath)
	if pl != nil && pl.Leader != nil {
		configHandler.SetLeader(pl.Leader)
	}
	mux.Handle("/api/config/", auth.Wrap(configHandler.Routes()))

	var hookReg *hooks.Registry
	if pl != nil {
		hookReg = pl.HookRegistry
	}
	rulesHandler := transport.NewRulesHandler(hookReg, breedLoader, breedsDir, "AGENTS.md")
	mux.Handle("/api/rules", rulesHandler.Routes())
	mux.Handle("/api/prompt-injection/", rulesHandler.Routes())

	var evidenceStore memory.EvidenceStore
	if pl != nil {
		evidenceStore = pl.EvidenceStore
	} else {
		evidenceStore = memory.NewEvidenceStore()
	}
	memoryHandler := transport.NewMemoryHandler(evidenceStore)
	mux.Handle("/api/memory/", memoryHandler.Routes())

	// Persistent Identity P1-b (relationship capsule CRUD + Approval Hub) and
	// P3 rotation-aware continuity inspection. Both depend on the platform's
	// on-disk stores, so they are mounted only when the platform is initialized.
	if pl != nil && pl.Profiles != nil {
		profilesHandler := transport.NewProfilesHandler(pl.Profiles, pl.Continuity, pl.EvidenceStore, pl.AgentExecutor, pl.WorkspaceDir, pl)
		mux.Handle("/api/profiles", auth.Wrap(profilesHandler.Routes()))
		mux.Handle("/api/profiles/", auth.Wrap(profilesHandler.Routes()))
		// Wire the capsule handler into the WS handler so session seal can fire
		// a best-effort autonomous distill (KD-10 F276 maturity trigger).
		wsHandler.SetProfilesHandler(profilesHandler)
	}
	if pl != nil && pl.Continuity != nil {
		continuityHandler := transport.NewContinuityHandler(pl.Continuity)
		mux.Handle("/api/continuity", continuityHandler.Routes())
		mux.Handle("/api/continuity/", continuityHandler.Routes())
	}
	if pl != nil && pl.PeopleMemory != nil {
		pmOperator := "operator"
		if pl.Leader != nil && pl.Leader.Name != "" {
			pmOperator = pl.Leader.Name
		}
		peopleHandler := transport.NewPeopleMemoryHandler(pl.PeopleMemory, pmOperator,
			transport.NewThreadstoreAuthorizer(pl.ThreadStore, pl.MessageStore), pl.PeopleMemoryHub)
		mux.Handle("/api/people-memory", auth.Wrap(peopleHandler.Routes()))
		mux.Handle("/api/people-memory/", auth.Wrap(peopleHandler.Routes()))
	}

	// Persistent Identity (Shared Memory): typed-lane registry disposition +
	// truth recall. Pending candidates are produced at session close (P2) and
	// disposed here by a human (M5 提交权); approved truth is recalled into dog
	// prompts (P4). Mounted under /api/memory/lanes so it shares the evidence
	// store's namespace; the more specific /api/memory/lanes/ subtree wins over
	// the /api/memory/ evidence handler for these paths (Go 1.22 ServeMux).
	if pl != nil && pl.SharedMemory != nil {
		lanesOperator := "operator"
		if pl.Leader != nil && pl.Leader.Name != "" {
			lanesOperator = pl.Leader.Name
		}
		// P2-6 LLM reflection: opt-in synthesis service (irreversible-decisions
		// §4.8). Built from SG_REFLECT_* env; nil when unset so the platform
		// stays deterministic and the endpoint degrades to a clear 501.
		var lanesReflector transport.MemoryReflector
		if chat, rerr := capability.NewReflectModelFromEnv(context.Background()); rerr == nil && chat != nil {
			lanesReflector = capability.NewMemoryReflect(chat)
		}
		lanesHandler := transport.NewLanesHandler(pl.SharedMemory, pl.LaneDispositions, pl.LaneRecall, lanesOperator, lanesReflector)
		// Gap3 semantic recall: opt-in embedding model (SG_EMBED_API_KEY). Nil
		// when unset so semantic search degrades to 501 and lexical FTS5 stays.
		if emb, eerr := capability.NewEmbedModelFromEnv(context.Background()); eerr == nil && emb != nil {
			lanesHandler.SetEmbedder(capability.NewMemoryEmbed(emb))
		}
		// P1 hybrid RRF embed mode (off/shadow/on), homologous clowder
		// EmbedConfig.embedMode. SG_EMBED_MODE overrides; otherwise the registry
		// default (on when a vector store exists) applies.
		if mode := os.Getenv("SG_EMBED_MODE"); mode != "" {
			pl.SharedMemory.SetEmbedMode(mode)
		}
		mux.Handle("/api/memory/lanes", auth.Wrap(lanesHandler.Routes()))
		mux.Handle("/api/memory/lanes/", auth.Wrap(lanesHandler.Routes()))
	}

	notificationsHandler := transport.NewNotificationsHandler()
	mux.Handle("/api/notifications", notificationsHandler.Routes())
	mux.Handle("/api/notifications/", notificationsHandler.Routes())

	filesHandler := transport.NewFilesHandler(workspaceDir)
	mux.Handle("/api/files/", filesHandler.Routes())

	panelsHandler := transport.NewPanelsHandler()
	mux.Handle("/api/config/concierge", panelsHandler.Routes())
	mux.Handle("/api/config/voice", panelsHandler.Routes())
	mux.Handle("/api/config/connectors", panelsHandler.Routes())
	mux.Handle("/api/plugins", panelsHandler.Routes())
	mux.Handle("/api/plugins/", panelsHandler.Routes())
	mux.Handle("/api/marketplace", panelsHandler.Routes())

	if evalHandler != nil {
		mux.Handle("/api/evals", evalHandler.Routes())
	}

	// P2 hold_ball webhook: POST /api/custody/holds/{threadID}/webhook?token=XXX
	// releases a parked hold and resumes the holder (D3 scope: webhook wake).
	// G3: wrapped with operator-level auth so an external party still needs the
	// AUTH_TOKEN (in addition to the per-hold wake token).
	mux.HandleFunc("POST /api/custody/holds/", auth.WrapFunc(CustodyWakeHandler(wsHandler, pl)))

	// P4 Brief & Trail API: GET /api/custody/threads/{threadID}/trail projects
	// the custody ledger into a briefing (D5 engine; UI deferred to P5).
	mux.HandleFunc("GET /api/custody/threads/", CustodyTrailHandler(pl))

	// G6: cross-thread duty briefing (operations view). Snapshot every thread's
	// custody state into needsUser / deadBalls / voidPasses / staleBlocked.
	mux.HandleFunc("GET /api/custody/briefing", auth.WrapFunc(CustodyDutyBriefingHandler(pl)))

	// G8: project archive source — code-repo trajectory timeline + test endpoint.
	repoHandler := transport.NewRepoTrajectoryHandler(pl)
	mux.Handle("/api/repo/", repoHandler.Routes())

	mux.HandleFunc("GET /api/skills", SkillsHandler(pl))
	mux.HandleFunc("GET /api/mcp/servers", MCPServersHandler(pl))
	mux.HandleFunc("GET /api/ops/health", OpsHealthHandler(startTime))
	mux.HandleFunc("GET /api/ops/logs", OpsLogsHandler(logBuf))
	mux.HandleFunc("GET /api/diagnostics/pool", DiagnosticsPoolHandler(wsHandler, logBuf, registry))
	mux.HandleFunc("GET /api/ops/git", auth.WrapFunc(GitStatusHandler()))

	// QC auto-runner: server-side auto-trigger of the QC loop. Status is a
	// read-only heartbeat snapshot; run triggers an on-demand pass (use
	// ?heavy=1 to also run the heavy build/test step).
	if qcRunner != nil {
		mux.HandleFunc("GET /api/qc/status", auth.WrapFunc(QCStatusHandler(qcRunner)))
		mux.HandleFunc("POST /api/qc/run", auth.WrapFunc(QCRunHandler(qcRunner)))
	}

	opsHandler := transport.NewOpsHandler()
	opsHandler.RegisterRoutes(mux)

	mux.HandleFunc("GET /api/upgrade/info", auth.WrapFunc(UpgradeInfoHandler))
	mux.HandleFunc("POST /api/upgrade", auth.WrapFunc(UpgradeHandler))

	distDir := filepath.Join(workspaceDir, "web", "dist")
	if _, err := os.Stat(distDir); err == nil {
		mux.Handle("/", SPAHandler(distDir))
	}

	return telemetry.TraceMiddleware(cors(mux))
}

func SkillsHandler(pl *platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			items = append(items, skillItem{Name: s.ID + ".md", Source: "packs/default/skills"})
		}
		json.NewEncoder(w).Encode(items)
	}
}

func MCPServersHandler(pl *platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			items = append(items, mcpItem{Name: s.Name, Tools: []string{}, Enabled: s.Enabled})
		}
		json.NewEncoder(w).Encode(items)
	}
}

func OpsHealthHandler(startTime time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	}
}

func OpsLogsHandler(logBuf *ops.LogBuffer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		entries := logBuf.Recent(100)
		if entries == nil {
			entries = []ops.LogEntry{}
		}
		json.NewEncoder(w).Encode(entries)
	}
}

func DiagnosticsPoolHandler(wsHandler *transport.WSHandler, logBuf *ops.LogBuffer, registry *ragstore.StoreRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		json.NewEncoder(w).Encode(map[string]any{
			"goroutines": runtime.NumGoroutine(),
			"memory": map[string]any{
				"alloc": mem.Alloc, "total_alloc": mem.TotalAlloc, "sys": mem.Sys,
				"heap_alloc": mem.HeapAlloc, "heap_sys": mem.HeapSys,
				"heap_objects": mem.HeapObjects, "num_gc": mem.NumGC, "gc_pause_ns": mem.PauseTotalNs,
			},
			"rate_monitor": map[string]any{"tracked_sessions": wsHandler.SessionCount()},
			"log_buffer":   map[string]any{"entries": logBuf.Len()},
			"rag_cache":    map[string]any{"available": registry != nil},
		})
	}
}

// CustodyWakeHandler releases a parked hold_ball via an external webhook POST
// (D3 scope: webhook wake kind). Path: /api/custody/holds/{threadID}/webhook,
// with the shared secret passed as ?token=XXX. On success it resumes the holder
// breed over the existing WS session.
func CustodyWakeHandler(wsHandler *transport.WSHandler, pl *platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if pl == nil || pl.HoldScheduler == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": "hold scheduler unavailable"})
			return
		}
		// Path pattern: /api/custody/holds/{threadID}/webhook
		rest := strings.TrimPrefix(r.URL.Path, "/api/custody/holds/")
		rest = strings.TrimSuffix(rest, "/webhook")
		threadID := strings.Trim(rest, "/")
		if threadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing thread id"})
			return
		}
		token := r.URL.Query().Get("token")
		if err := wsHandler.ResumeHeldThread(r.Context(), threadID, custodyPorts.WakeWebhook, token); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, custodyServices.ErrNoActiveHold) {
				status = http.StatusNotFound
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"status": "resumed", "thread_id": threadID})
	}
}

// CustodyTrailHandler projects the custody ledger for a thread into a briefing
// (D5: Brief & Trail API engine). Path: /api/custody/threads/{threadID}/trail.
func CustodyTrailHandler(pl *platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if pl == nil || pl.BallLedger == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": "ledger unavailable"})
			return
		}
		// Path pattern: /api/custody/threads/{threadID}/trail
		rest := strings.TrimPrefix(r.URL.Path, "/api/custody/threads/")
		threadID := strings.TrimSuffix(rest, "/trail")
		threadID = strings.Trim(threadID, "/")
		if threadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing thread id"})
			return
		}
		briefing, err := pl.BallLedger.ProjectTrail(r.Context(), threadID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		// G14: fold the code-repo git-ref trajectory into the thread's unified
		// timeline so the custody trail and code activity share one axis. When no
		// repo URL is configured the unified list equals the custody trail.
		unified := []custodyPorts.UnifiedTrailEntry{}
		if pl.RepoTrajectoryStore != nil {
			unified = custodyServices.MergeUnifiedTrail(briefing, pl.RepoTrajectoryStore.List())
		}
		json.NewEncoder(w).Encode(map[string]any{
			"thread_id": briefing.ThreadID,
			"state":     briefing.State,
			"holder":    briefing.Holder,
			"turns":     briefing.Turns,
			"handoffs":  briefing.Handoffs,
			"holds":     briefing.Holds,
			"trail":     briefing.Trail,
			"unified":   unified,
		})
	}
}

// CustodyDutyBriefingHandler aggregates every thread's custody state into an
// operations view (G6). Path: GET /api/custody/briefing.
func CustodyDutyBriefingHandler(pl *platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if pl == nil || pl.BallLedger == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": "ledger unavailable"})
			return
		}
		briefing, err := pl.BallLedger.ProjectDutyBriefing(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(briefing)
	}
}

func GitStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		branch := ""
		if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
			branch = strings.TrimSpace(string(out))
		}
		ahead, behind := 0, 0
		if out, err := exec.Command("git", "rev-list", "--left-right", "--count", "HEAD...@{u}").Output(); err == nil {
			parts := strings.Fields(strings.TrimSpace(string(out)))
			if len(parts) >= 2 {
				fmt.Sscanf(parts[0], "%d", &ahead)
				fmt.Sscanf(parts[1], "%d", &behind)
			}
		}
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
		json.NewEncoder(w).Encode(map[string]any{
			"branch": branch, "ahead": ahead, "behind": behind,
			"dirty": modified > 0 || untracked > 0, "untracked": untracked, "modified": modified,
		})
	}
}

// QCStatusHandler returns the latest auto-run snapshot plus the persisted QC
// state and (when available) the aggregated eval:qc telemetry.
func QCStatusHandler(runner *sop.AutoRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		last, lastRun, lastErr := runner.Last()
		state := sop.LoadQCState(runner.StatePath())
		out := map[string]any{
			"passed":              last.Passed,
			"risk_tier":           last.RiskTier,
			"stale":               last.Stale,
			"reviewed_sha":        last.ReviewedSha,
			"steps":               last.Steps,
			"last_run":            lastRun.UTC().Format(time.RFC3339),
			"last_error":          lastErr,
			"state_phase":         state.Phase,
			"state_reviewed_sha":  state.ReviewedSha,
		}
		if agg, err := sop.AggregateQCMetrics(runner.MetricsPath()); err == nil {
			out["aggregate"] = agg
		}
		json.NewEncoder(w).Encode(out)
	}
}

// QCRunHandler triggers an on-demand QC pass. ?heavy=1 also runs the heavy
// build/test step; otherwise it respects the runner's skipHeavy default.
func QCRunHandler(runner *sop.AutoRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		forceHeavy := r.URL.Query().Get("heavy") == "1"
		result := runner.RunNow(forceHeavy)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}
}
