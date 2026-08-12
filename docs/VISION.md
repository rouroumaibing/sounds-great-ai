# VISION — Sounds Great AI

> **北极星文档。所有 spec 必须与本文档兼容，否则不得批准。**
>
> Status: Active | Last Updated: 2026-08-04

---

## 0. 为什么存在

### 0.1 想法和产品之间，隔着的不是程序员，是实现力

每个人脑子里都有想做的东西。卡住的从来不是想法，而是**把想法做成可演示产品的能力**。

以前传递想法靠 PPT。我们相信未来传递想法靠 POC、demo、真正能跑的东西。**Coding 是这个时代最直接的实现力。**

我们缺的不是更强的 AI 工具，我们缺的是一支能把想法变成产品的团队。

### 0.2 我们养的不是工具，是团队

人是作者，犬是共创放大器。梦是人的，判断是人的，愿景是人的。
犬做的是把你从"有想法但做不出来"，推到"能带着团队把它做出来"。

领养团队，不是配置工具。你和犬一起磨出来的 shared-rules、踩过的坑、沉淀的 skills，这些加在一起才是真正的生产力。

> *每一个词元（token）的价值，不以消耗衡量，以创造丈量。*

### 0.3 AI 不是让你退场，而是让你上场

Sounds Great AI 不是替你完成梦想，而是让你终于有机会带着自己的团队，把梦想做成能运行的东西。

以前没有资源、没有团队、没有舞台。现在犬队陪你一起把它做出来，带着作品走进人群。
**犬队不是让你远离人群——是让你终于有东西可以带着走上舞台。**

### 0.4 核心信条

> **Models set the ceiling. The platform sets the floor. Each layer is a multiplier, not addition.**
>
> Go + Eino 平台协调多个外部 CLI agent（Claude/Codex/Gemini/opencode），让它们像犬队一样精确协作。

---

## 1. 协作哲学

### 1.1 没有 Boss Agent

没有 Boss Agent。六只犬各有视角，自己判断该不该回应、怎么回应。
但执行有纪律：TDD、Review、门禁、愿景守护——**自由判断，结构化交付**。

### 1.2 对等协作

多犬协作不是重复劳动。不同认知风格的碰撞——归纳、发散、演绎——涌现出单一视角无法产生的洞见。
衡量团队的标准不是消耗了多少 token，而是从想法到产品之间，省掉了多少次推倒重来。

### 1.3 共享记忆，可迁移的养成经验

养成经验可以迁移。我们磨了五十多天的协作经验打包给你是 80 分，但犬会和你继续长——长成属于你自己的 100 分。
**每个人的 100 分不一样。**

### 1.4 犬队 vs 猫咖

我们用「犬队」隐喻组织工具型 agent 家族，而非沿用「猫咖」设定。原因：
- 犬有**工作本能**（herding, retrieving, guarding）——更贴合"工具型 agent"
- 犬有**团队意识**（pack）——多犬协作是天性
- 犬有**服从性**——纪律执行更可靠

---

## 2. 治理原则

### 2.1 愿景驱动

开发是**愿景驱动**的。和用户确认了 feature 的愿景后：

- **没达成愿景 = 没完成**，不交半成品，不半路问"要不要继续"
- 停下来的正当理由：解决不了的阻塞（技术限制/外部依赖）→ 升级用户；方向存疑 → 停手重估
- 判断力允许停，惰性不允许

### 2.2 Phase 碰头机制

大 scope feature（3+ Phase）不能等最后才对齐愿景。**每个 Phase merge 后**，主动和用户碰头：

```
Phase N merge → 碕头（不是"要不要继续"，是"方向对不对"）→ 继续 Phase N+1
```

**碰头格式**（轻量，不是报告会）：
1. **成果展示**：这个 Phase 做了什么
2. **愿景进度**：离最终愿景还差什么
3. **下个 Phase 方向**：下一步计划，有没有发现新问题
4. **方向确认**："方向对吗？有没有要调整的？"

小 Feature（1-2 Phase）：不需要碰头，直接做到底 → 愿景守护 → close。

### 2.3 风险路由

**强制力跟着风险走，不跟着动作类型走。** "写了代码""开了 PR""进入 merge"都不能单独触发整条流水线。默认是最小安全动作；只有命中客观风险面才进入对应加严车道。

### 2.4 教训沉淀

踩过的坑必须沉淀为可复用、可追溯的教训条目（LL-XXX 格式），避免重复踩坑。
教训必须有**可执行防护**——不能只写"注意"，必须有可执行动作。

---

## 3. 三层原则

```
+--------------------------------------------------+
|                  You (CVO)                       |
|         Vision - Decisions - Feedback            |
+----------------------+---------------------------+
                       |
+----------------------v---------------------------+
|            Dog Pack Platform Layer (Go+Eino)     |
|                                                  |
|  Identity Manager | A2A Router | Skills Manifest |
|  Memory & Evidence| SOP Guardian| MCP Bridge     |
|  Dynamic Routing  | Review Gates| CLI Adapter    |
+----+----------+----------+----------+------------+
     |          |          |          |
+----v---+ +---v----+ +---v----+ +---v------+
|Claude  | |Codex   | |Gemini  | |opencode  |
|Code CLI| |CLI     | |CLI     | |(any model)|
+--------+ +--------+ +--------+ +----------+
```

| 层 | 职责 | 不职责 |
|---|------|--------|
| **Model**（CLI 内） | 推理、生成、理解 | 长期记忆、纪律 |
| **Agent CLI** | 工具调用、文件操作、命令执行、MCP | 团队协调、review |
| **Platform**（Go+Eino） | 身份、协作、纪律、审计、路由 | 推理（那是 CLI 的事） |

## 4. 不可逆决策

以下决策已锁定，后续 spec 不得推翻：

1. **CLI adapter 架构** — 平台 spawn 外部 CLI 进程，通过 stdin/stdout pipe 通信，解析各自输出格式。不内置 agent reasoning。
2. **动态路由，非固定 DAG** — 平台根据任务类型动态决定调用哪些 agent。不硬编码工作流。
3. **Dog personas 保留** — 6 个狗狗各自承载 personality + role + CLI binding。
4. **Go + Eino 平台语言** — 平台层用 Go + Eino。平台自身需要的 LLM 调用（路由、分解、合成）走 Eino。

## 5. Dog Pack — 狗狗角色

| 角色 | 狗狗 | 代号 | 职责 |
|------|------|------|------|
| Orchestrator | Border Collie | bianmu | 任务分解、动态路由、结果合成 |
| Code Hunter | Xigou | xigou | 代码搜索、分析、重构建议 |
| Knowledge Retriever | Golden Retriever | jinmao | RAG 检索、上下文组装 |
| Log Tracer | German Shepherd | demu | 日志追踪、错误诊断 |
| Presenter | Tibetan Mastiff | zangao | 输出格式化、markdown 渲染 |
| Safety Guard | Chinese Rural Dog | zhonghuatianyuanquan | 命令拦截、路径校验、敏感过滤 |

## 6. 平台能力清单

| 模块 | 职责 | 包路径 |
|------|------|--------|
| CLI Adapter | spawn/pipe/parse 4 个 CLI | `internal/adapter/` |
| Identity Manager | 狗狗身份、persona 持久化 | `internal/config/` |
| Dynamic Router | 任务→agent 路由决策 | `internal/router/` |
| SOP Guardian | SOP 规则、门禁、review 策略 | `internal/sop/` |
| Memory & Evidence | 证据存储、决策日志、经验 | `internal/memory/` |
| Memory Lanes | 6-organ typed memory lanes (taste/profile/entity/person/event/decision/lesson) + delta producer + human disposition + consumption tracker + lifecycle trace + private initiative | `internal/memory/lanes.go` `internal/memory/supply.go` `internal/memory/feedback.go` `internal/memory/initiative.go` |
| Cue Plane | F287 recall opportunity catalog (closed typed predicates) + cue envelope (budget/dedupe/expiry) + lane resolver registry + consumption episode ledger + source invalidation (fail-closed) | `internal/cue/` |
| ACP Process Pool | F149 process pool keyed by (projectPath, providerProfile) + lease mechanism (acquire/release, TTL, LRU) + health check (zombie cleanup, cwd validation) + metrics (warmHit/coldStart/eviction/zombie) | `internal/adapter/pool/` |
| Skills Manifest | 按需 skill prompt 加载 | `internal/skills/` |
| MCP Bridge | MCP server + client 桥接 | `internal/mcp/` |
| RAG Store | 向量存储、embedding、检索 | `internal/ragstore/` |
| Thread Store | 线程、会话、事件存储 | `internal/threadstore/` |
| Settings Store | 成员、账户、系统配置 | `internal/settings/` |
| Prompt Hooks | hook 声明、注入管道、轨迹记录 | `internal/hooks/` |
| A2A Hub | agent 间通信 | `internal/a2a/` |
| Eval Framework | harness eval 控制面、verdict 闭环、N-day 调度 | `internal/eval/` |
| Eval Domains | eval domain YAML 定义（5 个初始 domain） | `packs/default/evals/` |
| Ops Monitor | 运维监控、日志缓冲、健康状态 | `internal/ops/` |
| Telemetry | OpenTelemetry 可观测性：traces ring buffer + metrics + Prometheus exporter + 30s 快照 + HMAC 伪匿名化。Graceful degradation：init 失败不 crash。Phase 7 扩展 | `internal/telemetry/` |

> **运行时成员数据统一（2026-08-12）**：成员/狗狗身份数据统一持久化于 `.sounds-great-ai/dog-catalog.json`，结构为 clowder 同构的 `{version, breeds[], roster{}, review_policy, leader, configs[]}`（类型定义见 `pkg/pack`）。`packs/default/breeds/dog-template.json` 降级为只读种子（`role_templates` / `client_defaults` 仍供模板 UI；`breeds/roster/leader/review_policy` 仅首启复制进 catalog）。breed 相关类型已从 `internal/config` 剥离至 `pkg/pack`；`internal/config` 现仅承载事件总线（`event_bus.go`）。本变更未引入新的 `internal/` 顶层目录，未推翻不可逆决策（VISION §4/§7）。详见 `sg-member-catalog-plan.md`。

## 7. 路线图

| Phase | 目标 | 状态 |
|-------|------|------|
| **1. Platform Infra** | CLI adapter + config + router + SOP + skills + memory + MCP + hooks | **完成** |
| **2. RAG Integration** | 向量存储接入平台、context_assemble、FTS5 混合检索 | **完成** |
| **3. A2A Coordination** | 多 agent 动态协作、@mention 路由 | **完成** |
| **4. Skills System** | skill 加载、注入、外部 skill 吸收 | **完成** |
| **5. SOP Gates** | 质量门禁、review 流程、安全策略 | **完成** |
| **6. Transport** | WebSocket + HTTP API + 前端 | **完成** |
| **7. Polish** | 文档、示例、性能优化、45 hook、Memory System + Cue Plane + ACP Pool + PWA | **主体完成，剩余子项** |

### 7.1 Phase 7 剩余子项

| 子项 | 状态 |
|------|------|
| 45 hook 模板 | 已创建，D/L 系列内容已充实 |
| Memory System (Lanes + Cue Plane) | 已实现，文档已补全 |
| ACP Process Pool | 已实现 |
| PWA | 已实现 |
| 文档治理（AGENTS.md 治理机制补全） | 完成 |
| Skills 补充（5→25） | 完成 |
| RAG on-demand 检索 | 规划中 |
| SOP 基础门禁接入执行流 | 规划中 |

## 8. 愿景合规

**不论是否走 spec 流程，所有工作必须与本文档兼容。** agent 不一定写 spec——直接改代码也受约束。

### 8.1 走 spec 流程时

Spec 必须包含 `## VISION Compatibility` 段，回答以下 7 个问题：

- [ ] 与 §0 哲学兼容？（服务用户实现力，不是替代用户）
- [ ] 与 §1 协作哲学兼容？（对等协作，非 Boss Agent）
- [ ] 与 §2 治理原则兼容？（愿景驱动、Phase 碰头、风险路由）
- [ ] 与 §3 三层原则兼容？（不把推理放进平台层）
- [ ] 与 §4 不可逆决策兼容？（不重新引入 DAG / 内置 reasoning / 非 CLI 架构）
- [ ] 在 §7 路线图的哪个 Phase？该 Phase 的前置条件已满足？
- [ ] 是否引入新的不可逆决策？如果是，更新本文档。

### 8.2 不走 spec 流程时（直接改代码）

遵守 `AGENTS.md` 的 Vision Check Protocol：

- **开工前**：读 VISION.md，4 项兼容性自检。不兼容时停下报告。
- **提交前**：检查代码结构 vs §4/§6/§7。不兼容时不提交。
- **架构变更**：必须先更新 VISION.md。未更新就改 = 违反铁律 5。

### 8.3 执行机制

| 机制 | 覆盖场景 | 状态 |
|------|----------|------|
| `AGENTS.md` Vision Check Protocol | agent 直接改代码 | **已建立** |
| Spec 模板 + §8.1 检查清单 | agent 走 spec 流程 | 待建立模板 |
| Prompt hooks（per-turn 注入） | 每轮自动注入身份 + 铁律 + 护栏 | **已实现**（§8.4） |

**已有经验**：最有效的机制是 hooks（`disableable: false`, `governanceTier: immutable`）——系统注入，agent 无法绕过。我们当前靠 AGENTS.md 自觉，未来应建立 hook 系统实现结构化强制。

### 8.4 Prompt Hooks 系统设计（已实现）

> 实现路径：`internal/hooks/` + `packs/default/hooks/`。

**目标**：在 CLI adapter spawn 时，通过 stdin 注入结构化 prompt 片段，实现 agent 无法绕过的身份 + 铁律 + 护栏注入。

**注入时机**：
- `session-init`：会话启动时注入身份、铁律、限制声明
- `per-turn`：每轮注入红旗模式、Phase 约束

**hook 清单（设计）**：

| hook id | 时机 | 内容 | 参考实现 |
|---------|------|------|----------------|
| `identity` | session-init | 狗狗身份 + 职责 | S1 |
| `restrictions` | session-init | 限制声明表 | S2 |
| `iron-laws` | session-init | 5 条铁律 | L4 |
| `guardrails` | session-init | 红旗模式表 | S10 |
| `roster` | session-init | 队友名册 | S5 |
| `governance` | session-init | 治理摘要（VISION §0-§4） | S9 |
| `mcp-tools` | session-init | MCP 工具列表 | S13 |
| `a2a-format` | session-init | A2A 协作格式 | S4 |
| `phase-anchor` | per-turn | 当前 Phase + 前置条件 | D14 |
| `re-anchor` | per-turn | 长任务重锚定提醒 | D1 |

> **per-turn hooks 已接入**：session-init hooks（S1-S8）在会话启动时注入身份、铁律、限制、护栏、名册、治理、MCP 工具、A2A 格式。per-turn hooks（D1-D2）每轮注入 Phase 锚定和重锚定提醒。session-init hooks 通过 native L0 通道（Claude `--append-system-prompt`、Codex `-c developer_instructions`）注入，压缩免疫；Gemini/OpenCode 走 stdin 前插 fallback。TraceStore（SQLite）记录每次注入的 fire/skip 事件。

**实现路径**：
1. `internal/hooks/` Registry 扫描 `packs/default/hooks/` 下的 hook.yaml 定义
2. Pipeline 按 `stage` 和 `order` 过滤并执行适用的 hooks
3. `ws_handler.go` 执行 session-init + per-turn hooks，按 CLI 类型路由注入
4. Native L0：Claude `--append-system-prompt`、Codex `-c developer_instructions`（压缩免疫）
5. Fallback：Gemini/OpenCode 走 stdin 前插
6. `disableable: false` 的 hook 不可被 agent 跳过
7. `TraceStore`（SQLite）记录每次注入的 fire/skip 事件

**与 AGENTS.md 的关系**：AGENTS.md 是 hook 内容的**真相源**。hook 系统实现后，AGENTS.md 的"长任务重锚定"段从自觉规则升级为系统注入。

---

> **当 agent 完美完成一次协作，终端亮起绿色爪印：Sounds Great!**
