# Setup Guide

**English** | [中文](SETUP.zh-CN.md)

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| **Go** | >= 1.26 | [go.dev](https://go.dev/dl/) |
| **Node.js** | >= 20.0.0 | [nodejs.org](https://nodejs.org/) (for web frontend) |
| **Redis** | >= 7.0 | `brew install redis` (macOS) — *可选，仅当设置 `SG_REDIS_URL`（或 `REDIS_URL`）环境变量时启用，默认关闭* |
| **Git** | any recent | Comes with most systems |

## Quick Start

```bash
# 1. Clone
git clone https://github.com/sounds-great-ai/sounds-great-ai.git
cd sounds-great-ai

# 2. Install CLI tools (see docs/reference/cli.md for details)
npm install -g @anthropic-ai/claude-code@latest
npm install -g @google/gemini-cli@latest

# 3. Build
go build ./...

# 4. Run
go run cmd/server/main.go

# 5. Open frontend (optional)
cd web && npm install && npm run dev
```

## CLI Tool Configuration

See `docs/reference/cli.md` for installing and configuring Claude Code, Gemini CLI, Codex, and other CLI agents.

## Project Structure

| Directory | Purpose |
|-----------|---------|
| `cmd/` | Entry points |
| `internal/` | Core packages (config, memory, ragstore, adapter, etc.) |
| `pkg/` | Public packages |
| `packs/` | Breed configs, skills, hooks |
| `docs/` | Documentation |
| `web/` | Frontend (React + Vite) |
| `scripts/` | Build/utility scripts |

## Troubleshooting

- **Port already in use**: 用 `PORT` 环境变量改端口，例如 `PORT=9090 go run cmd/server/main.go`（默认 8080）
- **CLI not found**: Ensure CLI tools are installed globally (see `docs/reference/cli.md`)
- **Breed config not loading**: Check `packs/default/breeds/dog-template.json` syntax
