# API Reference

All endpoints are served by the Go backend on port 8080 (default).

## Authentication

All API endpoints require a valid session token. The token is obtained via the WebSocket handshake or HTTP header:

```
Authorization: Bearer <token>
```

Missing or invalid token returns `401 Unauthorized`.

## Error Codes

| HTTP Status | Meaning | Example |
|-------------|---------|---------|
| 400 | Bad Request — invalid JSON / missing required field | `{"error": "missing field: title"}` |
| 401 | Unauthorized — missing or invalid token | `{"error": "unauthorized"}` |
| 404 | Not Found — resource does not exist | `{"error": "breed not found: foo"}` |
| 409 | Conflict — duplicate resource | `{"error": "breed already exists: bianmu"}` |
| 500 | Internal Server Error | `{"error": "database error"}` |

Error response format:
```json
{
  "error": "string",
  "detail": "optional string"
}
```

## Pack API

### GET /api/breeds
List all breed configurations.
**Response:** `200 OK`
```json
[
  {
    "id": "bianmu",
    "name": "bianmu",
    "display_name": "Border Collie",
    "cli_adapter": "claude",
    "tendency": ["任务分解", "路由决策", "结果合成"],
    "restrictions": ["直接写业务代码"]
  }
]
```

### POST /api/breeds
Create a new breed.
**Request:**
```json
{
  "id": "newbreed",
  "name": "newbreed",
  "display_name": "New Breed",
  "cli_adapter": "claude",
  "tendency": ["职责"],
  "restrictions": ["禁区"]
}
```
**Response:** `201 Created` — breed object | `409 Conflict` — id already exists

### GET /api/breeds/{id}
Get a specific breed by ID.
**Response:** `200 OK` — breed object | `404 Not Found`

### PUT /api/breeds/{id}
Update a breed configuration.
**Request:** breed object (partial update supported)
**Response:** `200 OK` — updated breed | `404 Not Found`

### DELETE /api/breeds/{id}
Delete a breed.
**Response:** `204 No Content` | `404 Not Found`

## Thread API

### GET /api/threads
List all conversation threads.
**Query Params:** `?limit=20&offset=0`
**Response:** `200 OK`
```json
[
  {
    "id": "thread_abc123",
    "title": "Hook integration discussion",
    "created_at": "2026-08-01T10:00:00Z",
    "updated_at": "2026-08-09T03:13:00Z",
    "message_count": 42
  }
]
```

### POST /api/threads
Create a new thread.
**Request:** `{ "title": "Discussion title" }`
**Response:** `201 Created` — thread object

### GET /api/threads/{id}
Get a specific thread with messages.
**Query Params:** `?limit=100&before=<messageId>`
**Response:** `200 OK` — thread object with messages array | `404 Not Found`

### DELETE /api/threads/{id}
Delete a thread.
**Response:** `204 No Content` | `404 Not Found`

## Settings API

### GET /api/settings
Get current application settings.
**Response:** `200 OK`
```json
{
  "default_breed": "bianmu",
  "max_parallel": 3,
  "a2a_max_depth": 3,
  "rag_enabled": true,
  "telemetry_enabled": true
}
```

### PUT /api/settings
Update application settings.
**Request:** partial settings object
**Response:** `200 OK` — updated settings

## Skills API

### GET /api/skills
List all available skills.
**Response:** `200 OK`
```json
[
  {
    "name": "quality-gate",
    "description": "开发完成后的自检门禁",
    "triggers": ["完成", "交付", "quality gate", "自检"]
  }
]
```

## MCP API

### GET /api/mcp/servers
List registered MCP servers.
**Response:** `200 OK`
```json
[
  {
    "name": "ragstore",
    "command": "ragstore-mcp",
    "enabled": true
  }
]
```

## Upgrade API

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
Execute upgrade.
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

**Connection:** `ws://localhost:8080/ws`

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
