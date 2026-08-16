# [SG-ORC-001] [Tech Story] 梳理并固化 Multi-Agent Orchestration 前后端逻辑

> 本文档基于 `sounds-great-ai` 真实源码核查（**2026-08-14 重新生成，反映截至本日的代码真实状态**）与既有编排 spec（`docs/plans/2026-multi-agent-orchestration.md`）编写，目标是把"多智能体编排"这一核心子系统的**前后端协作逻辑**固化为单一可信来源（single source of truth），供后续开发、review 与新人 onboarding 使用。
>
> **与初版差异**：初版在"全部修复"轮次前写成，含若干过时表述（kimi 无 adapter、webhook 无全局 auth、人工唤醒 UI 待补、配置默认犬与执行脱钩）。本版已全部校正，并补入 `client_id` 归一化（迁移修复）与默认犬配置接进路由的落地细节。
>
> **2026-08-16 校正**：WS 事件契约（§4.2）补全为运行时**单一真相**（补 `AGENT_MESSAGE`/`AGENT_LIVENESS`/`CARRIER_HEALTH`/`ERROR`，并修正对死组件 `TerminalOutputBlock` 的引用 → `CliOutputBlock`）；补 `dog-catalog.json` 的 `seen_template_breeds` 字段（§4.5）。CLI adapter 专属内容已收敛到 `SG-CLI-001`，本文仅保留运行时编排视角。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [x] Biz Story (业务)   [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu(路由) / @xigou(代码) | QA: @demu(诊断)
- **故事点/复杂度**: [ L (8分) ] —— 核心主链路，跨前后端 + 多域
- **业务/技术目标**:
  - As a **用户(Operator)/犬队成员**,
  - I want to **在一个对话(thread)里用 `@句柄` 把任务路由给不同的"狗"(外部 CLI agent)，并实时看到每只狗的推理/工具/代码过程、以及"谁持球"的协作轨迹**,
  - So that **多 agent 协作像犬队一样可观测、可审计、可接管，而非黑盒串行调用**.
- **关键指标/埋点**: 无前端埋点；可观测性来自 custody 轨迹 API（`/api/custody/threads/{id}/trail`）与 Ops 面板（`/api/ops/traces|metrics|evals`）。

### 1.1 一句话定位

后端把"犬队编排"建模为 **WebSocket 事件流 + 球权账本(append-only event ledger) + CLI adapter 进程池**；前端是**发起者 + 观察者**：发原始文本、渲染流式事件、旁观球权轨迹。所有路由/分解/接力/持球决策都在**后端**完成，前端不运行任何 agent。

### 1.2 端到端编排逻辑总览（前后端主线）

```
┌──────────────────────────────────────────────────────────────────────────┐
│ 前端 web/src                                                                 │
│  CommandBar 输入(可含 @bianmu/@xigou)                                       │
│    └─ useChatStore.sendPrompt() ─ WsManager.send(USER_INPUT) ─┐             │
│                                                              │ WS /ws      │
└──────────────────────────────────────────────────────────────│─────────────┘
                                                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ 后端 internal/transport/ws_handler.go:HandleWS                              │
│   payload.Message ──▶ MentionRouter.Route()  ← 路由决策(后端)               │
│        ├─ 无 @        → 配置默认犬 (single; 见 §4.5.1)                      │
│        ├─ 单 @        → 该犬 (single)                                      │
│        └─ 多 @        → parallel(默认) | serial(若含 串联/串行/serial/→/>>) │
│   BARK_START 事件 ──▶ Streamer 推前端                                       │
│        └─ executeWithPlatform / executeSerial / executeParallel            │
│             (maxConcurrentBark=8 信号量限流)                                │
│                ├─ PromptBuilder.Build + injectHooks                        │
│                ├─ MessageStore.GetByThread(sessionID,21)  ← 同 thread 共享历史│
│                ├─ AgentExecutor.Execute(ClientID=variant.client_id)        │
│                │     └─ unified.ProcessManager.Spawn(claude/codex/...)     │
│                │           stdin pipe ← system+history+query               │
│                │           stdout pipe → NDJSON → StreamEvent 通道          │
│                ├─ 流式 THINKING/TOOL_CALL/CODE_DIFF/TERMINAL_OUTPUT → 前端  │
│                ├─ 回复含 @某犬 → detectMentionInResponse → handleA2AHandoff │
│                │     → 递归 executeWithPlatform(下一犬)  ← "传球"           │
│                └─ 回复含 ```hold_ball 栅栏 → HoldScheduler 挂起线程(parked) │
│   BARK_RESULT / BARK_ERROR ──▶ Streamer 推前端                             │
│   custody 账本写 ball.handed/held/dispatch_dispositioned/task.done ...      │
└──────────────────────────────────────────────────────────────────────────┘
                                                                 │
                              GET /api/custody/threads/{id}/trail │ (轮询式, WS 事件触发 refresh)
                                                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ 前端 web/src/components/workspace/CustodyTrail.tsx                          │
│   useCustodyTrail() → services/custody.ts → 渲染 回合/传球/持球 + 事件轨迹   │
│   + 实时持球者 = isGenerating && 最近 breed_response_start 推导            │
│   + (parked/blocked 状态) "唤醒"按钮 → wsManager.sendWakeHold(...)         │
└──────────────────────────────────────────────────────────────────────────┘
```

### 1.3 前后端职责边界（关键事实）

| 维度 | 前端(React+TS+Zustand) | 后端(Go+Eino) |
|------|------------------------|----------------|
| 路由/分解/接力 | ❌ 不解析 `@`、不路由 | ✅ `MentionRouter.Route()` + `handleA2AHandoff` |
| Agent 执行 | ❌ 不 spawn CLI | ✅ `unified.AgentExecutor` + ProcessManager |
| 流式传输 | ✅ `WsManager`(自动重连5次) 消费事件 | ✅ `Streamer` 边执行边推 |
| 球权状态 | ✅ 旁观(`CustodyTrail`)+ WS 推导 live holder | ✅ `custody` 账本(append-only) |
| 成员/账户/leader 配置 | ✅ 编辑 UI → REST | ✅ `settings_handler`/`config_handler` 落盘 |
| 持球挂起/唤醒 | ✅ 展示 + 人工唤醒按钮(`sendWakeHold`) | ✅ `HoldScheduler` + `WAKE_HOLD`/`webhook` |
| 默认执行犬配置 | ✅ 成员管理面板「全局默认犬」选择器 | ✅ `default_breed` 配置读进 `Route()` |

---

## 2. 验收标准 (Acceptance Criteria - AC)

> 以下 AC 描述"编排逻辑按当前实现应当表现的行为"，可作为回归用例与 review 基线。

- [x] **AC-01 (正常路径-单犬/默认)**: Given 用户在 CommandBar 输入不含 `@` 的消息并发送, When 后端 `MentionRouter.Route()` 解析, Then 默认路由到**配置默认犬**（种子 `bianmu`，可由成员管理面板改；`DEFAULT_BREED_ID` 可部署级覆盖）(single)，前端 `StreamTimeline` 出现该犬的 `BreedCard`(`BARK_START`→`BARK_RESULT`)，事件带同一 `session_id=threadId`。
- [x] **AC-02 (正常路径-显式路由)**: Given 输入含 `@xigou 修这个 bug`, When 后端路由, Then 仅 `xigou` 被调用(single)，前端只渲染 xigou 卡片。
- [x] **AC-03 (正常路径-并行)**: Given 输入含 `@bianmu @xigou 分析`, When 后端识别多 mention 且无 serial 标记, Then 两犬 `parallel`(fan-out, 受 `maxConcurrentBark=8` 限流)，前端同一时间线先后/并行出现两张犬卡。
- [x] **AC-04 (正常路径-串行)**: Given 输入含 `@sa 先做 → @sb 再校验`(含 `→`/`串行`/`serial`/`>>` 标记), When `isSerialIntent` 命中, Then 路由策略=`serial`，A 输出喂给 B，`executeSerial` 在每节写 `dispatch_dispositioned`，调用恰好 2 次(无双跑)。
- [x] **AC-05 (正常路径-A2A 传球)**: Given 某犬回复文本含 `@下一犬`, When 后端 `detectMentionInResponse` 扫描命中, Then `handleA2AHandoff` 经 `tryDispatch` 守卫递归调用下一犬，账本写 `ball.handed`(父→子) + `dispatch_dispositioned`。
- [x] **AC-06 (正常路径-持球挂起)**: Given 某犬回复末尾含 ```` ```hold_ball\n{"kind":"manual"|"webhook","token":"..."}\n``` ````, When 后端 `extractHoldCondition` 解析, Then 流式输出剥离该栅栏(前端不可见)，线程写 `ball.held` 并 parked，跳过 `task.done` 与 @递归；唤醒(`WAKE_HOLD` 由前端按钮发送 或 `POST /api/custody/holds/{id}/webhook?token=`)后 `resumeHeld` 重跑 holder。
- [x] **AC-07 (正常路径-共享上下文)**: Given 同一 thread 多犬接力, When 后端 `MessageStore.GetByThread(sessionID, 21)` 取最近 21 条历史, Then 下一犬能看到前犬对话(共享上下文)，`ProtectRecentPairs(4)` 保护最近 4 轮 Q→A 不被截断。
- [x] **AC-08 (正常路径-球权轨迹)**: Given 任意 thread 已发生编排, When 前端 `CustodyTrail` 调 `GET /api/custody/threads/{id}/trail`, Then 返回 `Briefing{turns/handoffs/holds/trail[]}`，中文标签映射 `ball.handed`→接球、`dispatch_dispositioned`→传球、`ball.held`→持球挂起等。
- [x] **AC-09 (异常与边界-未知/legacy client_id)**: Given breed 的 `variant.client_id` 为白名单外 legacy 值(如 `anthropic/openai/google`), When `file_store.reloadFromDisk()` 加载, Then `normalizeClientID()` 在加载期将其映射为 `claude/codex/gemini` 并打 WARN 日志（self-healing，含恢复旧备份）；若某 client_id 彻底无对应 adapter，`GetAdapter` 返回 `BARK_ERROR`，不产生脏数据、不崩溃整个 thread。
- [x] **AC-10 (异常与边界-进程崩溃)**: Given CLI 子进程被 kill, When ACP Pool 僵尸回收触发, Then 账本写 `invocation.died`→投影 `dead/zombie`，主链路不阻塞。
- [x] **AC-11 (异常与边界-账户引用)**: Given 删除被成员绑定的账户, When `DELETE /api/settings/accounts/{id}` 返回 409 + `bound_member_ids`, Then 前端弹确认强删(`?force=true`)，否则拒绝。
- [x] **AC-12 (权限与安全)**: Given 未登录用户访问 `GET /api/custody/briefing` / `POST .../webhook` 等受保护端点, When 请求无有效 auth, Then 返回 401/403(`auth.WrapFunc` 包裹)；credentials.json 权限 0600，密钥不通过 REST 明文返回。

---

## 3. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- [x] **资损与网络安全 (Security)**
  - 敏感数据脱敏: ✅ 密钥存 `.sounds-great-ai/credentials.json`(0600)，REST 仅返回 `key_preview`/`key_set` 占位，不返回明文。
  - 防重提交/幂等: ✅ custody 账本为 append-only 事件流 + 纯函数投影(`Project`/`ProjectTrail`)，天然幂等重放；`hold_ball` 双唤醒/种类不匹配均返回错误(`ErrNoActiveHold`/种类错)。
  - webhook 唤醒鉴权: ✅ `POST /api/custody/holds/{threadID}/webhook` 已由 `auth.WrapFunc(CustodyWakeHandler)` 包裹（operator `AUTH_TOKEN` 全局鉴权）+ 每 hold token 常量时间比较。
- [x] **高并发与限流降级 (High Availability)**
  - Peak QPS: 编排主入口为 WebSocket(单连接事件流)，非高 QPS REST；REST 端点为配置/查询类，默认普通。
  - 降级/兜底: ✅ CLI 进程优雅 kill 链(`SIGINT` 2000ms → `SIGTERM` 3000ms → `SIGKILL`)；进程池 `SpawnWithPool` 失败优雅降级为一次性 spawn；MCP 按 breed 下发，gemini 不支持 MCP 时跳过 `--mcp-config`。
  - 动态开关: ❌ 本项目未用 Feature Flag；能力开关以内置 build tag / 配置为主。
- [x] **可服务性与监控 (Serviceability)**
  - 核心日志与错误码: ✅ custody 账本事件含 `thread_id/breed/handoff` 全链路；Ops 面板 `GET /api/ops/traces|metrics|evals` 提供跨 agent trace/指标/评测。
  - 监控告警: ⚠️ 依赖 Ops 端点人工观察；未配置自动 RT/错误率告警阈值(可按需补)。

---

## 4. 技术契约与接口设计 (Technical Contract)

### 4.1 传输与主入口

| 项 | 值 | 位置 |
|----|----|------|
| 编排入口 | `GET /ws` (WebSocket，流式) | `cmd/server/routes.go:64` → `ws_handler.go:HandleWS` |
| 健康检查 | `GET /health`、`GET /ready`(返回 adapters/breeds 数) | `routes.go:46/50` |
| 前端 WS 封装 | `WsManager`(自动重连最多 5 次，支持单条/批量事件) | `web/src/services/ws.ts` |
| 前端事件中枢 | `useChatStore.handleWsEvent`(WS `WsEvent`→前端 `StreamEvent` 增量拼接 + `seq` 断连检测) | `web/src/store/useChatStore.ts` |

### 4.2 WebSocket 事件契约（后端→前端，运行时单一真相 / single source of truth）

> 本表是 **WS 事件契约的权威来源**。所有经 `GET /ws` 下发的事件均在此列（类型常量见 `pkg/protocol/event.go`；`ERROR` 经 `protocol.NewEvent("ERROR", ...)` 动态构造于 `internal/transport/ws_handler.go`）。事件按**产出层**分类——`编排` 由 `ws_handler.go`/`execution.go`/`mention_router` 在路由/执行/A2A/hold 流程发出；`适配` 由 `internal/adapter/unified/*` + 各 provider adapter 在 CLI 子进程执行与 NDJSON 解析中发出（G1/R8/T25 落地项）。CLI 层只负责构造**标「适配」**的子集 payload，字段细节见 `SG-CLI-001` §5.1。

| 事件类型 | 产出层 | 含义 | 前端映射 |
|----------|--------|------|----------|
| `USER_INPUT` | 编排 | 用户输入回显 | (发送侧) |
| `BARK_START` | 编排 | 某犬开始生成 | `breed_response_start` + `isGenerating=true` |
| `THINKING` | 适配 | 推理流（adapter 透传 CLI thinking） | `thinking`(续写) |
| `TOOL_CALL` | 适配 | 工具调用（result 内嵌） | `tool_call` |
| `CODE_DIFF` | 适配 | 代码改动 | `code_diff` |
| `TERMINAL_OUTPUT` | 适配 | 终端输出（stderr/stdout 增量） | `terminal_output` |
| `AGENT_MESSAGE` | 适配 | 助手文本增量（live 累积） | `breed_response_live` |
| `AGENT_LIVENESS` | 适配 | 活体探测状态（软/硬 stall、恢复） | `breed_stall_warning` |
| `HITL_APPROVAL` | 编排 | 人工审批请求 | `approval_request` |
| `BARK_RESULT` | 编排 | 某犬完成（终态带 Content） | `breed_response_complete` + `isGenerating=false` |
| `BARK_ERROR` | 适配 | 某犬出错（结构化诊断） | `error` |
| `ERROR` | 编排 | 全局错误（**仅 toast，不进 timeline**） | (仅 toast) |
| `CARRIER_HEALTH` | 适配 | carrier 健康度（quota/structural/transient） | `carrierHealth` map → `ConnectionStatusBar` |
| `SYSTEM_NOTICE` | 编排 | 系统提示 | `system_notice` |
| `HITL_RESPONSE` | 编排 | 人工响应(前端→后端) | (发送侧) |
| `WAKE_HOLD` | 编排 | 唤醒持球线程(前端→后端) | (发送侧) |

> 前端 `StreamTimeline.tsx` 用 `switch(event.type)` 把上述事件渲染为 `BreedCard / ThinkingBlock / ToolLogBlock / CodeDiffBlock / CliOutputBlock / ApprovalBlock / ErrorBlock` 等。`CliOutputBlock`（合并相邻 `tool_call`+`terminal_output` 为单卡片）已取代 `TerminalOutputBlock`——后者在 `web/src` 中**无任何引用（死代码）**，勿再引用。
>
> **`BARK_REJECTED`**：仅前端 `useChatStore.ts:469` 有处理分支（清 `isGenerating` + toast），**后端当前无发出点**，属防御性分支，未在上表列出。

### 4.3 球权账本事件（custody 域，`internal/domains/custody/ports/ledger.go`）

append-only 事件流，纯函数投影为可观测状态：

`ball.handed` → `invocation.started` → `invocation.heartbeat` → `invocation.died`
`dispatch_dispositioned`(传球 父→子) → `ball.held` → `wake_condition_met` → `hold_dispositioned`(持球唤醒)
`task.done` → `resolved`；另有 `blocked`(SOP 限深熔断)。

投影状态：`active / resolved / blocked / parked / dead`。

### 4.4 REST 端点（编排相关）

| 端点 | 方法 | 用途 | 前端调用 |
|------|------|------|----------|
| `/api/breeds` | GET/POST/PATCH/DELETE | 狗狗(犬队)配置 CRUD | `breedService` |
| `/api/settings/roster/{id}` | PATCH | 启用/停用成员(`available`) | `settingsService.updateRosterEntry` |
| `/api/settings/accounts` | GET/POST/PATCH/DELETE | 账户与密钥(409=被绑定) | `AccountKeys` + `settingsService` |
| `/api/config/leader` | PATCH | 编辑 Leader/Owner(`@You/@leader/@owner`) | `HubLeaderEditor` |
| `/api/config/default-breed` | GET/PUT | 读/设全局默认犬 | `settingsService.getDefaultBreed/setDefaultBreed` |
| `/api/config/breed-order` | PUT | 拖拽排序 | `settingsService.setBreedOrder` |
| `/api/threads`、`/api/threads/{id}/messages` | GET | 会话列表 / 历史水合(增量加载) | `threadService` + `useChatHistory` |
| `/api/custody/threads/{id}/trail` | GET | 球权轨迹投影 | `services/custody.ts` → `CustodyTrail` |
| `/api/custody/briefing` | GET | 跨线程值班简报 | `CustodyDutyBriefingHandler` |
| `/api/custody/holds/{threadID}/webhook?token=` | POST | 外部 webhook 唤醒 hold（带 `auth.WrapFunc` 全局鉴权） | `CustodyWakeHandler` |
| `/api/rag/`、`/api/memory/`、`/api/mcp/servers` | GET | 共享能力(检索/记忆/MCP) | 对应 service |
| `/api/ops/traces|metrics|evals` | GET | 跨 agent 可观测 | `opsService` |

### 4.5 关键后端组件与数据流

| 层 | 文件 | 职责 |
|----|------|------|
| 传输 | `internal/transport/ws_handler.go` | WS 事件循环、路由入口、`WAKE_HOLD`/`resumeHeld` |
| 执行 | `internal/transport/execution.go` | `executeWithPlatform/Serial/Parallel`、A2A handoff、hold 剥离、custody 写账本 |
| 路由 | `internal/domains/routing/services/mention_router.go` | `@mention` 解析、配置默认犬（§4.5.1）、serial 意图判定 |
| 适配 | `internal/adapter/unified/*` + `claude/codex/gemini/opencode/kimi` | spawn CLI、NDJSON 解析、MCP config 下发 |
| 球权 | `internal/domains/custody/*` | 账本 append-only + 投影 + `HoldScheduler` |
| 名册 | `internal/pack/breed.go` + `internal/settings/file_store.go` | breed/roster/leader 加载与持久化 |
| 平台 | `internal/platform/platform.go` | **仅装配**，组合 adapter/域/MCP/memory/ragstore/hooks/SOP（不推理） |
| 协调 | `internal/a2a/hub.go` | **进程内**内存 hub（非 HTTP），handoff 记账 + SOP 深度 + participants |

**CLI client_id 白名单**（`internal/settings/validation.go`）：`claude / codex / gemini / opencode / kimi`。5 个 client **均有对应 adapter**（kimi 为 `internal/adapter/kimi`，凭证走 `KIMI_API_KEY` 等环境变量），均可 spawn。

**持久化文件**（`.sounds-great-ai/`，`ConfigRoot` 解析顺序：`SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT` → `{projectRoot}/.sounds-great-ai` → `{home}/.sounds-great-ai`）：
- `dog-catalog.json`(0644)：breeds/roster/review_policy/leader/configs/deleted_breeds/seen_template_breeds — **运行时唯一真相**（字段与首启空/升级同步机制详见 `SG-MEM-001` §5.1 / §6.2）
- `accounts.json`(0644)：账户元数据
- `credentials.json`(0600)：密钥
- 种子：`packs/default/breeds/dog-template.json`；`MergedBreeds()` 用 catalog 覆盖模板。

### 4.5.1 默认犬配置（全局默认执行犬）

无 `@` 提及时的"默认执行犬"**由配置驱动，不再硬编码 bianmu**（落地于 2026-08-14）：

- **存储**：`configs["default_breed"]`（落 `.sounds-great-ai/dog-catalog.json`）；种子默认 `bianmu`（`internal/settings/memory.go`）。
- **环境变量覆盖**：`DEFAULT_BREED_ID` 在读取时优先于配置值（operator 级部署覆盖）。
- **API**：`GET /api/config/default-breed`（`{breed_id, is_override}`）、`PUT /api/config/default-breed`（`{breed_id}`，未知犬 → 404）。
- **前端**：成员管理面板「全局默认犬」下拉（`web/src/components/settings/MemberManagement.tsx:426-484`）→ `settingsService.setDefaultBreed()`，落盘并 toast。
- **执行链路接通**：`MentionRouterService` 持有一个 `defaultBreedProvider`（由 `platform.go:222` 装配，运行时读取 env + `ListConfig`）；`Route()` 在 `case 0`（无 @）时调用 `resolveDefaultBreed()` —— 返回配置值并经 `breedIDs` 校验，非法/未知则回退 `"bianmu"`。因此**页面设置的默认犬真正决定对话默认执行犬**，无需 `@`。

### 4.5.2 client_id 归一化（迁移修复，落地于 2026-08-14）

历史种子模板与部分 catalog 曾使用厂商代号 `anthropic`/`openai`/`google` 作为 `client_id`，而这些值**不在 adapter 白名单**，会导致对应狗狗被路由时 `Get(adapter)` 找不到 → spawn 失败。修复采用 **加载期 self-healing**，不依赖手动清理：

- `internal/settings/file_store.go` 定义 `legacyClientIDMap{anthropic→claude, openai→codex, google→gemini}` + `normalizeClientID()`；在 `reloadFromDisk()` 加载 breeds/accounts 时对每个 `variant.client_id` 归一化，并打 WARN 日志。
- **生效范围**：首次启动播种、热加载（`file_store` 文件轮询）、以及**恢复旧 `.bak` 备份**都会自动修正，杜绝"恢复备份即炸"。
- **活体文件已就地修正**：`.sounds-great-ai/dog-catalog.json` 与种子 `packs/default/breeds/dog-template.json` 的 legacy 值已全部改为 `claude/codex/gemini`（各 12 处），当前残留计数 = 0。
- **回归测试**：`TestFileStore_LegacyClientIDNormalizedOnLoad` 锁定该行为。

### 4.6 关键前端组件

| 组件/文件 | 职责 |
|-----------|------|
| `web/src/store/useChatStore.ts` | 编排核心状态：`events[threadId]`、`isGenerating`、`lastSeq`、WS 事件分发 |
| `web/src/store/useAppStore.ts` | 导航/输入状态(Zustand persist)：`activeNav`、`mentionOpen`、`activeThreadId` |
| `web/src/services/ws.ts` | `WsManager`(WS 连接/重连/批量) + `sendWakeHold()` 人工唤醒发送 |
| `web/src/components/workspace/StreamTimeline.tsx` | 同一时间线渲染多犬事件流；`useChatHistory` 接线历史水合 |
| `web/src/components/workspace/BreedCard.tsx` + `BreedResponseStart/Complete.tsx` | 每只犬一次"叫唤"卡片 |
| `web/src/components/workspace/CustodyTrail.tsx` + `hooks/useCustodyTrail.ts` + `services/custody.ts` | 球权轨迹面板(状态/持球者/回合·传球·持球统计/事件轨迹) + 人工唤醒按钮 |
| `web/src/components/workspace/CommandBar.tsx` + `MentionPopover.tsx` | `@` 触发 mention 弹层，插入 `@{breedId}` |
| `web/src/components/settings/MemberManagement.tsx` + `HubBreedEditor/HubLeaderEditor.tsx` | 犬队成员/默认犬/Leader 编辑 |
| `web/src/components/settings/AccountKeys.tsx` | 账户与密钥(409 强删确认) |
| `web/src/components/drawer/tabs/PlanTab.tsx` | 任务计划面板；无 `taskPlanSteps` 时显示「暂无任务计划」空状态（诚实呈现，不误导） |
| `web/src/lib/breed-colors.ts` | 6 犬配色(边牧蓝/西高地粉/金毛橙/德牧深灰/藏獒紫/田园犬绿) |
| `web/src/types/api.ts` + `types/index.ts` | `WsEvent`/`BreedConfig`/`Variant`/`RosterEntry`/`StreamEvent`/`DogAgent` 等契约类型 |

---

## 5. Story 级 Definition of Done (DoD Checklist)

- [x] 编排主链路文档化（前端发起 → WS → 后端路由 → CLI spawn → 流式 → A2A 传球 → 球权账本 → 轨迹呈现）已固化于本文。
- [x] 前后端职责边界明确（路由/执行全在后端，前端为发起者+观察者；默认犬配置已接进执行链路）。
- [x] 技术契约完整：WS 事件表、custody 账本事件、REST 端点表、关键组件清单、持久化文件与白名单。
- [x] 已知缺口已全部处置（见 §6 处置总览）：kimi adapter 已补、webhook 已鉴权、人工唤醒 UI 已接、默认犬配置已接进路由、client_id 已归一化、聊天历史水合已接线。
- [ ] 单元测试覆盖率达到团队基线（编排逻辑已有 `execution_serial_test`/`execution_hold_test`/`mention_router_test`/`hold_scheduler_test`/`briefing_test`/`file_store_test` 等，核心链路 > 70%）。
- [ ] 静态代码扫描无 P0/P1 级安全漏洞（`go build ./...` + `go test ./...` + `tsc -b` 全绿为现状基线）。
- [ ] 监控告警与降级开关在预发/灰度环境验证正常（当前依赖 Ops 端点人工观察，自动告警待补）。

---

## 6. 已知缺口处置总览（截至 2026-08-14 全部处置）

原初版列出的 6 项缺口（G-1~G-6）+ 后续发现的「默认犬配置与执行脱钩」+「legacy client_id 迁移风险」，本日均已处置：

| # | 缺口 | 处置 | 证据 |
|---|------|------|------|
| G-1 | kimi 在白名单但无 adapter | ✅ **已修复（代码）** | 新增 `internal/adapter/kimi/adapter.go`（kimi NDJSON 事件解析器）+ 注册进 `platform.go:132`；`go build`/`go test`/`go vet` 全绿 |
| G-2 | webhook 唤醒鉴权 | ✅ **已解决** | `routes.go:174` `auth.WrapFunc(CustodyWakeHandler)` 包裹（operator `AUTH_TOKEN` + 每 hold token 常量比较） |
| G-3 | 前端人工唤醒 UI 未接线 | ✅ **已修复（代码）** | `ws.ts` 加 `sendWakeHold`；`CustodyTrail.tsx:73/120-127` 在 parked/blocked 时显示"唤醒"按钮 |
| G-4 | TaskPlan 占位误导 | ✅ **已缓解（代码）** | `PlanTab.tsx:15/35` 无数据时显示「暂无任务计划」+ 虚线提示，消除"永远 Executing" |
| G-5 | 进程池 warm 复用 | ⚪ **澄清为非缺陷（独立排期）** | `SetPool`/`SpawnWithPool` 生产零调用，`SpawnWithPool` 仍走一次性 `Spawn` → 已知设计限制（性能议题，含压测） |
| G-6 | 聊天历史水合 | ✅ **已解决** | 后端 `GET /api/threads/{id}/messages` + 前端 `useChatHistory.ts` 已由 `StreamTimeline.tsx` 接线 |
| G-7 | 默认犬配置与执行脱钩 | ✅ **已修复（代码）** | `mention_router.go:resolveDefaultBreed()` + `platform.go:222 SetDefaultBreedProvider`；无 `@` 走配置默认犬 |
| G-8 | legacy client_id 迁移风险 | ✅ **已修复（代码+数据）** | `file_store.go:normalizeClientID()` self-healing；活体 catalog + 种子模板 legacy 值已归零；`file_store_test.go` 回归 |

**无 P1 阻塞项**：编排主链路无阻断性缺口。仅 G-5 为性能/设计议题、G-4 后端 `taskPlanSteps` 产出为待排期特性。

---

> 关联文档：`docs/plans/2026-multi-agent-orchestration.md`（编排升级 spec，全阶段已完成）、`docs/plans/2026-multi-agent-orchestration-gaps*.md`（差距分析）、`docs/DESIGN-STORYS/SG-MEM-001-member-management.md`、`SG-ACC-001-accounts-keys-auth.md`。
