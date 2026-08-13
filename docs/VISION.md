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

### 5.1 首启空、按需组队

六犬是**模板 / 菜单**（canonical personas），不是出厂即部署。首次启动 `dog-catalog.json` 为空（仅 owner），用户通过「成员管理 → 从模板添加」把犬加入团队、绑定账号与密钥后，犬才进入运行时。这让「我们养的是团队」（§0.2）落到行为层：团队由用户亲手组建，而非预先灌满。

- **可用性 = 用户开关 ∧ 凭据就绪**。凭据就绪：`oauth` 账号查 CLI 二进制，`api_key` 账号查 `~/.sounds-great-ai/credentials.json`。无凭据的犬显示「待配置」而非「已启用」。
- **升级新增的模板犬自动加入 catalog**（`seen_template_breeds` 机制），但同样需绑定凭据才「就绪」。

见 `docs/decisions/ADR-001` 与 `docs/DESIGN-STORYS/SG-MEM-001-member-management.md`（§6 首启空 Catalog + 凭据就绪闸门）。

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
| Ball Custody Ledger | 球权事件账本（append-only 事件流 + 纯函数投影状态机）+ CAS 租约；编排一等状态源。复用 `internal/domains/custody` 既有实现并接入运行时 | `internal/domains/custody/` |
| Orchestration Domains | threads/routing/agents/sop/custody 五个六边形域全部接入运行时（适配器模式包装扁平包，替代 `internal/router` 直引；`internal/router` 已删除）。详见 `docs/plans/2026-multi-agent-orchestration.md` | `internal/domains/{threads,routing,agents,sop,custody}/` |
| Brief & Trail API | 从球权账本投影简报/轨迹查询端点（GET /api/custody/threads/{id}/trail），支撑多 agent 协作可观测/可审计 | `internal/domains/custody/` 投影 + `cmd/server/routes.go` |
| Code-Repo Trajectory | 可配置代码库地址（默认空）+ git-ref 分支轨迹采集（append-only 事件 + 纯函数投影）。默认空=零影响；非空时由 reconciler 周期采集（5min）落地 `repo-trajectory.json`，作为"项目归档源（狗狗开发讨论功能回溯）"的代码维度数据源，与球权事件流 + 聊天消息并列为三源 | `internal/domains/custody/services/git_ref_collector.go` `internal/domains/custody/stores/repo_trajectory_store.go` `internal/transport/repo_trajectory_handler.go` `internal/settings/config_handler.go` |

> **运行时成员数据统一（2026-08-12）**：成员/狗狗身份数据统一持久化于 `.sounds-great-ai/dog-catalog.json`，结构为 clowder 同构的 `{version, breeds[], roster{}, review_policy, leader, configs[]}`（类型定义见 `pkg/pack`）。`packs/default/breeds/dog-template.json` 降级为只读种子（`role_templates` / `client_defaults` 仍供模板 UI；`breeds/roster/leader/review_policy` 仅首启复制进 catalog）。breed 相关类型已从 `internal/config` 剥离至 `pkg/pack`；`internal/config` 现仅承载事件总线（`event_bus.go`）。本变更未引入新的 `internal/` 顶层目录，未推翻不可逆决策（VISION §4/§7）。详见 `sg-member-catalog-plan.md`。

> **编排能力升级（2026-08-13，不可逆决策）**：球权账本（append-only 事件流 + 纯函数投影状态机）成为 SG 多 agent 编排的一等状态源，落地于 `internal/domains/custody/`。同时运行时从扁平包（`internal/router`/`internal/threadstore`/`internal/adapter`/`internal/sop`/`internal/a2a`）迁往 `internal/domains/` 六边形层（适配器模式，Strangler Fig 逐域迁移）。本变更属于 Phase 7 编排完善扩展项，未引入新的 `internal/` 顶层目录，未推翻 §4 不可逆决策（仍走 CLI adapter、动态路由、平台层不内置推理）。运行时代码迁移（threads/routing/agents/sop/custody 五域 + P0 球权账本 + P1 心跳对账 + P2 托管持球 + P3 worklist/处置闭环 + P4 简报/轨迹 API + P5 前端轨迹面板）已于 2026-08-13 全部完成，双架构合并收口。详见 `docs/plans/2026-multi-agent-orchestration.md`。

> **编排二期缺口 G8–G10 收口（2026-08-13，完成）**：在 G1–G7 之上补齐剩余三缺口，全部落地（代码 + 单测，`go build ./...` / `go test ./...` 全绿；前端 `tsc -b` / `vite build` 全绿）。G8 新增「系统配置」分区与「项目归档源」卡片（可配置代码库地址 `repo_url`，默认空；非空时 reconciler 周期跑 `git ls-remote` 采集 `branch_pushed`/`branch_updated` 落地 `repo-trajectory.json`）；G9 修复 D6 死接口，新建 `web/src/hooks/useChatHistory.ts` 对齐 clowder 模块级常量，从 `GET /api/threads/{id}/messages` 游标分页水合历史 + scroll-up 增量；G10 前端新增「运维监控」分区（4 子 tab：总览/Traces/健康/评估，纯消费已有 `/api/ops/*` 与 `/api/evals`）+ 后端把 `EvalHandler` 依赖收口为端口（`transport.EvalStore` 接口，规避 `eval` 包循环）。G8 引入 git-ref 新数据源已登记于 §6「Code-Repo Trajectory」；G9/G10 行为不变、无新端点，未引入不可逆决策。详见 `docs/plans/2026-multi-agent-orchestration-g8g9g10-analysis.md`（状态：已实施）。

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
| 编排能力完善（球权账本/托管持球/A2A worklist/简报轨迹/domains 迁移） | 已完成（2026-08-13，spec `docs/plans/2026-multi-agent-orchestration.md` 全阶段验收通过） |

### 7.2 多 Agent 编排二期缺口（G1–G7，2026-08-13 完成）

对 `docs/plans/2026-multi-agent-orchestration-gaps.md` 中一期交付后剩余的 7 个编排缺口，本期全部落地（代码 + 单测，`go build ./...` 与 `go test ./...` 全绿）。实现要点见 `docs/plans/2026-multi-agent-orchestration-gaps-implementation.md`。

| 缺口 | 内容 | 落地位置 | 状态 |
|------|------|----------|------|
| **G1** | 精确处置守卫：条件写（per-thread 读-判-写锁），拒绝时写 `ball.disposition_rejected` 审计事件、不扭状态 | `internal/domains/custody/services/ball_ledger.go` `TryDispatchDispositioned` / `TryHoldDispositioned` | 完成 |
| **G2** | 运行时动态 worklist + 乒乓/深度熔断：invID 预算（递归 handoff 共享、resume 新 mint）；warn@2, block@4；实质工作（输出≥200 runes 或工具调用）豁免 | `internal/domains/routing/services/worklist_registry.go` + `internal/transport/execution.go` / `ws_handler.go` | 完成 |
| **G3** | webhook 唤醒鉴权：复用 `transport.AuthMiddleware`（`AUTH_TOKEN`，Bearer / X-Auth-Token）包裹唤醒端点 | `cmd/server/routes.go` `POST /api/custody/holds/` 经 `auth.WrapFunc` | 完成 |
| **G4** | 事件集补齐：9 事件（handed_cvo / void_pass / hold_expired / wake_sent / frozen / degraded / abandoned / unblocked / idle_long），投影效应钉死，add-only | `internal/domains/custody/ports/ledger.go` + `internal/domains/custody/services/projector.go` | 完成 |
| **G5** | 持球唤醒补全：定时 `FireAfterMs` + 命令 `Command`（ProcessManager 跑完自动唤醒）+ `Cancel` + `Tick` 自动唤醒/过期 | `internal/domains/custody/services/hold_scheduler.go` | 完成 |
| **G6** | 跨线程 duty-briefing 聚合：needsUser / deadBalls / voidPasses / staleBlocked（staleBlocked 按 UpdatedAt 降序） | `internal/domains/custody/services/ball_ledger.go` `ProjectDutyBriefing` + `cmd/server/routes.go` `GET /api/custody/briefing` | 完成 |
| **G7** | 跨 handoff 上下文消毒：来源溯源 `buildHandoffSourceNotice` + `telemetry.RedactSecrets`（HMAC 伪匿名 + 密钥掩码）+ `prompt.ProtectRecentPairs` 保护 Q→A 语义链 | `internal/transport/handoff_context.go` + `internal/telemetry/redactor.go` + `internal/prompt/context.go` | 完成 |

> 注：G8（另立项）、G9/G10 **不在本期范围**——已于 2026-08-13 在 `2026-multi-agent-orchestration-g8g9g10-analysis.md` 中另立项并实现（见 §7.3）。本期 G1–G7 未触及前端（G6/G7 仅后端端点与消毒逻辑），前端 tsc/vite 无需改动。

### 7.3 多 Agent 编排二期缺口（G8–G10，2026-08-13 完成）

对 G1–G7 之后的剩余三缺口，本期全部落地（代码 + 单测，`go build ./...` 与 `go test ./...` 全绿；前端 `tsc -b` 与 `vite build` 全绿）。实现要点见 `docs/plans/2026-multi-agent-orchestration-g8g9g10-analysis.md`（状态：已实施）。

| 缺口 | 内容 | 落地位置 | 状态 |
|------|------|----------|------|
| **G8** | 「系统配置」分区 + 项目归档源卡片（可配置代码库地址 `repo_url`，默认空）；非空时 reconciler 周期跑 `git ls-remote` 采集 `branch_pushed`/`branch_updated`，append-only 落盘 `repo-trajectory.json`（原子写） | `internal/settings/config_handler.go`（GET/PUT `/api/config/repo`）、`internal/domains/custody/services/git_ref_collector.go`、`internal/domains/custody/stores/repo_trajectory_store.go`、`internal/transport/repo_trajectory_handler.go`（GET `/api/repo/trajectory` + POST `/api/repo/test`）、`internal/platform/platform.go`（5min 周期采集）、`cmd/server/routes.go`、`web/src/components/settings/SystemPanel.tsx` | 完成 |
| **G9** | 修复 D6 死接口 `loadThreadEvents` 零调用：新建 `useChatHistory.ts`（对齐 clowder 模块级常量 `HISTORY_PAGE_SIZE=50` 等），从 `GET /api/threads/{id}/messages?limit=&before=` 游标分页水合历史 + scroll-up 增量；WS 实时事件与历史按 `type:timestamp:content` 去重；`BreedResponseCompleteEvent` 加可选 `content` 使历史 assistant 文本可见 | `web/src/hooks/useChatHistory.ts`、`web/src/store/useChatStore.ts`（`prependHistory` 去重合并）、`web/src/components/workspace/StreamTimeline.tsx`（`onScroll` 触发 `loadOlder`）、`web/src/components/workspace/BreedResponseComplete.tsx`、`web/src/types/index.ts` | 完成 |
| **G10** | 前端「运维监控」分区（4 子 tab：总览/Traces/健康/评估，纯消费已有 `/api/ops/*` 与 `/api/evals`）；后端把 `EvalHandler` 依赖收口为端口（`transport.EvalStore` 接口 = `ListVerdicts`/`GetVerdict`，`*eval.ResultStore` 结构满足，规避 `eval` 包循环），可注入 mock 单测 | `web/src/components/settings/OpsPanel.tsx`、`web/src/services/opsService.ts`、`web/src/components/settings/settings-nav-config.ts`、`SettingsContent.tsx`、`internal/transport/eval_handler.go`（EvalStore 端口 + 重写 `eval_handler_test.go`）、`internal/transport/eval_handler_test.go` | 完成 |

> 注：G8 引入 git-ref 新数据源，已登记于 §6「Code-Repo Trajectory」；G9/G10 行为不变、无新端点，未引入不可逆决策。G8 默认 `repo_url` 为空 = 零影响（采集器不运行、端点返回空事件）。

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
