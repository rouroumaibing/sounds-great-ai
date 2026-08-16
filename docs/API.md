# API Reference

All endpoints are served by the Go backend on port `8080` by default (override with the `PORT` environment variable). The SPA is served from `/` when `web/dist` exists.

> Source of truth: endpoints are mounted in `cmd/server/routes.go` and implemented under `internal/transport/*` and `internal/packapi/*`. If this document and the code disagree, the code wins — please open an issue or PR.

## Table of Contents

- [Authentication](#authentication)
- [Error Codes](#error-codes)
- [Pack API (breeds)](#pack-api-breeds)
- [Config API](#config-api)
- [Settings API](#settings-api)
- [Thread API](#thread-api)
- [Memory API](#memory-api)
- [People Memory API](#people-memory-api)
- [Profiles API](#profiles-api)
- [Continuity API](#continuity-api)
- [Rules & Prompt-Injection API](#rules--prompt-injection-api)
- [Plugins / Marketplace / Panels API](#plugins--marketplace--panels-api)
- [Notifications API](#notifications-api)
- [Files API](#files-api)
- [RAG API](#rag-api)
- [Repo Trajectory API](#repo-trajectory-api)
- [Custody API](#custody-api)
- [Ops API](#ops-api)
- [Evals API](#evals-api)
- [Skills API](#skills-api)
- [MCP API](#mcp-api)
- [Upgrade API](#upgrade-api)
- [WebSocket](#websocket)

## Authentication

Most API endpoints require a valid session token. The token is passed either via the WebSocket handshake or the HTTP header:

```
Authorization: Bearer <token>
```

Missing or invalid token returns `401 Unauthorized`.

**Public (unauthenticated) endpoints:** `GET /health`, `GET /ready`, `GET /ws` (WebSocket), and `GET /api/rag/backend` (degraded mode). Everything else is wrapped by `transport.NewAuthMiddleware`.

## Error Codes

| HTTP Status | Meaning | Example |
|-------------|---------|---------|
| 400 | Bad Request — invalid JSON / validation failure / missing required field | `{"error": "invalid client_id \"foo\"; allowed: claude, codex, gemini, opencode, kimi"}` |
| 401 | Unauthorized — missing or invalid token | `{"error": "unauthorized"}` |
| 403 | Forbidden — operation rejected by policy (e.g. `pack.Register`/`Unregister` refusal such as a protected system breed) | `{"error": "cannot unregister system breed: bianmu"}` |
| 404 | Not Found — resource does not exist | `{"error": "breed not found: foo"}` |
| 409 | Conflict — resource conflict (e.g. deleting an account bound to members without `force`) | `{"error": "account bound to members", "bound_member_ids": ["m1"]}` |
| 500 | Internal Server Error | `{"error": "database error"}` |

Error response body is a JSON object:

```json
{
  "error": "string",
  "detail": "optional string"
}
```

Some handlers attach structured fields (e.g. `bound_member_ids`) alongside `error`.

## Pack API (breeds)

Handlers: `internal/packapi/handler.go`. Base path: `/api/breeds`.

A breed is described by `pkg/pack/breed.go`'s `BreedConfig`. The canonical identity is now **variant-based**: each breed has one or more `variants`, and each variant names a `client_id` from the CLI whitelist (`claude`, `codex`, `gemini`, `opencode`, `kimi`). The legacy top-level `cli_adapter` / `tendency` / `restrictions` fields from older docs no longer exist.

### GET /api/breeds
List all breed configurations (system + user + plugin sources merged).
**Response:** `200 OK`
```json
[
  {
    "id": "bianmu",
    "name": "bianmu",
    "display_name": "Border Collie",
    "nickname": "边牧",
    "avatar": "🐕",
    "color": { "primary": "#3b82f6", "secondary": "#1e40af" },
    "personality": "冷静、精确、爱拆解任务",
    "role_description": "任务分解、路由决策、结果合成",
    "team_strengths": "跨犬协作与编排",
    "mention_patterns": ["@bianmu", "@边牧"],
    "roles": ["coordinator"],
    "caution": "不倾向直接写业务代码",
    "default_variant_id": "claude",
    "variants": [
      {
        "id": "claude",
        "variant_label": "Claude",
        "client_id": "claude",
        "default_model": "claude-opus-4",
        "mcp_support": true,
        "cli": { "command": "claude", "output_format": "stream-json", "default_args": ["--print"] },
        "strengths": ["reasoning"],
        "team_strengths": "编排",
        "caution": "避免直接写业务代码",
        "context_budget": { "max_prompt_tokens": 200000, "max_context_tokens": 200000, "max_messages": 200 },
        "voice_config": { "voice": "zh-CN-XiaoxiaoNeural" },
        "account_ref": "acc_claude",
        "provider": "anthropic",
        "strategy": "default",
        "auto_compact_token_limit": 100000
      }
    ],
    "review_policy": { "prefer_lead": false },
    "features": { "sessionChain": false, "missionHub": { "selfClaimScope": "breed" } },
    "restrictions": ["直接写业务代码"],
    "relationship_key": "pack",
    "dog_id": "bianmu",
    "source": "system",
    "enabled": true
  }
]
```

> The exact field set is defined in `BreedConfig` / `Variant` (`pkg/pack/breed.go`). Unknown fields are ignored on decode. Nested objects shown above are representative, not exhaustive.

### POST /api/breeds
Create or replace a breed. Validates mention-pattern uniqueness, `client_id` whitelist, and `account_ref` existence.
**Request:** a `BreedConfig` object (omit `source` to default to `user`).
**Response:** `200 OK` — the created/updated config | `400 Bad Request` — validation error | `403 Forbidden` — rejected by `pack.Register` (e.g. alias conflict)

> Note: creation returns **200**, not 201. Registering over an existing `id` updates it in place.

### PATCH /api/breeds/{id}
Partial update of a breed.
**Response:** `200 OK` — updated config | `404 Not Found` | `403 Forbidden`

### DELETE /api/breeds/{id}
Unregister and remove a breed (rolls back if removal fails).
**Response:** `200 OK` | `403 Forbidden` — protected/system breed cannot be removed

### GET /api/breeds/templates
Return the full role-template list used by the "create from template" UI (`RoleTemplate[]`).
**Response:** `200 OK`

### GET /api/breeds/{id}/status
Runtime status for a breed (adapter health, last run, etc.).
**Response:** `200 OK` | `404 Not Found`

### POST /api/breeds/{id}/bark
Trigger a "bark" (a test/diagnostic invocation) for the given breed.
**Response:** `200 OK` | `404 Not Found`

> There is **no** `GET /api/breeds/{id}` single-resource endpoint — to fetch one breed, list all via `GET /api/breeds` and filter by `id`.

## Config API

Handlers: `internal/transport/config_handler.go`. Base path: `/api/config`. These mutate runtime/system config that is persisted to disk (hot-reloaded).

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/config/default-breed` | Current default breed (`{ "breed_id": "...", "is_override": bool }`) |
| PUT | `/api/config/default-breed` | Set default breed |
| GET | `/api/config/breed-order` | Ordered breed IDs |
| PUT | `/api/config/breed-order` | Reorder breeds |
| GET | `/api/config/repo` | Configured code-repo URL (project archive source) |
| PUT | `/api/config/repo` | Set code-repo URL |
| GET | `/api/config/env-summary` | Snapshot of relevant env vars (read-only) |
| PATCH | `/api/config/env` | Update persisted env overrides |
| GET | `/api/config/leader` | Current leader (owner) config |
| PATCH | `/api/config/leader` | Update leader config (persisted) |

## Settings API

Handlers: `internal/transport/settings_handler.go`. Base path: `/api/settings`.

> **Important:** there is **no** root `GET /api/settings` or `PUT /api/settings` endpoint. Settings are grouped into sub-resources below and persisted automatically (no separate "save" call). Changes are hot-reloaded by the file-backed store.

### Accounts
| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/settings/accounts` | List accounts (metadata only; secrets are never returned) |
| POST | `/api/settings/accounts` | Create an account |
| PATCH | `/api/settings/accounts/{id}` | Update an account |
| DELETE | `/api/settings/accounts/{id}` | Delete an account. Returns `409 Conflict` with `bound_member_ids` if any member references it, unless forced |

### Roster (members)
| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/settings/roster` | List roster entries (members) |
| GET | `/api/settings/roster/{id}` | Get one member |
| PATCH | `/api/settings/roster/{id}` | Update a member (e.g. `account_ref`, display fields) |

### Review Policy
| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/settings/review-policy` | Get the pack-wide review policy |
| PUT | `/api/settings/review-policy` | Set the review policy |

### System Config
| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/settings/config` | List system config key/value pairs |
| PATCH | `/api/settings/config` | Update system config key/value pairs |

Credentials (API keys/secrets) live in a separate credential store mounted alongside this handler; they are **not** exposed via these GET endpoints.

## Thread API

Handlers: `internal/transport/thread_handler.go`. Base paths: `/api/threads`, `/api/sessions`.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/threads` | List threads (`?limit=&offset=`) |
| POST | `/api/threads` | Create a thread (`{ "title": "..." }`) → `200 OK` |
| GET | `/api/threads/{id}` | Get a thread with its messages (`?limit=&before=`) |
| PATCH | `/api/threads/{id}` | Update thread metadata |
| DELETE | `/api/threads/{id}` | Delete a thread |
| GET | `/api/threads/{id}/messages` | List messages of a thread |
| POST | `/api/threads/{id}/events` | Append a thread event |
| GET | `/api/threads/{id}/sessions` | List CLI sessions for a thread |
| POST | `/api/sessions/{id}/unseal` | Unseal a session (finalize its transcript) |

## Memory API

Handlers: `internal/transport/memory_handler.go`. Base path: `/api/memory`.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/memory/evidence` | List evidence records |
| POST | `/api/memory/evidence` | Add an evidence record |

Evidence records power the shared memory system (decisions, lessons, proofs). See `docs/architecture/memory-system-overview.md`.

## People Memory API

Handlers: `internal/transport/people_memory_handler.go`. Base path: `/api/people-memory`. Persistent-identity store for people/operators the pack interacts with. Representative endpoints:

- `GET /api/people-memory` — list people
- `GET /api/people-memory/operators` — list operators
- `GET /api/people-memory/candidates` — proposed-person candidates (Approval Hub)
- `GET /api/people-memory/deferred` — deferred (parked) memory receipts
- `GET /api/people-memory/person/{personID}` — get a person
- `GET /api/people-memory/person/{personID}/card` — recall capsule card
- `POST /api/people-memory/propose` — propose a new person/claim
- `POST /api/people-memory/candidates/{candidateID}/approve` (and `/reject`, `/reject-drafts`, `/not-now`, `/withdraw`, `/forget`, `/undo`) — Approval Hub actions
- `POST /api/people-memory/person/{personID}/claims/{claimID}/correct` (and `/retire`) — claim lifecycle
- `POST /api/people-memory/deferred/{receiptID}/claim` (and `/withdraw`, `/forget`) — deferred receipt lifecycle
- `POST /api/people-memory/person/{personID}/forget` — forget a person (redaction)

> Exact request/response shapes live in `people_memory_handler.go`. This surface is part of the Persistent Identity feature (`docs/DESIGN-STORYS/SG-PI-001-persistent-identity.md`).

## Profiles API

Handlers: `internal/transport/profiles_handler.go`. Base path: `/api/profiles`. Key/value profile store with propose/approve workflow and autonomous distillation.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/profiles` | List profile keys |
| GET | `/api/profiles/{key}` | Get a profile |
| PUT | `/api/profiles/{key}` | Upsert a profile |
| DELETE | `/api/profiles/{key}` | Delete a profile |
| POST | `/api/profiles/{key}/propose` | Propose a change (needs approval) |
| GET | `/api/profiles/{key}/proposal` | Get pending proposal |
| POST | `/api/profiles/{key}/proposal/approve` (and `/reject`) | Resolve proposal |
| POST | `/api/profiles/{key}/distill` (and `/distill/agent`) | Trigger a distill |

## Continuity API

Handlers: `internal/transport/profiles_handler.go` (`ContinuityHandler`). Base path: `/api/continuity`. Rotation-aware continuity inspection.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/continuity` | List continuity state across breeds |
| GET | `/api/continuity/{breedID}` | Get continuity state for a breed |

## Rules & Prompt-Injection API

Handlers: `internal/transport/rules_handler.go`. Base paths: `/api/rules`, `/api/prompt-injection`.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/rules` | Get the active ruleset (compiled from `AGENTS.md` + hooks) |
| GET | `/api/prompt-injection/manifest` | Hook manifest (which hooks fire, with resolved variables) |
| GET | `/api/prompt-injection/preview` | Compile a preview of the injected system prompt |

## Plugins / Marketplace / Panels API

Handlers: `internal/transport/panels_handler.go`. Panels are read/served configuration blocks for the UI.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/config/concierge` | Concierge panel config |
| GET | `/api/config/voice` | Voice panel config |
| GET | `/api/config/connectors` | Connectors panel config |
| GET | `/api/plugins` | List installed plugins |
| GET | `/api/plugins/{id}` | Get one plugin |
| GET | `/api/marketplace` | List marketplace offerings |

## Notifications API

Handlers: `internal/transport/notifications_handler.go`. Base path: `/api/notifications`.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/notifications` | List notifications |
| GET / PATCH / DELETE | `/api/notifications/{id}` | Get / update / dismiss a notification |

## Files API

Handlers: `internal/transport/files_handler.go`. Base path: `/api/files`.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/files/tree` | Workspace file tree (scoped to the workspace dir) |

## RAG API

Handlers: `internal/transport/rag_handler.go`. Base path: `/api/rag`. Requires an initialized embedder; in degraded mode only `/api/rag/backend` is served (returns `active: "none"`).

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/rag/backend` | Active RAG backend + status |
| GET | `/api/rag/backend/switch` | Switch backend |
| POST | `/api/rag/sync` | Trigger a sync/index |
| GET | `/api/rag/sync/progress` | Sync progress |

## Repo Trajectory API

Handlers: `internal/transport/repo_trajectory_handler.go`. Base path: `/api/repo`. Powers the project archive source (code-repo git-ref timeline).

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/repo/trajectory` | Get the repo trajectory timeline |
| POST | `/api/repo/test` | Test the configured repo connection |

## Custody API

Custody (ball/hold) endpoints coordinate cross-thread duty and `hold_ball` wakeups. Mounted in `cmd/server/routes.go`.

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/custody/holds/{threadID}/webhook?token=XXX` | External webhook to release a parked hold and resume the holder (`CustodyWakeHandler`) |
| GET | `/api/custody/threads/{threadID}/trail` | Project the custody ledger for a thread into a briefing (`CustodyTrailHandler`) |
| GET | `/api/custody/briefing` | Cross-thread duty briefing (operations view) (`CustodyDutyBriefingHandler`) |

These require the platform (and its ball ledger / hold scheduler) to be initialized; otherwise they return `503`.

## Ops API

Operations/observability endpoints.

| Method | Path | Purpose | Auth |
|--------|------|---------|------|
| GET | `/api/ops/health` | Process health, uptime, goroutines, memory, otel status | required |
| GET | `/api/ops/logs` | Recent in-memory log buffer (last 100) | required |
| GET | `/api/ops/metrics` | Runtime metrics | — |
| GET | `/api/ops/metrics/history` | Metrics history | — |
| GET | `/api/ops/traces` | Traces | — |
| GET | `/api/ops/git` | Git status (branch, ahead/behind, dirty) | required |
| GET | `/api/diagnostics/pool` | CLI process pool + rate-monitor diagnostics | — |

## Evals API

Handlers: `internal/transport/eval_handler.go` (mounted only when an eval handler is configured). Base path: `/api/evals`.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/evals` | List evaluations |
| POST | `/api/evals/run` | Run an evaluation |
| GET | `/api/evals/results` | List evaluation results |
| GET | `/api/evals/results/{id}` | Get one result |

## Skills API

Handler: `SkillsHandler` in `cmd/server/routes.go`. Base path: `/api/skills`.

### GET /api/skills
List available skills from `packs/default/skills`.
**Response:** `200 OK`
```json
[
  {
    "name": "quality-gate.md",
    "source": "packs/default/skills"
  }
]
```

> Unlike older docs, the response items carry `name` (including the `.md` extension) and `source` — there is no `description` or `triggers` field.

## MCP API

Handler: `MCPServersHandler` in `cmd/server/routes.go`. Base path: `/api/mcp/servers`.

### GET /api/mcp/servers
List registered MCP servers.
**Response:** `200 OK`
```json
[
  {
    "name": "ragstore",
    "tools": [],
    "enabled": true
  }
]
```

> Unlike older docs, the response items carry `name`, `tools` (currently always an empty array), and `enabled` — there is no `command` field.

## Upgrade API

Handlers: `UpgradeInfoHandler` / `UpgradeHandler` in `cmd/server/routes.go`.

### GET /api/upgrade/info
Detect installation mode and current version.
**Response:** `200 OK`
```json
{
  "mode": "source",
  "version": "v0.1.0",
  "repo": "sounds-great-ai"
}
```

### POST /api/upgrade
Execute an upgrade.
**Request:** `{ "pull": true }`
**Response:** `200 OK`
```json
{
  "success": true,
  "message": "Upgraded to v0.2.0",
  "logs": ["pulling...", "building...", "done"]
}
```

## WebSocket

### WS /ws
Real-time communication for chat execution, streaming output, and agent coordination.

**Connection:** `ws://localhost:8080/ws` (public — no Bearer token; auth happens via handshake)

**Event: execute** — Start agent execution
```json
{
  "type": "execute",
  "thread_id": "thread_abc123",
  "breed_id": "bianmu",
  "message": "用户输入文本"
}
```

**Event: execute_parallel** — Start parallel execution across multiple breeds
```json
{
  "type": "execute_parallel",
  "thread_id": "thread_abc123",
  "breed_ids": ["bianmu", "xigou"],
  "message": "用户输入文本"
}
```

**Event: stream** — Streaming output from agent
```json
{
  "type": "stream",
  "thread_id": "thread_abc123",
  "breed_id": "bianmu",
  "chunk": "输出文本片段",
  "sequence": 1
}
```

**Event: error** — Execution error
```json
{
  "type": "error",
  "thread_id": "thread_abc123",
  "breed_id": "bianmu",
  "error": "error message",
  "code": "INTERNAL_ERROR"
}
```

**Event: complete** — Execution completed
```json
{
  "type": "complete",
  "thread_id": "thread_abc123",
  "breed_id": "bianmu",
  "message_id": "msg_xyz789",
  "duration_ms": 5234
}
```
