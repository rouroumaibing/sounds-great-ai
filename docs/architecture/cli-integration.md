---
feature_ids: [F118, F212]
topics: [architecture, cli, integration]
doc_kind: note
created: 2026-02-26
---

# CLI 集成架构：claude / codex / gemini / opencode / kimi / a2a

> Sounds Great AI 如何通过一层 **CLI adapter** 对接六个外部 AI CLI 工具
> 最后更新：2026-08-21

## 概述

Sounds Great AI 通过 `internal/adapter/` 下的 **client adapter** 对接六个外部 AI CLI，由 `internal/platform/platform.go:199-206` 的 `adapters` map 装配：

| 犬（dog_id） | client_id | 角色 |
|---|---|---|
| 边牧 bianmu | `claude` | orchestrator / architect |
| 金毛 jinmao | `gemini` | knowledge-retriever |
| 灵缇 xigou | `codex` | code-hunter / reviewer |
| 德牧 demu | `opencode` | tracer / diagnostician |
| 藏獒 zangao | `claude` | presenter / formatter |
| 中华田园犬 zhonghuatianyuanquan | `codex` | safety-guard |
| 远程协作者（A2A 客户端） | `a2a` | 外部已部署 agent |

犬↔client 映射的真相源是 `packs/default/breeds/dog-template.json`（运行时以 `.sounds-great-ai/dog-catalog.json` 为准）。

各 adapter 共用一套 **unified 框架**（`internal/adapter/unified/`）拉起外部 CLI 子进程，spawn contract 返回 `*SpawnHandle`，并按四级 carrier 链降级。

## 架构概览

```
 CLI adapter（internal/adapter/{claude,codex,gemini,opencode,kimi,a2a}/adapter.go）
        │  buildArgs() 构造 CLI 参数（双返回值：([]string, 临时 MCP 配置路径)）
        ▼
 unified.ProcessManager.Spawn(ctx, cmd, args, stdin)  →  *SpawnHandle
        │  (Stdout / PID / ExitInfo / StderrString / OnExit)
        ▼
 carrier 链（per-provider，四级降级）：
   bg_daemon → interactive_pty → print_sdk（默认安全层）→ api_key
        │
        ▼
 输出按 adapter 解析（NDJSON / plain text）
```

**核心设计决策：CLI 子进程而非 SDK**

1. **订阅复用**：用户已有 Claude / ChatGPT / Gemini 订阅，不想再付 API 费用
2. **功能完整**：CLI 已实现 MCP、工具调用、文件操作等复杂能力
3. **隔离安全**：子进程天然隔离，崩溃不影响主进程
4. **更新解耦**：CLI 更新不需要重新部署后端

## spawn contract（`internal/adapter/unified/`）

- **`ProcessManager.Spawn(ctx, cmd, args, stdin) (*SpawnHandle, error)`**（`process.go:238`）——统一进程拉起入口。
- **`SpawnHandle`**（`process.go:39`）：`Stdout io.Reader` / `PID` / `ExitInfo()` / `StderrString()` / `OnExit()`。
- **`buildArgs` 双返回值** `([]string, string)`（第二返回值为临时 MCP 配置路径，`OnExit` 时清理）：claude `adapter.go:90`、codex `adapter.go:82`、opencode `adapter.go:81`；gemini 参数在 `Execute` 内联构造（`adapter.go:51`）；kimi 仅返回 `[]string`（`adapter.go:84`）。

### carrier 链（`internal/adapter/unified/carrier.go`）

四级降级顺序 `bg_daemon → interactive_pty → print_sdk → api_key`（`carrier.go:30-35`）：

| tier | 说明 |
|------|------|
| `bg_daemon` | 常驻 warm pool（R2），无冷启动但带 lease/zombie/MCP 重建成本 |
| `interactive_pty` | 伪终端交互（R3），保留给需要真实 TTY 的 CLI |
| `print_sdk` | 一次性 CLI 子进程走 stdout 管道，**当前 SG 行为 + 默认安全层**（`carrier.go:22-24`） |
| `api_key` | 直连 API key，最后兜底 |

- 默认构建下，warm pool 退化为 one-shot `print_sdk`；`WireWarmPools` 自 2026-08-17 **默认开启**（`platform.go:212`）。
- per-provider：claude/codex/gemini 以 `bg_daemon` 打头、回退 `print_sdk`；opencode/kimi 保持 one-shot `print_sdk`（`platform.go:208-210`）。
- 默认载波链装配见 `platform.go:208-218`；`TransportPrintSDK` 经 `ProcessTransport` 把 `ProcessManager` 适配进 print_sdk tier（`carrier.go:249-264`）。

## 各 CLI adapter 简述

- **claude / codex / gemini / opencode / kimi**：各 `internal/adapter/<client>/adapter.go` 实现 `unified.AgentExecutor`，负责 `buildArgs` 拼装 + 输出解析 + session resume（`--resume <id>`）。
- **a2a**（`internal/adapter/a2a/adapter.go`）：与 CLI 并列的 sibling carrier，向**外部**已部署 agent 发 A2A Protocol `tasks/send` JSON-RPC over HTTPS（`adapter.go:110-172`）；`Capabilities()` 显式 `SupportsMCP/Tools/FileOps=false`（`adapter.go:72-80`），不暴露入站 server、不经平台推理（`adapter.go:1-16` 注释）。

## A2A（仅客户端）

- 协议类型与常量在 `pkg/a2a/`（`MethodTasksSend="tasks/send"` 等，`methods.go:3-7`）。
- `internal/a2a/hub.go` 是**进程内**协作 hub（`A2AHub`/`Thread`/`Handoff`），用于犬间 handoff 线程记账，**不是网络 server**。
- 红线：平台只作 A2A **客户端**，不新建 A2A server（见 `docs/features/FT-A2A-001-a2a-communication.md:27`）。

## 共享能力注入（MCP）

CLI agent 通过内置 MCP server 访问共享能力（非 @mention 内嵌 tool）：

- **RAG**：`cmd/server/main.go:76-82` `pl.MCPStore.SeedKnowledge(...)`。
- **记忆 / roster / breeds**：`cmd/server/main.go:89-99` `SeedPlatform(...)` 暴露 `collab/memory/people/roster/breeds`。
- CLI 侧以临时 `--mcp-config` 文件注入（如 `claude/adapter.go:102-107` 写 ephemeral 配置并在 `OnExit` 删除）。

## 配置管理

- 运行时狗狗配置落盘 `.sounds-great-ai/`（`accounts.json` / `credentials.json`(0600) / `dog-catalog.json` 等），由 `internal/settings/` 的 `FileSettingsStore` / `FileCredentialStore` 原子写管理。
- `ValidCLIClientIDs` 白名单 = `{claude, codex, gemini, opencode, kimi}`（`internal/settings/validation.go:11-17`）；`a2a` 是单独的 client_id，由 a2a adapter 处理。

## 测试策略与相关文件

- Go gate：`go build ./...` · `go vet ./...` · `go test ./...`；前端 `cd web && npx tsc --noEmit && npm run build`。
- 相关文件：`internal/adapter/{claude,codex,gemini,opencode,kimi,a2a}/adapter.go`、`internal/adapter/unified/{process.go,carrier.go,fallback.go}`、`internal/platform/platform.go`、`pkg/a2a/`、`internal/a2a/hub.go`、`packs/default/breeds/dog-template.json`。
