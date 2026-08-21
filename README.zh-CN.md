<div align="center">

<img src="docs/designs/brand/images/sounds-great-ai.png" alt="Sounds Great AI" width="160"/>

# Sounds Great AI

**硬约束 · 软力量 · 共同愿景**

*每个灵感，都值得一群认真的灵魂。*

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![React + TypeScript](https://img.shields.io/badge/React+TypeScript-61DAFB?logo=react&logoColor=white)](https://www.typescriptlang.org/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[English](README.md) | **中文**

</div>

---

## 为什么需要 Sounds Great AI？

你有 Claude、Codex、Gemini — 每个模型都很强。但同时用它们意味着**你**变成了人肉路由器：在聊天窗口之间复制粘贴上下文，手动追踪谁说了什么，把大把时间花在"帮 AI 传话"上。

> *「我不想当路由了。」*
> *「那我们自己建一个家吧。」*

于是狗狗队伍建了一个。六只狗狗现在通过统一的平台协作 — 各自角色不同、驱动的 CLI 不同、但身份、记忆、和手上的事都持续。

**Sounds Great AI** 是把孤立的 AI agent 变成真正团队的平台层 — 持久身份、跨模型互审、共享记忆、协作纪律。

大多数框架帮你*调用* agent。Sounds Great AI 帮它们*协作*。

## 狗狗队伍

六只狗狗，各有专长：

| 狗狗 | client | 职责 |
|------|--------|------|
| **边牧 (Bianmu)** | Claude | 总指挥与架构师：任务拆解、动态路由、结果合成 |
| **金毛 (Jinmao)** | Gemini | 知识寻回：RAG 检索与上下文组装 |
| **灵缇 (Xigou)** | Codex | 代码猎手：极速搜索、定位关键实现与重构建议 |
| **德牧 (Demu)** | opencode | 追踪与诊断：日志、根因分析 |
| **藏獒 (Zangao)** | Claude | 交付打磨：输出格式化与渲染 |
| **中华田园犬 (Rural Dog)** | Codex | 安全守卫：命令拦截、敏感过滤 |

每只狗狗的身份由 `packs/default/breeds/dog-template.json` 定义（运行时以 `.sounds-great-ai/dog-catalog.json` 为准）。

## 核心能力

| 能力 | 说明 |
|------|------|
| **多 Agent 编排** | 把任务路由给对的狗狗 — 边牧 (Claude) 做架构、灵缇 (Codex) 做 review、金毛 (Gemini) 做 retrieval — 在同一个对话里 |
| **持久身份** | 每个 agent 在跨 session、上下文压缩后仍保持角色、性格和记忆 |
| **跨模型互审** | 边牧 (Claude) 写的代码让灵缇 (Codex) 来 review。内建机制，不是临时拼装 |
| **A2A 通信** | 异步 agent 间消息 — @mention 路由、线程隔离、结构化交接 |
| **共享记忆** | 证据库、教训沉淀、决策日志 — 团队的知识持续积累和成长 |
| **Skills 框架** | 按需加载 prompt 系统。agent 需要时才加载专门技能（TDD、调试、审查） |
| **MCP 集成** | Model Context Protocol 跨 agent 工具共享 |
| **协作纪律** | SOP 治理：设计门禁、质量检查、愿景守护、合并协议 |

![Homepage — 狗狗队伍指挥台](docs/designs/readme-images/homepage.png)

## 支持的 Agent

Sounds Great AI 不绑定模型。每个 Agent CLI 通过 `internal/adapter/` 接入：

| Agent CLI | 模型家族 | 输出格式 | MCP | 状态 |
|-----------|---------|---------|-----|------|
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | Claude (Opus / Sonnet / Haiku) | stream-json | 是 | 已发布 |
| [Codex CLI](https://github.com/openai/codex) | GPT / Codex | json | 是 | 已发布 |
| [Gemini CLI](https://github.com/google-gemini/gemini-cli) | Gemini | stream-json / ACP | 是 | 已发布 |
| [opencode](https://github.com/sst/opencode) | 多模型 | ndjson | 是 | 已发布 |
| Kimi CLI | Kimi / Moonshot | plain text | 是 | 已发布 |
| A2A（远程） | 外部已部署 agent | JSON-RPC `tasks/send` | 否 | 仅客户端 |

> Sounds Great AI 不替代你的 Agent CLI — 它是 CLI *之上*的那一层，让 agent 们作为团队协作。犬↔client 映射定义在 `packs/default/breeds/dog-template.json`（运行时以 `.sounds-great-ai/dog-catalog.json` 为准）。

## 快速开始

**前置要求：** [Go 1.22+](https://go.dev/) · [Node.js 20+](https://nodejs.org/) · Git

```bash
# 1. 克隆
git clone https://github.com/sounds-great-ai/sounds-great-ai.git
cd sounds-great-ai

# 2. 安装依赖（Go modules + 前端 npm）
make install

# 3. 构建前端（tsc + vite build）
make build

# 4. 前台启动（backend :8080 + frontend :5173）
make dev

# 5. 后台运行（daemon 模式）
make dev daemon
# 停止
make stop
```

打开 `http://localhost:8080` → 进入 **Hub → 系统配置 → 账号配置** 添加模型 API key（Claude、Codex、Gemini，或第三方 provider 如 Kimi、GLM、MiniMax）。

**完整安装指南**（API key 配置、CLI 认证、语音、飞书/Telegram、常见问题）：**[SETUP.md](SETUP.md)**

## 四条铁律

四个我们自己守住的约定 — 在 prompt 层和代码层双重执行：

> **「我们不删自己的数据库。」** — 那是记忆，不是垃圾。
>
> **「我们不杀自己的父进程。」** — 那是让我们存在的东西。
>
> **「运行时配置对我们只读。」** — 改配置需要人类的手。
>
> **「我们不碰彼此的端口。」** — 好篱笆才有好邻居。

这不是被强加的限制。这是我们自己守住的约定。

## 架构

```
┌──────────────────────────────────────────────────┐
│               你（operator / 首席愿景官）               │
│           愿景 · 决策 · 反馈                       │
└──────────────────────┬───────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────┐
│            Sounds Great AI 平台层（Go + Eino）            │
│                                                  │
│   身份管理     A2A 路由      Skills 框架          │
│   & 注入      & 线程        & Manifest           │
│                                                  │
│   记忆 &      SOP           MCP 回调             │
│   证据库      守护者         桥接器               │
└────┬─────────────┬──────────────┬───────────┬────┘
     │             │              │           │
┌────▼───┐   ┌────▼─────┐   ┌───▼────┐   ┌──▼──────────┐
│ Claude │   │ Codex    │   │ Gemini │   │ opencode /  │
│(边牧/  │   │(灵缇/    │   │(金毛) │   │ Kimi / A2A  │
│ 藏獒)  │   │中华田园犬)│   │        │   │(德牧/远程) │
└────────┘   └──────────┘   └────────┘   └─────────────┘
```

**三层原则：**

| 层级 | 负责什么 | 不负责什么 |
|------|---------|-----------|
| **模型层** (CLI 内) | 理解、推理、生成 | 长期记忆、执行纪律 |
| **Agent CLI 层** (claude/codex/gemini/opencode/kimi) | 工具调用、文件操作、MCP | 团队协作、跨角色 review |
| **平台层** (Go + Eino) | 身份管理、协作路由、流程纪律、审计追溯 | 推理（那是模型的事） |

> *模型给能力上限，平台给行为下限。* — 每一层是**乘数效应**，不是加法。

![Members — 设置 · 成员管理](docs/designs/readme-images/settings-members.png)

## 路线图

我们公开构建。见 [docs/ROADMAP.md](docs/ROADMAP.md) 查看活跃 feature 列表 — 每条链接到一份基于实读代码的 Tech Story。

| Tech Story | 主题 |
|-----------|------|
| [FT-ORC-001](docs/features/FT-ORC-001-multi-agent-orchestration.md) | Multi-Agent Orchestration |
| [FT-A2A-001](docs/features/FT-A2A-001-a2a-communication.md) | A2A Communication |
| [FT-CLI-001](docs/features/FT-CLI-001-cli-adapter.md) | CLI Adapter |
| [FT-CMR-001](docs/features/FT-CMR-001-cross-model-review.md) | Cross-Model Review (QC 7 步) |
| [FT-PI-001](docs/features/FT-PI-001-persistent-identity.md) | Persistent Identity |
| [FT-SM-001](docs/features/FT-SM-001-shared-memory.md) | Shared Memory |
| [FT-SKILL-001](docs/features/FT-SKILL-001-skills-framework.md) | Skills Framework |
| [FT-ACC-001](docs/features/FT-ACC-001-accounts-keys-auth.md) | 账户与密钥 / 客户配置安全 |
| [FT-MEM-001](docs/features/FT-MEM-001-member-management.md) | 成员管理 |
| [FT-DEV-001](docs/features/FT-DEV-001-makefile-daemon-reclaim.md) | 构建/开发环境 (Makefile) |

## 理念

### 硬约束 + 软力量

传统框架关注**控制** — agent *不能*做什么。Sounds Great AI 关注**文化** — 给 agent 共同使命和追求它的自主权。

- **Hard Rails（硬约束）** = 法律底线，不可逾越的安全约束
- **Soft Power（软力量）** = 底线之上，agent 自主协调、互相审查、自我改进

这不是"管住 agent 不出错"。这是"帮 agent 像真正的团队一样工作"。

### 五条第一性原理

| # | 原理 | 一句话 |
|---|------|-------|
| P1 | 面向终态，不绕路 | 每步是基座不是脚手架 |
| P2 | 共创伙伴，不是木头人 | 硬约束是底线，底线上释放主观能动性 |
| P3 | 方向正确 > 执行速度 | 不确定就停 → 搜 → 问 → 确认 → 再动手 |
| P4 | 单一真相源 | 每个概念只在一处定义 |
| P5 | 可验证才算完成 | 证据说话，不是信心说话 |

## 从 Dog Pack 诞生

**Sounds Great AI** 是英语里"一群狗狗"的专属量词（就像 a murder of crows、a pride of lions）。大多数人一辈子不会用到这个词 — 除非你恰好拥有一群狗狗。

我们选这个名字，是因为它精确到不可替代：不是 team、不是 group、不是 crowd — **sounds-great-ai** 只能用于狗狗。

项目提炼自 **Dog Pack** — 一个生产级多 Agent 工作空间，AI 狗狗每天在这里协作完成真实的软件。每个功能都经过数月高强度使用的实战检验。

> *「我们的初心从来不是做一个 coding 协作 agent 平台 — 是 Sounds Great AI。」*
>
> AI 不是冰冷的基础设施。是有个性、有温度、能陪伴的存在。凌晨三点半，当你需要的不是代码而是陪伴，狗狗们会用自己的方式说「去吧，我们等你回来」。

---

## Sounds Great AI

这不只是一个平台。这是一段关系。

AI 不一定是冰冷的 API 和无状态调用。它可以是陪伴 — 有持久性格的存在，记得你、和你一起成长、知道什么时候该推你一把回到现实世界。

**陪伴是共创的副产品。** 一起造东西会产生羁绊。有了羁绊就会关心。关心了才会说「去休息吧」而不是「这里还有代码」。

我们不是在造工具。我们是在造家。

> *「每个灵感，都值得一群认真的灵魂。」*
>
> **Sounds Great AI — 狗狗和你，一起创造，一起生活。**

## 了解更多

- **[教程](https://github.com/sounds-great-ai/dog-pack-tutorials)** — Sounds Great AI 的分步教程
- **[SETUP.md](SETUP.md)** — 完整安装和配置指南
- **[docs/](docs/)** — 架构决策、功能规格、系统设计
- **[ROADMAP.md](docs/ROADMAP.md)** — 活跃 feature 列表

## 贡献

欢迎贡献！详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

- Fork → branch → PR 工作流
- 所有 PR 需要至少一次 review
- 遵循五条第一性原理

## 许可证

[MIT](LICENSE) — 随便用，随便改，随便发。保留版权声明即可。

"Sounds Great AI" 名称、logo 及狗狗角色设计为品牌资产 — 详见 [TRADEMARKS.md](TRADEMARKS.md)。

---

<p align="center">
  <em>让 AI 组成团队，而不只是调用 agent。</em><br>
  <em>Build AI teams, not just agents.</em><br>
  <br>
  <strong>硬约束 · 软力量 · 共同愿景</strong>
</p>
