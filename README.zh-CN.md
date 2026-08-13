<div align="center">

# Sounds Great AI

**When AI Agents Bark Together, It Sounds Great.**

*每一声吠叫，都是一次精准的协同。*

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Eino](https://img.shields.io/badge/Eino-v0.9+-blueviolet)](https://github.com/cloudwego/eino)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

**中文** | [English](README.md)

</div>

---

## Why Sounds Great AI?

你有一个代码库。它很庞大，很复杂，每天都在产生新的技术债务。

你有 Claude、GPT、Gemini —— 强大的模型，各自有独特的优势。但让它们协同工作意味着**你**变成了路由器：在聊天窗口之间复制粘贴上下文，手动追踪谁说了什么，在中间管理上浪费大量时间。

> *"我不需要一群孤狼，我需要一个紧密协作的团队。"*
> *"那就像狗狗特工队一样——忠诚、分工明确、一声令下，全员出击。"*

所以在 Go 语言与 Eino 编排引擎的框架下，**Sounds Great AI** 诞生了。

这不是一个简单的 Agent 调用框架。这是一个 **Pack** —— 一群各有特长、彼此信任、协同作战的狗狗特工队。每只狗狗都有自己的角色、性格和能力，通过 A2A 协议通信，在 DAG 工作流中紧密配合。

> *当 Agent 们完美完成一次协同，终端亮起绿色的爪印：*
> **`Sounds Great!`**

## 截图

<div align="center">

**主页**

![主页](docs/images/homepage.png)

**设置 — 成员管理**

![设置 — 成员管理](docs/images/settings-members.png)

</div>

## What It Does

| 能力 | 说明 |
|------|------|
| **CLI 适配器架构** | 4 个 CLI agent（Claude/Codex/Gemini/opencode）作为子进程启动，stdin/stdout pipe 通信，NDJSON 流解析 |
| **配置驱动角色系统** | 狗狗角色是纯 JSON 数据；`dog-template.json` 为只读种子，用户在设置页创建/修改成员落盘 `dog-catalog.json`，热加载即时生效 |
| **平台层协调** | Go + Eino 平台处理身份、路由、安全、记忆、技能 —— 平台层不做 LLM 推理 |
| **安全护栏（Hard Rails）** | 命令黑名单、路径校验、敏感数据过滤 —— 安全由代码强制执行，不依赖 prompt |
| **RAG 存储** | 3 后端（Memory/SQLite/Eino）动态切换，向量检索，30 天退役池 |
| **技能系统** | SKILL.md 提示词包从磁盘加载，注入 CLI adapter 系统提示 |
| **热加载** | 运行时注册新狗狗 → 立即生效；文件监听 + HTTP API 双路径 |
| **Eino 框架集成** | 基于 CloudWeGo Eino 的 ChatModel 接口，支持 OpenAI / Azure / 本地模型 |

## The Pack — 狗狗角色映射

六只狗狗，六个角色，各司其职：

| 角色 | 狗狗 | 性格特征 |
|------|------|----------|
| **Orchestrator** | 边牧 *(bianmu)* | 极高智商、控场大师、眼神敏锐 |
| **Safety Guardrail** | 中华田园犬 *(zhonghuatianyuanquan)* | 忠诚可靠、警惕性高、熟悉家园环境 |
| **UI / CLI Presentation** | 藏獒 *(zangao)* | 体型雄浑、威严沉稳、一夫当关 |
| **Code Hunter** | 细狗 *(xigou)* | 身形流线、极速迅猛、目标明确 |
| **RAG / Retriever** | 金毛 *(jinmao)* | 寻回本能强、温和靠谱 |
| **Log & Bug Tracer** | 德牧 *(demu)* | 警觉敏锐、黑背立耳、执行力强 |

> 用户可以创建自己的狗狗 —— 在设置页填写身份与 variants 即可，无需写代码，保存落盘 `dog-catalog.json`，热加载立即生效。

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
│              internal/platform/ (组合根)             │
│   config + router + adapters + skills + mcp + a2a   │
│   + sop + memory + ragstore + threadstore + settings│
└──────────────────────┬────────────────────────────┘
                       │ CLI Adapter Execute()
                       ▼
┌───────────────────────────────────────────────────┐
│            internal/adapter/ (CLI 适配器)           │
│   claude/    codex/    gemini/    opencode/         │
│   unified/ (ProcessManager + NDJSON 解析)           │
└──────────────────────┬────────────────────────────┘
                       │ stdin/stdout pipe
                       ▼
┌───────────────────────────────────────────────────┐
│            外部 CLI 进程                             │
│   claude CLI  |  codex CLI  |  gemini CLI  |  ...   │
└───────────────────────────────────────────────────┘
```

**三层分离原则：**

| 层 | 负责 | 不负责 |
|----|------|--------|
| **Breed JSON（数据）** | 角色身份、性格、variant 配置、模型选择 | 代码逻辑 |
| **Platform（Go + Eino）** | 身份、路由、安全、记忆、技能、协调 | LLM 推理（那是 CLI 的事） |
| **CLI Adapter** | 启动 CLI、注入 prompt、解析流、管理生命周期 | 角色定义、协调 |

> *角色是数据，平台协调，CLI 推理。*

## Quick Start

### Prerequisites

- [Go 1.26+](https://go.dev/)
- [Eino](https://github.com/cloudwego/eino) (自动安装)
- 可选：OpenAI API Key 或其他兼容模型

### Build & Run

```bash
# 1. Clone
git clone https://github.com/your-org/sounds-great-ai.git
cd sounds-great-ai

# 2. Install dependencies
go mod download

# 3. Configure
cp .env.example .env
# 编辑 .env，填入 MODEL_API_KEY 等

# 4. Run both backend and frontend
make dev
# Backend on :8080, Frontend on :5173

# Or run individually
make backend   # Go server only
make frontend  # Vite dev server only
```

Server 启动后：
- `http://localhost:8080/health` — 健康检查
- `http://localhost:8080/ws` — WebSocket 通信
- `http://localhost:8080/api/breeds` — 狗狗 CRUD API

### 升级

#### 通过 UI

点击右上角 Header 中的升级按钮（↑ 图标），选择是否拉取最新代码。

#### 通过 CLI

```bash
make upgrade
```

会提示"是否需要拉取最新的代码？(y/n)"，然后安装依赖、重新构建前端和后端。

### Create Your Own Dog

```bash
# 创建一个新狗狗
curl -X POST http://localhost:8080/api/breeds \
  -H "Content-Type: application/json" \
  -d '{
    "id": "mydog",
    "name": "mydog",
    "display_name": "我的狗狗",
    "avatar": "mydog.png",
    "personality": "活泼、好奇、什么都想试试",
    "default_variant_id": "v1",
    "variants": [
      {
        "id": "v1",
        "client_id": "claude",
        "default_model": "claude-sonnet-4-20250514",
        "system_prompt": "你是我的狗狗，负责探索新事物。"
      }
    ],
    "source": "user",
    "version": "v1"
  }'

# 立即生效，可以调用
curl -X POST http://localhost:8080/api/breeds/mydog/bark \
  -H "Content-Type: application/json" \
  -d '{ "command": "ls", "path": "/workspace" }'
```

## Roadmap

我们公开构建。以下是当前进展。

### v0: 已上线（旧架构，保留运行）

| Feature | Status |
|---------|--------|
| Pack 协调器 (Register / Bark / Validate) | Shipped |
| BreedConfig 配置驱动 (JSON 热加载) | Shipped |
| REST API (CRUD + Source 保护) | Shipped |
| WebSocket → Bark 端到端链路 | Shipped |
| 安全护栏 (CommandCheck / PathValidate) | Shipped |

### v1: 平台层（进行中）

> Spec：见 `docs/ROADMAP.md`

**已完成：**

| 包 | 说明 | 状态 |
|---|------|------|
| `internal/adapter/` | 4 个 CLI 适配器 + ProcessManager | ✅ Shipped |
| `pkg/pack/` | 品种配置 schema 与加载器（variants[] 格式） | ✅ Shipped |
| `internal/skills/` | 技能框架（.md 加载 + 注入） | ✅ Shipped |
| `internal/ragstore/` | RAG 存储（3 后端：Memory/SQLite/Eino） | ✅ Shipped |
| `internal/transport/` | WebSocket + HTTP API + SPA | ✅ Shipped |
| `internal/platform/` | 平台组合根 | ✅ Shipped |
| `internal/capability/` | 6 个纯逻辑能力（安全护栏 + 路由 + 上下文） | ✅ Shipped |
| `internal/prompt/` | System Prompt Builder + Context Assembler（5 段提示，token 预算） | ✅ Shipped |
| `internal/threadstore/` | 线程 + 消息存储（SQLite WAL + 内存，工厂模式） | ✅ Shipped |
| `internal/router/` | 动态路由引擎 + @mention 多狗狗路由 | ✅ Shipped |
| `internal/a2a/` | A2A Hub + 上下文压缩 | ✅ 最小实现 |
| `internal/sop/` | SOP 门控 + 跨模型審查 | ✅ 最小实现 |
| `internal/mcp/` | MCP 注册表 | ✅ 最小实现 |
| `internal/memory/` | 共享内存 | ✅ 最小实现 |
| `internal/settings/` | 设置存储（内存） | ✅ 最小实现 |

**多狗狗协作 — 已完成：**

| 功能 | 说明 | clowder-ai 参考 |
|------|------|----------------|
| System Prompt Builder | 5 段提示：身份 + 限制 + 队友名册 + 角色 + 技能 | SystemPromptBuilder |
| Context Assembler | 历史转 schema 消息，token 预算，截断 | ContextAssembler |
| @mention 路由 | 解析 @mention（中文+英文），按 breed config 模式路由 | AgentRouter |
| 串行执行 | 多狗狗链式：每个输出作为下一个的上下文 | route-serial |
| 并行执行 | goroutine 并发 + 共享 streamer + WaitGroup | route-parallel |
| SQLite 持久化 | WAL 模式，工厂模式，close/reopen 持久性 | ThreadStore + MessageStore |

### v2: 剩余工作

> Phase 7 主体已完成。以下子项仍在进行中。

| 工作项 | 说明 | clowder-ai 参考 | 状态 |
|--------|------|----------------|------|
| 文档治理补全 | AGENTS.md 治理机制 + Skills 补充 + per-breed 身份 + memory-philosophy 补全 | clowder-ai shared-rules + 49 skills | 进行中 |
| Hooks 内容充实 | D/L 系列 hook 模板补充实质内容 | clowder-ai prompt-hooks | 进行中 |
| RAG 按需检索 | MCP `search_knowledge` tool → RAG store → agent 按需查询 | domains/memory/（按需，非默认前置） | 规划中 |
| SOP 基础门禁 | SOPGuardian 接入执行流（review 触发、安全检查） | 五轴风险路由（简化版） | 规划中 |

## Security Audit（安全扫描）

v1 开发完成（验证清单全部通过）后，项目在发布前进行全量安全扫描。

### 工具

[codex-security](https://github.com/openai/codex-security) — OpenAI 的安全扫描 CLI 和 TypeScript SDK，用于发现、验证和修复代码安全漏洞。

### 前置条件

- v1 验证清单：全部 ✅
- `go build ./...` 通过
- `go test ./...` 通过
- `npx tsc --noEmit` 通过（前端）
- Node.js 22.13+ 已安装

### 扫描流程

```bash
# 1. 安装 codex-security
npm install @openai/codex-security

# 2. 认证登录
npx @openai/codex-security login

# 3. 基础扫描（快速，覆盖 Go 后端和 TypeScript 前端）
npx @openai/codex-security scan .

# 4. 深度扫描（全面，多 Agent，用于发布前检查）
npx @openai/codex-security scan . --mode deep --workers 2 --subagents 0 --stop-after-no-new 3 --max-discovery-runs 10
```

### 扫描范围

| 范围 | 路径 | 语言 |
|------|------|------|
| 后端 | `cmd/`, `internal/`, `pkg/` | Go |
| 前端 | `web/src/` | TypeScript/React |
| 配置 | `packs/`, `.env.example` | JSON / env |

### 修复流程

1. **分类** — 按严重程度分类每个发现（critical / high / medium / low）
2. **修复** — 发布前解决所有 critical 和 high 发现
3. **复扫** — 运行基础扫描验证修复
4. **记录** — 记录 medium/low 发现的已接受风险

### 通过标准

- 0 个 critical 发现
- 0 个 high 发现
- 所有 medium/low 发现已记录或已修复

---

## Philosophy

### Hard Rails + Dog Pack

传统框架关注**控制** —— Agent 不能做什么。Sounds Great AI 关注**协作** —— 给狗狗们一个共同的任务和执行任务的自主权。

- **Hard Rails** = 安全底线。不可协商。由代码强制执行，不依赖 prompt。
- **Dog Pack** = 底线之上，狗狗们自主协调、自主检查、自主改进。

> 安全不能依赖 prompt。田园犬检查安全是 Pack 层的 middleware，不是边牧的 prompt 里写着"请检查安全"。

### 核心原则

| # | Principle | Meaning |
|---|-----------|---------|
| P1 | 角色是数据，能力是代码 | Breed 是 JSON，Capability 是 Go，互不耦合 |
| P2 | 不改现有代码 | 适配器包装 internal/，新增能力只加不改 |
| P3 | 热加载优先 | 用户创建角色 → 立即生效，无需重启 |
| P4 | CLI adapter，不是 DAG | 狗狗通过 CLI adapter 执行，不是固定工作流 DAG |
| P5 | 安全由代码强制 | Hard Rails 在 Pack 层，不在 prompt 里 |

## Project Structure

```
sounds-great-ai/
├── cmd/
│   └── server/              # HTTP 服务器入口
├── pkg/
│   ├── a2a/                 # A2A 协议类型
│   └── pack/                # Pack/Breed 核心系统 (breed.go schema + loader.go)
├── internal/
│   ├── adapter/             # CLI 适配器 (claude/codex/gemini/opencode)
│   ├── a2a/                 # A2A Hub + 上下文压缩
│   ├── aspect/              # 安全护栏
│   ├── capability/          # 6 个纯逻辑能力
│   ├── component/           # Eino 模型工厂
│   ├── config/              # 事件总线 (config/settings 变更事件)
│   ├── mcp/                 # MCP 注册表
│   ├── memory/              # 共享内存
│   ├── packapi/             # REST API handler
│   ├── platform/            # 平台组合根
│   ├── ragstore/            # RAG 存储 (Memory/SQLite/Eino)
│   ├── router/              # 动态路由引擎
│   ├── skills/              # 技能框架
│   ├── sop/                 # SOP 门控
│   ├── settings/            # 设置存储
│   ├── threadstore/         # 线程存储
│   ├── transport/           # WebSocket + HTTP 传输层
│   ├── agent/               # Agent 实现
│   ├── tool/                # 工具
│   └── workspace/           # 工作区管理
├── packs/
│   └── default/
│       ├── breeds/          # 狗狗品种配置（dog-template.json 只读种子，运行时以 dog-catalog.json 为准）
│       └── skills/          # SKILL.md 提示词包
├── web/                     # 前端 (React + Vite)
└── docs/
    ├── architecture/        # 架构文档
    ├── decisions/           # ADR 决策记录
    ├── design/              # 设计文档
    ├── features/            # 功能文档
    └── plans/               # 实现计划
```

## Learn More

- [架构谱系](docs/architecture/architecture-lineage.md) — 全量架构主题索引
- [记忆哲学](docs/architecture/memory-philosophy.md) — 7 公理、21 定律、判据
- [角色设定](docs/design/CHARACTER-SETTING_zh-CN.md) — 狗狗角色映射表
- [故事背景](docs/design/STORY_zh-CN.md) — 狗狗特工队的诞生故事

## Contributing

欢迎贡献！请通过 Fork → branch → PR 的方式提交。

- 遵循现有代码风格和测试规范
- 新增 Capability 需要同时写适配器和测试
- 新增狗狗只需一个 JSON 文件

## 鸣谢

本项目站在巨人的肩膀上：

- **[Eino](https://github.com/cloudwego/eino)** — CloudWeGo 的 Go LLM 应用框架。编排引擎、ChatModel 接口和 schema 类型驱动着每个狗狗的智能。
- **[clowder-ai](https://github.com/clowder-ai/clowder)** — 启发了犬群的多 Agent 猫咖。他们的 A2A 协议设计、Pack 系统模式和开源理念是宝贵的参考。

## License

[MIT](LICENSE) — Use it, modify it, ship it.

---

<p align="center">
  <em>Build dog packs, not just agents.</em><br>
  <br>
  <strong>When AI Agents Bark Together, It Sounds Great.</strong>
</p>
