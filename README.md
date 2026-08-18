<div align="center">

# Sounds Great AI

**When AI Agents Bark Together, It Sounds Great.**

*Every bark is a precise coordination.*

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Eino](https://img.shields.io/badge/Eino-v0.9+-blueviolet)](https://github.com/cloudwego/eino)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[中文](README.zh-CN.md) | **English**

</div>

---

## Why Sounds Great AI?

You have a codebase. It's large, complex, and accumulating technical debt every day.

You have Claude, GPT, Gemini — powerful models, each with unique strengths. But making them collaborate means **you** become the router: copying context between chat windows, manually tracking who said what, wasting time on middle management.

> *"I don't need a pack of lone wolves. I need a tightly coordinated team."*
> *"Like a dog agent squad — loyal, clearly specialized, one command and everyone moves."*

So under the Go language and Eino orchestration engine, **Sounds Great AI** was born.

This isn't just another Agent invocation framework. It's a **Pack** — a squad of dogs, each with their own role, personality, and capabilities, communicating via A2A protocol, working together in coordinated workflows.

> *When the Agents perfectly complete a collaboration, the terminal lights up with green paw prints:*
> **`Sounds Great!`**

## Design Overview

<div align="center">

High-level architecture and dog-pack collaboration model.

![Sounds Great AI Design](docs/brand/images/sounds-great-ai.png)

</div>

## Screenshots

<div align="center">

**Homepage**

![Homepage](docs/images/homepage.png)

**Settings — Member Management**

![Settings — Members](docs/images/settings-members.png)

</div>

## Features

Each subsystem below has a formal Tech Story (`FT-XXX`) as the single source of truth for its design and implementation.

| Subsystem | Tech Story | Description |
|-----------|-----------|-------------|
| Multi-Agent Orchestration | [FT-ORC-001](docs/designs/FT-ORC-001-multi-agent-orchestration.md) | WebSocket event stream + custody ledger + CLI adapter pool; the front end is initiator and observer |
| CLI Adapter | [FT-CLI-001](docs/designs/FT-CLI-001-cli-adapter.md) | 5 CLI providers driven as one-shot NDJSON subprocesses via a unified `ProcessManager` |
| A2A Communication & Custody | [FT-A2A-001](docs/designs/FT-A2A-001-a2a-communication.md) | In-pack `@mention` collaboration + a controlled external A2A client (no internal server) |
| Settings — Accounts & Keys | [FT-ACC-001](docs/designs/FT-ACC-001-accounts-keys-auth.md) | OAuth / API Key accounts, secret/metadata separation, referential integrity |
| Settings — Member Management | [FT-MEM-001](docs/designs/FT-MEM-001-member-management.md) | Breed CRUD, ordering, default dog, leader, empty-first-run |
| Shared Memory | [FT-SM-001](docs/designs/FT-SM-001-shared-memory.md) | Deterministic supply → human approval → recall injection (zero LLM) |
| Persistent Identity | [FT-PI-001](docs/designs/FT-PI-001-persistent-identity.md) | F231 profiles, F276 people memory, continuity digests |
| Cross-Model Review / QC | [FT-CMR-001](docs/designs/FT-CMR-001-cross-model-review.md) | QC 7-step loop, 3-layer review, Reviewer Delta, pre-merge gate |
| Build & Daemon Toolchain | [FT-DEV-001](docs/designs/FT-DEV-001-makefile-daemon-reclaim.md) | `make dev/prod daemon` lifecycle, self-only process reclaim |

> Phase-level progress (Platform / RAG / A2A / Skills / SOP / Transport / Polish) is tracked in [docs/ROADMAP.md](docs/ROADMAP.md).

## The Pack — Breed Role Mapping

Six dogs, six roles, each with its own specialty:

| Role | Breed | Personality |
|------|-------|-------------|
| **Orchestrator** | Border Collie *(bianmu)* | Extremely intelligent, field-control master, sharp-eyed |
| **Safety Guardrail** | Chinese Rural Dog *(zhonghuatianyuanquan)* | Loyal, reliable, highly alert, knows home terrain |
| **UI / CLI Presentation** | Tibetan Mastiff *(zangao)* | Majestic, imposing, steadfast, gatekeeper |
| **Code Hunter** | Xigou *(xigou)* | Streamlined, lightning-fast, laser-focused |
| **RAG / Retriever** | Golden Retriever *(jinmao)* | Strong retrieval instinct, gentle, dependable |
| **Log & Bug Tracer** | German Shepherd *(demu)* | Alert, black-backed, upright ears, strong execution |

> Users can create their own dogs — just one JSON file, select registered capabilities, define a workflow, and hot-reload takes effect instantly. Member management is documented in [FT-MEM-001](docs/designs/FT-MEM-001-member-management.md).

> **Empty on first run, build your pack on demand**: a fresh install starts with an empty member list (Owner only); the six dogs are an optional *template menu*. Add a dog from **Member Management → Add from template**, bind an account and credentials, and it joins the runtime. Members without credentials show as "Needs config" rather than "Enabled". See `docs/VISION.md` §5.1.

## Architecture

```
┌───────────────────────────────────────────────────┐
│                 packs/default/breeds/               │
│   dog-template.json (只读种子：role_templates +        │
│   breeds；运行时以 dog-catalog.json 为准，用户在设置    │
│   页编辑成员→落 catalog，热加载即时生效)                │
└──────────────────────┬────────────────────────────┘
                       │ LoadFromFile / POST API
                       ▼
┌───────────────────────────────────────────────────┐
│              internal/platform/ (composition root)  │
│   config + router + adapters + skills + mcp + a2a   │
│   + sop + memory + ragstore + threadstore + settings│
└──────────────────────┬────────────────────────────┘
                       │ CLI Adapter Execute()
                       ▼
┌───────────────────────────────────────────────────┐
│            internal/adapter/ (CLI adapters)         │
│   claude/    codex/    gemini/    opencode/    kimi/     │
│   unified/ (ProcessManager + NDJSON parser)         │
└──────────────────────┬────────────────────────────┘
                       │ stdin/stdout pipe
                       ▼
┌───────────────────────────────────────────────────┐
│            External CLI Processes                   │
│   claude CLI  |  codex CLI  |  gemini CLI  |  ...   │
└───────────────────────────────────────────────────┘
```

**Three-Layer Separation Principle:**

| Layer | Responsible For | Not Responsible For |
|-------|----------------|---------------------|
| **Breed JSON (Data)** | Role identity, personality, variant config, model selection | Code logic |
| **Platform (Go + Eino)** | Identity, routing, safety, memory, skills, coordination | LLM reasoning (that's CLI's job) |
| **CLI Adapter** | Spawn CLI, inject prompt, parse stream, manage lifecycle | Role definition, coordination |

> *Roles are data, platform coordinates, CLI reasons.*

## Quick Start

### Prerequisites

- [Go 1.26+](https://go.dev/)
- [Eino](https://github.com/cloudwego/eino) (auto-installed)
- Optional: OpenAI API Key or compatible model

### Build & Run

```bash
# 1. Clone
git clone https://github.com/sounds-great-ai/sounds-great-ai.git
cd sounds-great-ai

# 2. Install dependencies
go mod download
cd web && npm install && cd ..

# 3. Configure (optional — server starts with defaults)
cp .env.example .env
# Edit .env, fill in MODEL_API_KEY etc.

# 4. Run both backend and frontend
make dev
# Backend on :8080, Frontend on :5173

# Or run individually
make backend   # Go server only
make frontend  # Vite dev server only
```

After server starts:
- `http://localhost:8080/health` — Health check
- `http://localhost:8080/ws` — WebSocket communication
- `http://localhost:8080/api/breeds` — Breed CRUD API

### Upgrade

#### Via UI

Click the upgrade button (↑ icon) in the top-right header. Choose whether to pull latest code.

#### Via CLI

```bash
make upgrade
```

This will prompt "是否需要拉取最新的代码？(y/n)", then install dependencies, rebuild frontend and backend.

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
    "default_variant_id": "v1",
    "variants": [
      {
        "id": "v1",
        "client_id": "claude",
        "default_model": "claude-sonnet-4-20250514",
        "system_prompt": "You are my dog, responsible for exploring new things."
      }
    ],
    "source": "user",
    "version": "v1"
  }'

# Takes effect immediately — call it
curl -X POST http://localhost:8080/api/breeds/mydog/bark \
  -H "Content-Type: application/json" \
  -d '{ "command": "ls", "path": "/workspace" }'
```

## Security Audit

After development of a release is complete (all verification checklist items pass), the project undergoes a full security scan before release.

### Tool

[codex-security](https://github.com/openai/codex-security) — OpenAI's CLI and TypeScript SDK for finding, validating, and fixing security vulnerabilities in code.

### Prerequisites

- Verification checklist: all items ✅
- `go build ./...` passes
- `go test ./...` passes
- `npx tsc --noEmit` passes (frontend)
- Node.js 22.13+ installed

### Scan Procedure

```bash
# 1. Install codex-security
npm install @openai/codex-security

# 2. Authenticate
npx @openai/codex-security login

# 3. Basic scan (quick, covers both Go backend and TypeScript frontend)
npx @openai/codex-security scan .

# 4. Deep scan (thorough, multi-agent, for pre-release)
npx @openai/codex-security scan . --mode deep --workers 2 --subagents 0 --stop-after-no-new 3 --max-discovery-runs 10
```

### Scan Scope

| Scope | Path | Language |
|-------|------|----------|
| Backend | `cmd/`, `internal/`, `pkg/` | Go |
| Frontend | `web/src/` | TypeScript/React |
| Config | `packs/`, `.env.example` | JSON / env |

### Fix Workflow

1. **Triage** — classify each finding by severity (critical / high / medium / low)
2. **Fix** — address all critical and high findings before release
3. **Re-scan** — run basic scan to verify fixes
4. **Document** — record accepted risks for medium/low findings

### Pass Criteria

- 0 critical findings
- 0 high findings
- All medium/low findings documented or fixed

---

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
| P4 | CLI adapter, not DAG | Breeds use CLI adapters for execution, not fixed workflow DAGs |
| P5 | Safety enforced by code | Hard Rails at Pack layer, not in prompts |

## Project Structure

```
sounds-great-ai/
├── cmd/
│   └── server/              # HTTP server entry point
├── pkg/
│   ├── a2a/                 # A2A protocol types
│   └── pack/                # Pack/Breed core system (breed.go schema + loader.go)
├── internal/
│   ├── adapter/             # CLI adapters (claude/codex/gemini/opencode/kimi)
│   ├── a2a/                 # A2A Hub + context compression
│   ├── aspect/              # Safety guardrails (command_guard, approval, tracing)
│   ├── capability/          # 6 pure-logic capabilities
│   ├── component/           # Eino model factory
│   ├── config/              # Event bus (config/settings change events)
│   ├── mcp/                 # MCP registry
│   ├── memory/              # Shared memory (evidence/decisions/lessons)
│   ├── packapi/             # REST API handler
│   ├── platform/            # Platform composition root
│   ├── ragstore/            # RAG store (Memory/SQLite/Eino backends)
│   ├── domains/             # 六边形域（routing/threads/custody/sop 等）
│   ├── skills/              # Skills framework (.md loading + injection)
│   ├── sop/                 # SOP guardian
│   ├── settings/            # 设置存储（文件落盘 + 热加载）
│   ├── threadstore/         # Thread store
│   ├── transport/           # WebSocket + HTTP transport layer
│   ├── agent/               # Agent implementation (coder, skill_manager)
│   ├── tool/                # Tools (fs_tools, terminal_tools)
│   └── workspace/           # Workspace management
├── packs/
│   └── default/
│       ├── breeds/          # 狗狗品种配置（dog-template.json 只读种子，运行时以 dog-catalog.json 为准）
│       └── skills/          # SKILL.md prompt packs
├── web/                     # Frontend (React + Vite)
└── docs/
    ├── designs/             # Tech Stories (FT-XXX) — subsystem single source of truth
    ├── architecture/        # Architecture docs
    ├── governance/decisions/           # ADR records
    ├── brand/              # Design docs
    └── plans/               # Implementation plans
```

## Learn More

- [Tech Stories (FT-XXX)](docs/designs/) — Subsystem design truth (orchestration, CLI adapter, A2A, settings, memory, identity, QC, toolchain)
- [Architecture Lineage](docs/architecture/architecture-lineage.md) — Full architecture topic index
- [Memory Philosophy](docs/architecture/memory-philosophy.md) — 7 axioms, 21 laws, judgment criteria
- [Character Setting](docs/brand/CHARACTER-SETTING.md) — Breed role mapping table
- [Origin Story](docs/brand/STORY.md) — The birth story of the Dog Agent Squad
- [Roadmap](docs/ROADMAP.md) — Phase-level progress

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
