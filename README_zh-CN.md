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

## What It Does

| Capability | What It Means |
|------------|---------------|
| **Config-Driven 角色系统** | 犬种角色是纯 JSON 数据，用户可在页面上创建/修改/删除狗狗，热加载立即生效，无需重启 |
| **A2A 多 Agent 协作** | 异步 Agent 间通信，支持 @mention 路由、线程隔离、SSE 流式传输、结构化交接 |
| **DAG 工作流引擎** | 每个犬种定义自己的工作流（拓扑排序 + 并行执行 + 依赖传递），非简单线性调用 |
| **安全护栏（Hard Rails）** | 命令黑名单、路径校验、沙盒隔离 —— 安全不依赖 prompt，由代码强制执行 |
| **Capability 适配器** | 薄适配器层包装现有代码，新增能力只需实现接口 + 注册，不改现有代码 |
| **热加载** | 运行时注册新犬种 → 立即生效；文件监听 + HTTP API 双路径 |
| **Eino 框架集成** | 基于 CloudWeGo Eino 的 ChatModel 接口，支持 OpenAI / Azure / 本地模型 |

## The Pack — 犬种角色映射

六只狗狗，六个角色，各司其职：

| 角色 | 犬种 | 性格特征 | 核心职责 | Capabilities |
|------|------|----------|----------|-------------|
| **Orchestrator** | 边牧 *(bianmu)* | 极高智商、控场大师、眼神敏锐 | 任务拆解、DAG 工作流调度、状态机控制 | task_decompose, agent_dispatch, result_merge |
| **Safety Guardrail** | 中华田园犬 *(zhonghuatianyuanquan)* | 忠诚可靠、警惕性高、熟悉家园环境 | 安全边界、命令黑名单、权限代码审计 | command_check, path_validate, sensitive_filter |
| **UI / CLI Presentation** | 藏獒 *(zangao)* | 体型雄浑、威严沉稳、一夫当关 | TUI 状态框渲染、日志大盘展示、人类确认 | format_output, render_markdown, stream_response |
| **Code Hunter** | 细狗 *(xigou)* | 身形流线、极速迅猛、目标明确 | 自动化 Refactor、高难度 Bug 修复代码生成 | code_search, code_analyze, refactor_suggest |
| **RAG / Retriever** | 金毛 *(jinmao)* | 寻回本能强、温和靠谱 | 向量检索、上下文叼取、文档关联 | rag_search, rag_index, context_assemble |
| **Log & Bug Tracer** | 德牧 *(demu)* | 警觉敏锐、黑背立耳、执行力强 | Panic 追查、StackTrace 分析、Log 溯源 | log_trace, error_diagnose, performance_profile |

> 用户可以创建自己的狗狗 —— 只需一个 JSON 文件，选择已注册的 capability，定义工作流，热加载立即生效。

## Architecture

```
┌───────────────────────────────────────────────────┐
│                 packs/default/breeds/               │
│   bianmu.json  zhonghuatianyuanquan.json  zangao.json │
│   xigou.json   jinmao.json              demu.json      │
│          (纯数据，用户可创建/修改/热加载)              │
└──────────────────────┬────────────────────────────┘
                       │ LoadFromDir / POST API
                       ▼
┌───────────────────────────────────────────────────┐
│                 Pack (协调者)                        │
│   registry: map[string]*BreedConfig                 │
│   capabilities: map[string]Capability               │
│   mu: sync.RWMutex                                  │
│                                                     │
│   Bark(breedID, input) → DAG 拓扑排序 → 并行执行     │
└──────────────────────┬────────────────────────────┘
                       │ Capability.Run()
                       ▼
┌───────────────────────────────────────────────────┐
│            internal/capability/ (适配器)             │
│   CommandCheck  PathValidate  StreamResponse        │
│   CodeSearch    TaskDecompose  ...                  │
│   (薄适配器，包装 internal/ 现有代码)                 │
└──────────────────────┬────────────────────────────┘
                       │ 调用
                       ▼
┌───────────────────────────────────────────────────┐
│            internal/ (现有代码，不改)                │
│   aspect/  transport/  agent/  tool/  workspace/    │
│   a2a/  component/                                  │
└───────────────────────────────────────────────────┘
```

**三层分离原则：**

| Layer | Responsible For | Not Responsible For |
|-------|----------------|---------------------|
| **Breed JSON (数据)** | 角色身份、性格、模型配置、能力声明、工作流定义 | 代码逻辑 |
| **Capability (Go 代码)** | 具体能力实现，包装现有 internal/ 代码 | 角色定义、任务调度 |
| **Pack (协调者)** | 注册管理、DAG 调度、热加载、安全校验 | 具体能力实现 |

> *角色是数据，能力是代码。用户定义"谁"，系统决定"怎么做"。*

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

# 4. Run server
go run cmd/server/main.go

# 5. Or run A2A multi-agent test
go run cmd/a2a-test/main.go
```

Server 启动后：
- `http://localhost:8080/health` — 健康检查
- `http://localhost:8080/ws` — WebSocket 通信
- `http://localhost:8080/api/breeds` — 犬种 CRUD API

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
    "system_prompt": "你是我的狗狗，负责探索新事物。",
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

# 立即生效，可以调用
curl -X POST http://localhost:8080/api/breeds/mydog/bark \
  -H "Content-Type: application/json" \
  -d '{ "command": "ls", "path": "/workspace" }'
```

## Roadmap

我们公开构建。以下是当前进展。

### Core Platform

| Feature | Status |
|---------|--------|
| Pack 协调器 (Register / Bark / Validate) | Shipped |
| BreedConfig 配置驱动 (JSON 热加载) | Shipped |
| DAG 工作流引擎 (拓扑排序 + 并行执行) | Shipped |
| Capability 生命周期 (Init / Health / Close) | Shipped |
| REST API (CRUD + Source 保护) | Shipped |
| A2A 多 Agent 协议 (SSE + Polling) | Shipped |
| 安全护栏 (CommandCheck / PathValidate) | Shipped |
| 文件监听热加载 (fsnotify) | Planned |

### Capability Adapters

| Adapter | Status | Wraps |
|---------|--------|-------|
| command_check | Shipped | internal/aspect/command_guard.go |
| path_validate | Shipped | internal/aspect/command_guard.go |
| sensitive_filter | Planned | internal/aspect/ |
| task_decompose | Planned | internal/a2a/orchestrator/ |
| agent_dispatch | Planned | internal/a2a/ |
| result_merge | Planned | internal/a2a/ |
| code_search | Planned | internal/tool/ (grep/glob) |
| code_analyze | Planned | internal/agent/coder.go |
| refactor_suggest | Planned | internal/agent/coder.go |
| rag_search | Planned | Vector DB integration |
| rag_index | Planned | Vector DB integration |
| context_assemble | Planned | — |
| format_output | Planned | internal/transport/ |
| render_markdown | Planned | internal/transport/ |
| stream_response | Planned | internal/transport/ws_handler.go |
| log_trace | Planned | internal/aspect/tracing.go |
| error_diagnose | Planned | — |
| performance_profile | Planned | — |

### Server Integration

| Feature | Status |
|---------|--------|
| cmd/server 集成 Pack 系统 | Shipped |
| setupPack() 初始化 + LoadFromDir | Shipped |
| /api/breeds 路由挂载 | Shipped |
| WebSocket → Bark 端到端链路 | Shipped |

### v2: Pack Runtime Execution Model

| Feature | Target |
|---------|--------|
| Workflow Engine (retry/timeout/cancellation/checkpoint) | v2 |
| Error Model (ErrorCode enum) | v2 |
| Observability (ExecutionRecord / trace) | v2 |
| Permission Middleware (Safety 强制层) | v2 |
| Plugin/WASM Capability 扩展 | v2 |
| Prompt 版本管理 / A/B test | v2 |
| DB 存储替代 JSON 文件 | v2 |
| Contract Test (config 兼容性) | v2 |

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
| P4 | DAG 不是线性 | 工作流支持依赖、并行、传递，非简单 for 循环 |
| P5 | 安全由代码强制 | Hard Rails 在 Pack 层，不在 prompt 里 |

## Project Structure

```
sounds-great-ai/
├── cmd/
│   ├── server/              # HTTP 服务器入口
│   └── a2a-test/            # A2A 多 Agent 测试入口
├── pkg/
│   ├── a2a/                 # A2A 协议类型 (AgentCard, Task, Message)
│   └── pack/                # Pack/Breed 核心系统
│       ├── breed.go         # BreedConfig 数据模型
│       ├── capability.go    # Capability 接口 + TaskInput/Output
│       ├── pack.go          # Pack 协调器 (Register/Bark/Validate)
│       ├── workflow.go      # DAG 工作流执行器
│       └── loader.go        # JSON 文件加载器
├── internal/
│   ├── a2a/                 # A2A 实现 (server/client/orchestrator)
│   ├── aspect/              # 安全护栏 (command_guard, approval, tracing)
│   ├── capability/          # Capability 适配器 (command_check, path_validate)
│   ├── component/           # Eino 模型工厂
│   ├── packapi/             # REST API handler
│   ├── transport/           # WebSocket 传输层
│   ├── agent/               # Agent 实现 (coder, skill_manager)
│   ├── tool/                # 工具 (fs_tools, terminal_tools)
│   └── workspace/           # 工作区管理 (manager, sandbox, pty)
├── packs/
│   └── default/
│       └── breeds/          # 6 个犬种 JSON 配置
└── docs/
    ├── design/              # 设计文档 (character-setting, story)
    └── superpowers/
        ├── specs/           # 技术规格文档
        └── plans/           # 实现计划文档
```

## Learn More

- [Pack/Breed 系统设计文档](docs/superpowers/specs/2026-07-31-pack-breed-system-design.md) — 完整的 Config-Driven 角色系统设计
- [A2A 多 Agent 设计文档](docs/superpowers/specs/2026-07-30-a2a-multi-agent-design.md) — A2A 协议实现规格
- [角色设定](docs/design/CHARACTER-SETTING_zh-CN.md) — 犬种角色映射表
- [故事背景](docs/design/STORY_zh-CN.md) — 狗狗特工队的诞生故事

## Contributing

欢迎贡献！请通过 Fork → branch → PR 的方式提交。

- 遵循现有代码风格和测试规范
- 新增 Capability 需要同时写适配器和测试
- 新增犬种只需一个 JSON 文件

## 鸣谢

本项目站在巨人的肩膀上：

- **[Eino](https://github.com/cloudwego/eino)** — CloudWeGo 的 Go LLM 应用框架。编排引擎、ChatModel 接口和 schema 类型驱动着每个犬种的智能。
- **[clowder-ai](https://github.com/clowder-ai/clowder)** — 启发了犬群的多 Agent 猫咖。他们的 A2A 协议设计、Pack 系统模式和开源理念是宝贵的参考。

## License

[MIT](LICENSE) — Use it, modify it, ship it.

---

<p align="center">
  <em>Build dog packs, not just agents.</em><br>
  <br>
  <strong>When AI Agents Bark Together, It Sounds Great.</strong>
</p>
