<div align="center">

# Sounds Great AI

**When AI Agents Bark Together, It Sounds Great.**

*Every bark is a precise coordination.*

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Eino](https://img.shields.io/badge/Eino-v0.9+-blueviolet)](https://github.com/cloudwego/eino)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[中文](README_zh-CN.md) | **English**

</div>

---

## Why Sounds Great AI?

You have a codebase. It's large, complex, and accumulating technical debt every day.

You have Claude, GPT, Gemini — powerful models, each with unique strengths. But making them collaborate means **you** become the router: copying context between chat windows, manually tracking who said what, wasting time on middle management.

> *"I don't need a pack of lone wolves. I need a tightly coordinated team."*
> *"Like a dog agent squad — loyal, clearly specialized, one command and everyone moves."*

So under the Go language and Eino orchestration engine, **Sounds Great AI** was born.

This isn't just another Agent invocation framework. It's a **Pack** — a squad of dogs, each with their own role, personality, and capabilities, communicating via A2A protocol, working together in DAG workflows.

> *When the Agents perfectly complete a collaboration, the terminal lights up with green paw prints:*
> **`Sounds Great!`**

## What It Does

| Capability | What It Means |
|------------|---------------|
| **Config-Driven Role System** | Breed roles are pure JSON data — create/modify/delete dogs on the page, hot-reload takes effect instantly, no restart needed |
| **A2A Multi-Agent Collaboration** | Async inter-Agent communication with @mention routing, thread isolation, SSE streaming, structured handoffs |
| **DAG Workflow Engine** | Each breed defines its own workflow (topological sort + parallel execution + dependency passing) — not simple linear calls |
| **Hard Rails Safety** | Command blocklist, path validation, sandbox isolation — safety enforced by code, not prompts |
| **Capability Adapters** | Thin adapter layer wrapping existing code — add new capabilities by implementing an interface + registering, no existing code changes |
| **Hot Reload** | Register new breeds at runtime → instant effect; file watcher + HTTP API dual path |
| **Eino Framework Integration** | Based on CloudWeGo Eino's ChatModel interface, supports OpenAI / Azure / local models |

## The Pack — Breed Role Mapping

Six dogs, six roles, each with its own specialty:

| Role | Breed | Personality | Core Responsibilities | Capabilities |
|------|-------|-------------|----------------------|-------------|
| **Orchestrator** | Border Collie *(bianmu)* | Extremely intelligent, field-control master, sharp-eyed | Task decomposition, DAG workflow scheduling, state machine control | task_decompose, agent_dispatch, result_merge |
| **Safety Guardrail** | Chinese Rural Dog *(zhonghuatianyuanquan)* | Loyal, reliable, highly alert, knows home terrain | Safety boundaries, command blocklist, permission auditing | command_check, path_validate, sensitive_filter |
| **UI / CLI Presentation** | Tibetan Mastiff *(zangao)* | Majestic, imposing, steadfast, gatekeeper | TUI status rendering, log dashboard, human confirmation | format_output, render_markdown, stream_response |
| **Code Hunter** | Xigou *(xigou)* | Streamlined, lightning-fast, laser-focused | Automated Refactor, high-difficulty Bug fix code generation | code_search, code_analyze, refactor_suggest |
| **RAG / Retriever** | Golden Retriever *(jinmao)* | Strong retrieval instinct, gentle, dependable | Vector search, context fetching, document association | rag_search, rag_index, context_assemble |
| **Log & Bug Tracer** | German Shepherd *(demu)* | Alert, black-backed, upright ears, strong execution | Panic tracking, StackTrace analysis, Log tracing | log_trace, error_diagnose, performance_profile |

> Users can create their own dogs — just one JSON file, select registered capabilities, define a workflow, and hot-reload takes effect instantly.

## Architecture

```
┌───────────────────────────────────────────────────┐
│                 packs/default/breeds/               │
│   bianmu.json  zhonghuatianyuanquan.json  zangao.json │
│   xigou.json   jinmao.json              demu.json      │
│          (pure data, user-created/modifiable/hot-reload)            │
└──────────────────────┬────────────────────────────┘
                       │ LoadFromDir / POST API
                       ▼
┌───────────────────────────────────────────────────┐
│                 Pack (Coordinator)                  │
│   registry: map[string]*BreedConfig                 │
│   capabilities: map[string]Capability               │
│   mu: sync.RWMutex                                  │
│                                                     │
│   Bark(breedID, input) → DAG topological sort → parallel execution  │
└──────────────────────┬────────────────────────────┘
                       │ Capability.Run()
                       ▼
┌───────────────────────────────────────────────────┐
│            internal/capability/ (adapters)          │
│   CommandCheck  PathValidate  StreamResponse        │
│   CodeSearch    TaskDecompose  ...                  │
│   (thin adapters, wrapping internal/ existing code) │
└──────────────────────┬────────────────────────────┘
                       │ calls
                       ▼
┌───────────────────────────────────────────────────┐
│            internal/ (existing code, unchanged)     │
│   aspect/  transport/  agent/  tool/  workspace/    │
│   a2a/  component/                                  │
└───────────────────────────────────────────────────┘
```

**Three-Layer Separation Principle:**

| Layer | Responsible For | Not Responsible For |
|-------|----------------|---------------------|
| **Breed JSON (Data)** | Role identity, personality, model config, capability declarations, workflow definitions | Code logic |
| **Capability (Go Code)** | Specific capability implementation, wrapping existing internal/ code | Role definition, task scheduling |
| **Pack (Coordinator)** | Registry management, DAG scheduling, hot reload, safety validation | Specific capability implementation |

> *Roles are data, capabilities are code. Users define "who", the system decides "how".*

## Quick Start

### Prerequisites

- [Go 1.26+](https://go.dev/)
- [Eino](https://github.com/cloudwego/eino) (auto-installed)
- Optional: OpenAI API Key or compatible model

### Build & Run

```bash
# 1. Clone
git clone https://github.com/your-org/sounds-great-ai.git
cd sounds-great-ai

# 2. Install dependencies
go mod download

# 3. Configure
cp .env.example .env
# Edit .env, fill in MODEL_API_KEY etc.

# 4. Run server
go run cmd/server/main.go

# 5. Or run A2A multi-agent test
go run cmd/a2a-test/main.go
```

After server starts:
- `http://localhost:8080/health` — Health check
- `http://localhost:8080/ws` — WebSocket communication
- `http://localhost:8080/api/breeds` — Breed CRUD API

### Create Your Own Dog

```bash
# Create a new dog
curl -X POST http://localhost:8080/api/breeds \
  -H "Content-Type: application/json" \
  -d '{
    "id": "mydog",
    "name": "mydog",
    "display_name": "My Dog",
    "avatar": "mydog.png",
    "personality": "Lively, curious, eager to try everything",
    "system_prompt": "You are my dog, responsible for exploring new things.",
    "model_config": { "provider": "openai", "model": "gpt-4o-mini", "temperature": 0.5 },
    "capabilities": [
      { "name": "command_check", "version": "v1" },
      { "name": "path_validate", "version": "v1" }
    ],
    "workflow": {
      "steps": [
        { "id": "check", "capability_ref": "command_check:v1" },
        { "id": "validate", "capability_ref": "path_validate:v1", "depends": ["check"] }
      ]
    },
    "source": "user",
    "version": "v1"
  }'

# Takes effect immediately — call it
curl -X POST http://localhost:8080/api/breeds/mydog/bark \
  -H "Content-Type: application/json" \
  -d '{ "command": "ls", "path": "/workspace" }'
```

## Roadmap

We build in the open. Here's where we are.

### v0: Live (legacy architecture, still running)

| Feature | Status |
|---------|--------|
| Pack Coordinator (Register / Bark / Validate) | Shipped |
| BreedConfig Config-Driven (JSON hot-reload) | Shipped |
| REST API (CRUD + Source Protection) | Shipped |
| WebSocket → Bark end-to-end pipeline | Shipped |
| Safety Guardrails (CommandCheck / PathValidate) | Shipped |

### v1: Clowder-AI Alignment Restructuring (In Progress)

> Branch: `restructuring/clowder-ai-alignment` | Spec: [design doc](docs/superpowers/specs/2026-08-03-clowder-ai-alignment-restructuring-design.md)

**New platform layer (shipped):**

| Package | Description | Status |
|---------|-------------|--------|
| `internal/adapter/unified/` | Unified AgentExecutor interface + ProcessManager + NDJSON parser | Shipped |
| `internal/adapter/{claude,codex,gemini,opencode}/` | 4 CLI adapters | Shipped |
| `internal/config/` | New breed config (variants[] replaces capabilities[]+workflow[]) | Shipped |
| `internal/skills/` | Skills framework (.md prompt pack loading + injection) | Shipped |
| `internal/router/` | Dynamic routing engine (rule-based + LLM fallback) | Shipped |
| `internal/a2a/` | A2A Hub + context compression (replaces old client/server/orchestrator) | Shipped |
| `internal/sop/` | SOP guardian + cross-model review + max_a2a_depth=3 | Shipped |
| `internal/mcp/` | MCP registry | Shipped |
| `internal/memory/` | Shared memory (evidence/decisions/lessons) | Shipped |
| `internal/platform/` | Platform integration (wires all components) | Shipped |

**Pending cleanup (old code still referenced by server):**

| Old code | Referenced by | Cleanup condition | Status |
|----------|--------------|-------------------|--------|
| `internal/capability/` (20+ Go adapters) | `cmd/server/main.go` | Remove after new platform wired into server | Pending |
| `internal/agent/skill_manager.go` | `cmd/server/main.go` | Remove after replaced by `internal/skills/` | Pending |
| `pkg/pack/workflow.go` (fixed DAG) | server + transport + packapi + capability | Remove after replaced by `internal/router/` | Pending |
| `pkg/pack/capability.go` | server + capability | Remove after replaced by CLI adapters | Pending |

**Already cleaned:**
- `cmd/a2a-test/` — deleted (backed up)
- `internal/a2a/{client,server,orchestrator}/` — deleted (replaced by `internal/a2a/hub.go`, backed up)
- `backup/v0-capability-based/` — deleted (capabilities converted to 8 skill .md files, rest replaced by new platform layer)
- 8 skill .md files created in `packs/default/skills/`

**Verification checklist:**
- [ ] Wire new platform layer into `cmd/server/main.go` (replace `internal/capability` + `pkg/pack` calls)
- [ ] Migrate `internal/transport/` `pkg/pack` references to new router
- [ ] Migrate `internal/packapi/` `pkg/pack` references to new config
- [ ] Convert old `internal/capability/` adapters to skill .md files or MCP tools
- [ ] Replace all `pkg/pack/workflow.go` DAG references with `internal/router/` dynamic routing
- [ ] Full `go build ./...` passes (excluding backup/)
- [ ] Full `go test ./...` passes (excluding backup/)

### v2: Future Plans

| Feature | Target |
|---------|--------|
| File Watcher Hot Reload (fsnotify) | v2 |
| Eino Context Compression (auto-compress on handoff) | v2 |
| MCP Tool Marketplace (dynamic MCP server install) | v2 |
| Prompt version management / A/B test | v2 |
| DB storage replacing JSON files | v2 |
| Contract Test (config compatibility) | v2 |

## Philosophy

### Hard Rails + Dog Pack

Traditional frameworks focus on **control** — what Agents cannot do. Sounds Great AI focuses on **collaboration** — giving dogs a shared task and the autonomy to execute it.

- **Hard Rails** = safety baseline. Non-negotiable. Enforced by code, not prompts.
- **Dog Pack** = above the baseline, dogs self-coordinate, self-check, self-improve.

> Safety cannot depend on prompts. The Rural Dog checking safety is a Pack-layer middleware, not "please check safety" written in the Border Collie's prompt.

### Core Principles

| # | Principle | Meaning |
|---|-----------|---------|
| P1 | Roles are data, capabilities are code | Breed is JSON, Capability is Go, decoupled |
| P2 | Don't modify existing code | Adapters wrap internal/, new capabilities only add |
| P3 | Hot reload first | User creates role → instant effect, no restart |
| P4 | DAG not linear | Workflows support dependencies, parallelism, passing — not simple for loops |
| P5 | Safety enforced by code | Hard Rails at Pack layer, not in prompts |

## Project Structure

```
sounds-great-ai/
├── cmd/
│   ├── server/              # HTTP server entry point
│   └── a2a-test/            # A2A multi-agent test entry point
├── pkg/
│   ├── a2a/                 # A2A protocol types (AgentCard, Task, Message)
│   └── pack/                # Pack/Breed core system
│       ├── breed.go         # BreedConfig data model
│       ├── capability.go    # Capability interface + TaskInput/Output
│       ├── pack.go          # Pack coordinator (Register/Bark/Validate)
│       ├── workflow.go      # DAG workflow executor
│       └── loader.go        # JSON file loader
├── internal/
│   ├── a2a/                 # A2A implementation (server/client/orchestrator)
│   ├── aspect/              # Safety guardrails (command_guard, approval, tracing)
│   ├── capability/          # Capability adapters (command_check, path_validate)
│   ├── component/           # Eino model factory
│   ├── packapi/             # REST API handler
│   ├── transport/           # WebSocket transport layer
│   ├── agent/               # Agent implementation (coder, skill_manager)
│   ├── tool/                # Tools (fs_tools, terminal_tools)
│   └── workspace/           # Workspace management (manager, sandbox, pty)
├── packs/
│   └── default/
│       └── breeds/          # 6 breed JSON configs
└── docs/
    ├── design/              # Design docs (character-setting, story)
    └── superpowers/
        ├── specs/           # Technical spec docs
        └── plans/           # Implementation plan docs
```

## Learn More

- [Pack/Breed System Design](docs/superpowers/specs/2026-07-31-pack-breed-system-design.md) — Complete Config-Driven role system design
- [A2A Multi-Agent Design](docs/superpowers/specs/2026-07-30-a2a-multi-agent-design.md) — A2A protocol implementation spec
- [Character Setting](docs/design/CHARACTER-SETTING.md) — Breed role mapping table
- [Origin Story](docs/design/STORY.md) — The birth story of the Dog Agent Squad

## Contributing

Contributions welcome! Please submit via Fork → branch → PR.

- Follow existing code style and testing conventions
- New Capabilities require both adapter and tests
- New breeds only need a JSON file

## Acknowledgments

This project stands on the shoulders of giants:

- **[Eino](https://github.com/cloudwego/eino)** — CloudWeGo's Go LLM application framework. The orchestration engine, ChatModel interface, and schema types that power every breed's intelligence.
- **[clowder-ai](https://github.com/clowder-ai/clowder)** — The multi-agent cat cafe that inspired the dog pack. Their A2A protocol design, pack system patterns, and open-source philosophy were invaluable references.

## License

[MIT](LICENSE) — Use it, modify it, ship it.

---

<p align="center">
  <em>Build dog packs, not just agents.</em><br>
  <br>
  <strong>When AI Agents Bark Together, It Sounds Great.</strong>
</p>
