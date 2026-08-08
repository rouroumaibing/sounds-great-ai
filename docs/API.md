# API Reference

All endpoints are served by the Go backend on port 8080 (default).

## Pack API

### GET /api/breeds
List all breed configurations.
**Response:** `200 OK` — `[{ "id": "...", "name": "...", "display_name": "...", ... }]`

### POST /api/breeds
Create a new breed.
**Request:** `{ "id": "...", "name": "...", ... }`
**Response:** `201 Created` — breed object

### GET /api/breeds/{id}
Get a specific breed by ID.
**Response:** `200 OK` — breed object | `404 Not Found`

### PUT /api/breeds/{id}
Update a breed configuration.
**Request:** breed object
**Response:** `200 OK` — updated breed | `404 Not Found`

### DELETE /api/breeds/{id}
Delete a breed.
**Response:** `204 No Content` | `404 Not Found`

## Thread API

### GET /api/threads
List all conversation threads.
**Response:** `200 OK` — `[{ "id": "...", "title": "...", ... }]`

### POST /api/threads
Create a new thread.
**Request:** `{ "title": "..." }`
**Response:** `201 Created` — thread object

### GET /api/threads/{id}
Get a specific thread with messages.
**Response:** `200 OK` — thread object | `404 Not Found`

### DELETE /api/threads/{id}
Delete a thread.
**Response:** `204 No Content` | `404 Not Found`

## Settings API

### GET /api/settings
Get current application settings.
**Response:** `200 OK` — settings object

### PUT /api/settings
Update application settings.
**Request:** settings object
**Response:** `200 OK` — updated settings

## Skills API

### GET /api/skills
List all available skills.
**Response:** `200 OK` — `[{ "name": "...", "description": "...", "triggers": [...] }]`

## MCP API

### GET /api/mcp/servers
List registered MCP servers.
**Response:** `200 OK` — `[{ "name": "...", "command": "...", "enabled": true }]`

## Upgrade API

### GET /api/upgrade/info
Detect installation mode and current version.
**Response:** `200 OK` — `{ "mode": "source" | "release", "version": "v0.1.0", "repo": "..." }`

### POST /api/upgrade
Execute upgrade.
**Request:** `{ "pull": true }` — whether to git pull first (source mode only)
**Response:** `200 OK` — `{ "success": true, "message": "...", "logs": [...] }`

## WebSocket

### WS /ws
Real-time communication for chat execution, streaming output, and agent coordination.
**Events:** `execute`, `execute_parallel`, `stream`, `error`, `complete`
