# [FT-A2A-001] [Tech Story] 梳理并固化 A2A Communication 前后端逻辑

> 本文档基于 `sounds-great-ai` 真实源码核查（**2026-08-17 生成，反映截至本日的代码真实状态**）编写，目标是把"A2A Communication（Agent-to-Agent 通信）"这一核心子系统（含 `@mention` 路由、链式 handoff、球权账本 custody、受控外部 A2A 协议客户端、hold_ball）的**前后端协作逻辑**固化为单一可信来源（single source of truth），供后续开发、review 与新人 onboarding 使用。
>
> 关联：**前序 STORY** `FT-ORC-001`（多智能体编排总览，本文是其 A2A 子集的细化与权威补充）；**红线依据** `不可逆决策` §4.7；**协作提示词** `packs/default/hooks/s8-a2a-format/template.md`（已对齐 §4.7）。
>
> **关键纠正（相对旧对比报告 v2）**：旧 HTML 报告 `docs/A2A-COMMUNICATION-COMPARISON-2026-08-17-v2.html:58` 曾控告 `s8-a2a-format/template.md` "仍写不建 HTTP server/client（§4.1）"——经查当前 `template.md:32/44-46` **已对齐 §4.7**（允许受控客户端、禁止 server），该指控为过时误判，以本文为准。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [ ] Biz Story (业务)   [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu(路由) / @xigou(代码) | QA: @demu(诊断)
- **故事点/复杂度**: [ L (8分) ] —— 核心主链路（编排 + 球权 + 外部协议客户端），跨前后端 + 多域
- **业务/技术目标**:
  - As a **用户(Operator)/狗狗队伍成员**,
  - I want to **在一个对话(thread)里用 `@句柄` 把任务交给不同的犬（本地 CLI agent），并能把任务交给外部已部署的 A2A agent，同时实时看到每只 agent 的推理/工具/代码过程、以及"谁持球"的协作轨迹**,
  - So that **多 agent 协作像狗狗队伍一样可观测、可审计、可接管（含跨进程/跨实例的外部 agent），而非黑盒串行调用**.
- **关键指标/埋点**: 无前端埋点；可观测性来自 custody 轨迹 API（`/api/custody/threads/{id}/trail`）与 Ops 面板（`/api/ops/traces|metrics|evals`）。

### 1.1 一句话定位

A2A Communication 在 SG 中由**两层**构成，两者都走后端编排、前端只做发起者与观察者：

1. **狗狗队伍内部协作（主路径）**：`@mention` 文本约定 + `internal/domains/custody` 球权账本（append-only 事件 + 纯函数 8 态投影） + CLI adapter 进程池（stdin/stdout pipe）。一切路由/分解/接力/持球决策在后端，前端不 spawn CLI。
2. **受控外部 A2A 协议客户端（§4.7 受控接入）**：平台作为 **A2A 协议客户端**，经 Google A2A Protocol `tasks/send` JSON-RPC over HTTPS 调**外部已部署 agent**（如另一套 SG 实例）。实现限 `internal/adapter/a2a/`（与 CLI adapter 并列，实现 `unified.AgentExecutor`），协议类型复用 `pkg/a2a/`；**红线禁止**新建内部 A2A server、禁止 `internal/a2a/server|client/` 子目录、禁止在 `internal/` 内置推理。

### 1.2 端到端 A2A 逻辑总览（前后端主线）

```
┌──────────────────────────────────────────────────────────────────────────┐
│ 前端 web/src                                                                 │
│  CommandBar 输入(可含 @bianmu/@xigou/@远程协作者)                            │
│    └─ MentionPopover 插入 @{breedId} → useChatStore.sendPrompt()           │
│       └─ WsManager.send(USER_INPUT) ─┐                                      │
│                                     │ WS /ws                                │
└─────────────────────────────────────│──────────────────────────────────────┘
                                      ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ 后端 internal/transport/ws_handler.go:HandleWS                              │
│   payload.Message ──▶ MentionRouter.Route()  ← 路由决策(后端)               │
│        ├─ 无 @        → 配置默认犬 (single)                                 │
│        ├─ 单 @        → 该犬 (single)                                       │
│        └─ 多 @        → parallel | serial(含 串联/串行/serial/→/>>)         │
│   BARK_START 事件 ──▶ Streamer 推前端                                       │
│        └─ executeWithPlatform / executeSerial / executeParallel            │
│             ├─ PromptBuilder.Build + injectHooks                           │
│             ├─ MessageStore.GetByThread(sessionID,21)  ← 共享历史           │
│             ├─ AgentExecutor.Execute(ClientID=variant.client_id)           │
│             │     ├─ CLI adapter (claude/codex/gemini/opencode/kimi)        │
│             │     │     └─ unified.ProcessManager.Spawn → stdin/stdout NDJSON│
│             │     └─ A2A adapter (client_id="a2a") ← 外部 agent 接入路径     │
│             │           └─ adapter.Execute → tasks/send JSON-RPC over HTTPS │
│             ├─ 流式 THINKING/TOOL_CALL/CODE_DIFF/TERMINAL_OUTPUT → 前端     │
│             ├─ 回复含 @某犬 → detectMentionInResponse → handleA2AHandoff    │
│             │     → G1 tryDispatch 守卫 → 递归 executeWithPlatform(下一犬)  │
│             └─ 回复含 ```hold_ball 栅栏 → HoldScheduler 挂起线程(parked)    │
│   custody 账本写 ball.handed/dispatch_dispositioned/ball.held/task.done …  │
└──────────────────────────────────────────────────────────────────────────┘
                                                                 │
                       GET /api/custody/threads/{id}/trail   │ (WS 事件触发 refresh)
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
| Agent 执行 | ❌ 不 spawn CLI / 不调外部 A2A | ✅ `unified.AgentExecutor`（CLI + A2A 并列 carrier） |
| 外部 A2A 协议客户端 | ❌ | ✅ `internal/adapter/a2a`（§4.7 受控客户端，tasks/send） |
| 流式传输 | ✅ `WsManager`(自动重连5次) 消费事件 | ✅ `Streamer` 边执行边推 |
| 球权状态 | ✅ 旁观(`CustodyTrail`)+ WS 推导 live holder | ✅ `custody` 账本(append-only + 读驱动) |
| 持球挂起/唤醒 | ✅ 展示 + 人工唤醒按钮(`sendWakeHold`) | ✅ `HoldScheduler` + `WAKE_HOLD`/`webhook` |
| 默认执行犬配置 | ✅ 成员管理面板「全局默认犬」选择器 | ✅ `default_breed` 配置读进 `Route()` |

---

## 2. 验收标准 (Acceptance Criteria - AC)

> 以下 AC 描述"A2A Communication 按当前实现应当表现的行为"，可作为回归用例与 review 基线。

- [x] **AC-01 (正常路径-单犬/默认)**: Given 用户输入不含 `@` 并发送, When `MentionRouter.Route()` 解析, Then 默认路由到**配置默认犬**（single），前端 `StreamTimeline` 出现该犬 `BreedCard`（`BARK_START`→`BARK_RESULT`）。
- [x] **AC-02 (正常路径-显式路由)**: Given 输入含 `@xigou 修这个 bug`, When 后端路由, Then 仅 `xigou` 被调用(single)，前端只渲染 xigou 卡片。
- [x] **AC-03 (正常路径-并行)**: Given 输入含 `@bianmu @xigou 分析` 且无 serial 标记, When 路由, Then 两犬 `parallel`(fan-out, 受 `maxConcurrentBark=8` 限流)。
- [x] **AC-04 (正常路径-串行)**: Given 输入含 `@sa 先做 → @sb 再校验`, When `isSerialIntent` 命中, Then 策略=`serial`，A 输出喂给 B，`executeSerial` 每节经 `tryDispatch` 守卫（球权被顶替则终止）。
- [x] **AC-05 (正常路径-A2A 传球)**: Given 某犬回复文本含 `@下一犬`, When `detectMentionInResponse` 扫描命中, Then `handleA2AHandoff` 经 G1 `tryDispatch` 守卫（`BallLedger.TryDispatchDispositioned`）递归调用下一犬，账本写 `ball.handed`(父→子) + `dispatch_dispositioned`。
- [x] **AC-06 (正常路径-持球挂起)**: Given 回复末尾含 ```` ```hold_ball\n{"kind":"manual"|"webhook","token":"..."}\n``` ````, When `extractHoldCondition` 解析, Then 输出剥离栅栏(前端不可见)，线程写 `ball.held` 并 parked，跳过 `task.done` 与 @递归；唤醒（`WAKE_HOLD` 或 `POST /api/custody/holds/{id}/webhook?token=`）后 `resumeHeld` 重跑 holder。
- [x] **AC-07 (正常路径-共享上下文)**: Given 同一 thread 多犬接力, When `MessageStore.GetByThread(sessionID, 21)` 取最近 21 条历史, Then 下一犬能看到前犬对话，`ProtectRecentPairs(4)` 保护最近 4 轮不被截断。
- [x] **AC-08 (正常路径-球权轨迹)**: Given 任意 thread 已发生编排, When 前端 `CustodyTrail` 调 `GET /api/custody/threads/{id}/trail`, Then 返回 `Briefing{turns/handoffs/holds/trail[]}`，中文标签 `ball.handed`→接球、`dispatch_dispositioned`→传球、`ball.held`→持球挂起。
- [x] **AC-09 (正常路径-外部 A2A 客户端 §4.7)**: Given 某犬 variant `client_id="a2a"` 且 `a2a_url` 已配置, When 该犬被路由执行, Then `AgentExecutor` 按 ClientID 选 `a2aAdapter` → `adapter.Execute` 发 `tasks/send` JSON-RPC over HTTPS 到外部 agent，收外部 Task 结果流（`streamTask`）回灌前端；外部 agent 自带身份，SG 仅透传 prompt。
- [x] **AC-10 (异常与边界-未知/legacy client_id)**: Given `variant.client_id` 为白名单外 legacy 值, When `file_store.reloadFromDisk()` 加载, Then `normalizeClientID()` 映射为 `claude/codex/gemini` 并打 WARN；若彻底无对应 adapter，`Get` 返回 `BARK_ERROR`，不崩溃整个 thread。
- [x] **AC-11 (异常与边界-进程崩溃)**: Given CLI 子进程被 kill, When ACP Pool 僵尸回收触发, Then 账本写 `invocation.died`→投影 `dead/zombie`，主链路不阻塞。
- [x] **AC-12 (权限与安全)**: Given 未登录访问 `GET /api/custody/briefing` / `POST .../webhook`, When 无有效 auth, Then 返回 401/403（`auth.WrapFunc`）；credentials.json 权限 0600，密钥不通过 REST 明文返回。
- [x] **AC-13 (红线-禁 server)**: Given 任何代码改动, When 引入 `internal/a2a/server/` 或 `internal/a2a/client/` 子目录、或新建监听外部入站的 A2A HTTP server, Then 命中 AGENTS.md 红旗（§4.1+§4.7），必须阻止提交。
- [x] **AC-14 (正常路径-CVO 升级闭环, 2026-08-22)**: Given 交接链深度超限 `CheckA2ADepth() == EscalateToCVO`, When 熔断发生, Then 账本写 `BallHandedCVO` + 前端收到 `CVO_ESCALATION`（含 `escalation_id`/`reason`/预置选项）；操作员点选选项 → `CVO_ESCALATION_RESPONSE` 回传 → 后端按该选项的**服务端预置 prompt** 经 `routeAndDispatch` 重新派发（等价于一条新的用户消息），`intervene` 则只解除不重派；升级卡与 `escalations[threadId]` 标记同步清除（见 §4.8）。

---

## 3. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- [x] **资损与网络安全 (Security)**
  - 敏感数据脱敏: ✅ 密钥存 `.sounds-great-ai/credentials.json`(0600)，REST 仅返回 `key_preview`/`key_set`；A2A 适配器 `Bearer` token 走内存 `apiKeys`，不落 REST。
  - 防重提交/幂等: ✅ custody 账本 append-only + 纯函数投影，天然幂等重放；`hold_ball` 双唤醒/种类不匹配返回 `ErrNoActiveHold`/种类错。
  - webhook 唤醒鉴权: ✅ `POST /api/custody/holds/{threadID}/webhook` 已由 `auth.WrapFunc(CustodyWakeHandler)` 包裹 + 每 hold token 常量时间比较。
  - 外部调用隔离: ✅ A2A 客户端仅作 carrier，不在 `internal/` 内置推理；外部 agent 崩溃不影响本地编排（错误经 `streamTask`→`error` 事件回灌，主链路降级）。
- [x] **高并发与限流降级 (High Availability)**
  - Peak QPS: 编排主入口为 WebSocket（单连接事件流）；REST 为配置/查询类，默认普通。
  - 降级/兜底: ✅ 外部 A2A agent 超时/不可达 → `adapter.Health` 报端点未配置、`Execute` 返回 `error` 事件，本地 thread 不阻塞；CLI 进程优雅 kill 链 + 进程池降级。
  - 动态开关: ❌ 未用 Feature Flag；A2A 外部接入以 breed `enabled`/`a2a_url` 配置开关（默认 `a2a-remote` 犬 `enabled:false`）。
- [x] **可服务性与监控 (Serviceability)**
  - 核心日志与错误码: ✅ custody 账本事件含 `thread_id/breed/handoff` 全链路；`internal/a2a/hub.go` 经 OpenTelemetry `A2AHandoffCount` 埋点；Ops 面板 `GET /api/ops/traces|metrics|evals` 提供跨 agent trace。
  - 监控告警: ⚠️ 依赖 Ops 端点人工观察；未配置自动 RT/错误率告警阈值（待补）。

---

## 4. 技术契约与接口设计 (Technical Contract)

### 4.1 传输与主入口

| 项 | 值 | 位置 |
|----|----|------|
| 编排入口 | `GET /ws` (WebSocket，流式) | `cmd/server/routes.go` → `ws_handler.go:HandleWS` |
| 健康检查 | `GET /health`、`GET /ready` | `cmd/server/routes.go` |
| 前端 WS 封装 | `WsManager`(自动重连≤5次) | `web/src/services/ws.ts` |
| 前端事件中枢 | `useChatStore.handleWsEvent`(增量拼接 + `seq` 断连检测) | `web/src/store/useChatStore.ts` |

### 4.2 WebSocket 事件契约（后端→前端，运行时单一真相）

> 本表与 `FT-ORC-001` §4.2 一致（A2A 不引入新事件类型；外部 A2A agent 的结果经 `streamTask` 映射为 `text/error/done` 事件回灌同一通道）。事件类型常量见 `pkg/protocol/event.go`。

| 事件类型 | 产出层 | 含义 | 前端映射 |
|----------|--------|------|----------|
| `USER_INPUT` | 编排 | 用户输入回显 | (发送侧) |
| `BARK_START` | 编排 | 某犬开始生成 | `breed_response_start` + `isGenerating=true` |
| `THINKING` | 适配 | 推理流 | `thinking`(续写) |
| `TOOL_CALL` | 适配 | 工具调用 | `tool_call` |
| `CODE_DIFF` | 适配 | 代码改动 | `code_diff` |
| `TERMINAL_OUTPUT` | 适配 | 终端输出 | `terminal_output` |
| `AGENT_MESSAGE` | 适配 | 助手文本增量 | `breed_response_live` |
| `AGENT_LIVENESS` | 适配 | 活体探测 | `breed_stall_warning` |
| `BARK_RESULT` | 编排 | 某犬完成（终态带 Content） | `breed_response_complete` + `isGenerating=false` |
| `BARK_ERROR` | 适配 | 某犬出错（含 A2A 外部调用失败） | `error` |
| `CARRIER_HEALTH` | 适配 | carrier 健康度（含 A2A endpoint 健康） | `carrierHealth` map |
| `SOP_GATE` | 编排 | 跨犬审查门禁（拦截/建议路由） | `sop_gate` → `SopGate` |
| `CVO_ESCALATION` | 编排 | A2A 深度硬轨熔断，球权上交 CVO（§4.8） | `cvo_escalation` 决策卡 + `escalations[threadId]` |
| `WAKE_HOLD` | 编排 | 唤醒持球线程(前端→后端) | (发送侧) |
| `CVO_ESCALATION_RESPONSE` | 编排 | CVO 升级决策回传(前端→后端，§4.8) | (发送侧) |

### 4.3 球权账本事件（custody 域，`internal/domains/custody/ports/ledger.go`）

append-only 事件流，纯函数投影为可观测状态（**读驱动**：`handleA2AHandoff` 派发前读 `Snapshot` 校验持球者）：

`ball.handed` → `invocation.started` → `invocation.heartbeat` → `invocation.died`
`dispatch_dispositioned`(传球 父→子) → `ball.held` → `wake_condition_met` → `hold_dispositioned`(持球唤醒)
`task.done` → `resolved`；另有 `ball.disposition_rejected`(G1 守卫拒绝审计) 与 `blocked`(SOP 限深熔断)。

投影 8 态（纯函数 `Project`，`projector.go:12`）：`new / active / blocked / parked / dead / void / zombie / resolved`。

### 4.4 REST 端点（A2A 相关）

| 端点 | 方法 | 用途 | 前端调用 |
|------|------|------|----------|
| `/api/breeds` | GET/POST/PATCH/DELETE | 狗狗配置 CRUD（含 `a2a_url` 字段） | `breedService` |
| `/api/config/default-breed` | GET/PUT | 读/设全局默认犬 | `settingsService` |
| `/api/custody/threads/{id}/trail` | GET | 球权轨迹投影 | `services/custody.ts` → `CustodyTrail` |
| `/api/custody/briefing` | GET | 跨线程值班简报 | `CustodyDutyBriefingHandler` |
| `/api/custody/holds/{threadID}/webhook?token=` | POST | 外部 webhook 唤醒 hold（带全鉴权） | `CustodyWakeHandler` |

### 4.5 关键后端组件与数据流

| 层 | 文件 | 职责 |
|----|------|------|
| 传输 | `internal/transport/ws_handler.go` | WS 事件循环、路由入口、`WAKE_HOLD`/`resumeHeld`、`BroadcastCarrierHealth`、CVO 升级注册表 + 决策重派（`routeAndDispatch`:271 / `emitCvoEscalation`:382 / `handleEscalationResponse`:427，见 §4.8） |
| 执行/A2A | `internal/transport/execution.go` | `executeWithPlatform/Serial/Parallel`、`detectMentionInResponse`、`handleA2AHandoff`(730)、G1 `tryDispatch`(737)、§4.5 读 `Snapshot` 守卫(827-835)、custody 写账本、深度熔断点 `emitCvoEscalation`(891，§4.8) |
| @解析 | `internal/transport/mention.go` | `mentionRegex`(10)、`parseMention`(13)、`isLeaderMention`(27) |
| 路由 | `internal/domains/routing/services/mention_router.go` | `@mention` 解析、配置默认犬、serial 意图判定 |
| 熔断 | `internal/domains/routing/services/worklist_registry.go` | G2 深度 + ping-pong 熔断 `Push`(94) |
| 适配(CLI) | `internal/adapter/unified/*` + `claude/codex/gemini/opencode/kimi` | spawn CLI、NDJSON 解析、MCP config 下发 |
| 适配(A2A) | `internal/adapter/a2a/adapter.go` | **§4.7 受控客户端**：`Execute`(112) 发 `tasks/send`、`SetEndpoint`(61)、`Health`(83)、`lookupURL`(93)、`streamTask`(175)、`buildPrompt`(209) |
| 协议类型 | `pkg/a2a/types.go` + `methods.go` | Google A2A 风格 `AgentCard`/`Task`/`JSONRPCRequest`/`MethodTasksSend`；**已激活**（仅 `adapter.go:30` 引用，非死代码） |
| 球权 | `internal/domains/custody/*` | `BallLedger`(append-only + `TryDispatchDispositioned` 三态守卫)、`HoldScheduler`、`projector`(8 态纯函数)、`briefing`(只读投影) |
| 内存协作史 | `internal/a2a/hub.go` | **进程内**内存 hub（非网络），handoff 记账 + OpenTelemetry；经 `a2a_hub_adapter.go` 接入 |
| 分配 | `internal/domains/agents/services/agent_executor.go` | 按 `req.ClientID` 选 adapter(28)、转发 ClientID(40) |
| 平台 | `internal/platform/platform.go` | 装配 adapters（含 `"a2a"` 注册 186）、从 `variant.a2a_url` 调 `SetEndpoint`(289)、`Compressor` 字段已移除、`StartReconciler` 定时 ReconcileZombies/Tick |

**CLI client_id 白名单**（`internal/settings/validation.go`）：`claude / codex / gemini / opencode / kimi`（5 个均有 adapter）。**外部 A2A** = 第 6 类 carrier，`client_id="a2a"`（非白名单内 CLI，独立路由到 `a2aAdapter`）。

### 4.6 关键前端组件

| 组件/文件 | 职责 |
|-----------|------|
| `web/src/store/useChatStore.ts` | 编排核心状态：`events[threadId]`、`isGenerating`、`lastSeq`、WS 事件分发 |
| `web/src/services/ws.ts` | `WsManager`(WS 连接/重连/批量) + `sendWakeHold()` |
| `web/src/components/workspace/StreamTimeline.tsx` | 同一时间线渲染多犬事件流（彩色 `BreedCard`） |
| `web/src/components/workspace/CustodyTrail.tsx` + `hooks/useCustodyTrail.ts` + `services/custody.ts` | **球权轨迹面板**（8 态中文、回合/传球/持球统计、事件轨迹）+ 人工唤醒按钮 |
| `web/src/components/workspace/CommandBar.tsx` + `MentionPopover.tsx` | `@` 触发 mention 弹层，插入 `@{breedId}`（含 `@远程协作者`） |
| `web/src/components/settings/MemberManagement.tsx` | 狗狗队伍成员/默认犬/Leader 编辑 |
| `web/src/types/api.ts` + `types/index.ts` | `WsEvent`/`BreedConfig`/`Variant`(含 `a2a_url`)/`StreamEvent` 契约类型 |

### 4.7 受控外部 A2A 协议客户端接入（§4.7 落地细节）

**配置驱动启用**（默认关闭，不污染运行时）：

- **字段**：`pkg/pack/breed.go` `Variant.A2AURL`（行 66，json tag `a2a_url,omitempty`）；与 `ClientID`/`DogID` 同级。
- **示例犬**：`packs/default/breeds/dog-template.json` 的 `a2a-remote` 犬（约 1168-1226 行）：`client_id:"a2a"`、`a2a_url:""`（空=不启用）、犬级 `enabled:false`、`restrictions` 明确"仅在 a2a_url 与凭据就绪时启用；平台不为其开放入站控制面"。
- **平台接线**：
  - `platform.go:179` `a2aAdapter := a2aadapter.New(pm, "a2a")`
  - `platform.go:186` `adapters["a2a"] = a2aAdapter`
  - `platform.go:284-294` 遍历 breeds/variants，若 `v.ClientID!="" && v.A2AURL!=""` → `a2aAdapter.SetEndpoint(v.ClientID, v.A2AURL, "")`(289)；非 `"a2a"` 的 clientID 额外 `adapters[v.ClientID] = a2aAdapter`(291)（不同外部 agent 独立寻址）。
- **端点解析优先级**（`adapter.go:93 lookupURL`）：`a2a.urls[clientID]` > env `SG_A2A_URL_<UPPER(clientID)>` > 全局 `SG_A2A_URL`。
- **协议**：`Execute`(`adapter.go:112`) 组装 `a2aprotocol.JSONRPCRequest{Method:"tasks/send"}`，POST 到 endpoint，可选 `Bearer` token，120s 上下文超时；解码 `JSONRPCResponse` 后 `streamTask` 把外部 Task 的 `artifacts`/`history` 文本映射为 `unified.StreamEvent{Type:"text"|"error"|"done"}` 回灌。
- **球权守卫**：外部 A2A 派发**仍**经 `handleA2AHandoff` 的 G1 `tryDispatch` + §4.5 `Snapshot` 读校验（§4.7 约束：外部 agent 不绕过球权账本）。

### 4.8 CVO 升级闭环（G4 深度硬轨 → 操作员决策 → 重派，2026-08-22 接通）

> 背景：`execution.go` 的 G4 分支此前只发 `SYSTEM_NOTICE` + 停球，前端 `CvoEscalation.tsx` 的三个决策按钮是空函数、徽标永不点亮（死功能）。本节固化 2026-08-22 打通的全栈闭环。

**协议**（`pkg/protocol/event.go:217-256`）：

- `CVO_ESCALATION`（后端→前端）：payload `CvoEscalationPayload{escalation_id, reason, max_depth, from_breed, to_breed, options[]}`；每个选项 `CvoEscalationOption{id, prompt}` —— **label 不上线**，前端按 option id 本地化（`workspace.escalation.optionA/B`），只有语义 id 和服务端预置 prompt 过线。
- `CVO_ESCALATION_RESPONSE`（前端→后端）：payload `CvoEscalationResponsePayload{session_id, escalation_id, decision}`，decision = 选项 id 或 `intervene`。

**后端三段**：

| 段 | 位置 | 行为 |
|----|------|------|
| 发射 | `execution.go:891` `emitCvoEscalation`（实现在 `ws_handler.go:382`） | 深度熔断点在 `recordBall(BallHandedCVO)` + `SendSystemNotice` 后，生成 uuid、登记 `h.escalations` 注册表（`ws_handler.go:40/47`，mu 保护、应答即删）、推事件到该 session 的 streamer。预置两选项：`option_1` 接手拆解 / `option_2` 收尾总结（prompt 为服务端固定中文指令，不信任客户端回传的 prompt） |
| 接收 | `ws_handler.go:170-202` 读循环分支 | 解析响应 → `handleEscalationResponse`(427)：`takeEscalation`(415) 原子取出（未知/已处理 → SYSTEM_NOTICE 提示，不重放） |
| 重派 | `handleEscalationResponse` → `routeAndDispatch`(271) | 命中选项且带 prompt → `bindStreamer` 重绑当前连接 → 按**普通用户消息同一路径**（MentionRouter 路由 + BARK_START + 信号量限流 + worklist 预算）重新派发；`intervene` → 仅发确认通知，等待操作员在 CommandBar 手动指挥。`routeAndDispatch` 是从 USER_INPUT 块抽出的共享派发路径，两条入口（用户消息 / 升级决策）行为一致 |

**前端**：

| 组件 | 行为 |
|------|------|
| `useChatStore.handleWsEvent` | `CVO_ESCALATION` case：追加 `cvo_escalation` StreamEvent（携带 threadId/escalationId/reason/maxDepth/options）并置 `escalations[threadId]=true` |
| `useChatStore.resolveEscalation(threadId, decision, escalationId)` | `WsManager.sendEscalationResponse` 回传 → 移除对应升级卡（按 escalationId 精确匹配）→ 无剩余卡时清 `escalations[threadId]` |
| `CvoEscalation.tsx` | 三按钮（接手/收尾/人工介入）接 `resolveEscalation`；标题行渲染 `max_a2a_depth` 与 reason |
| 徽标消费方 | `PrimaryNav`（threads 钮 rose 脉冲徽标 = 任一线程有未决升级）、`ThreadItem`（CVO 徽标 + 警示图标）、`ThreadList`（escalated/active 筛选，改读 `escalations` store 而非后端 thread 字段） |

**边界与已知限制**：升级注册表为进程内存（重启即失，与机会瞬态设计同哲学）；**重载恢复**——`GET /api/escalations`（`routes.go`，auth.Wrap）返回未决升级投影（按创建时间排序），前端 `App` 挂载时 `restoreEscalations()` 重放升级卡并点亮标记（按 escalation_id 去重，旧后端 404 静默），2026-08-23 起刷新页面后卡片可恢复，仅服务重启丢失；响应中的 prompt 永远取服务端注册表里的预置值，客户端传来的只有 decision id —— 防止把执行指令的信任交给前端。

---

## 5. Story 级 Definition of Done (DoD Checklist)

- [x] A2A Communication 主链路文档化（前端发起 → WS → 后端路由 → CLI/A2A 执行 → 流式 → handoff 传球 → 球权账本 → 轨迹呈现）已固化于本文。
- [x] 两层 A2A 职责明确：内部协作（@提及+CLI+球权账本）主路径 与 受控外部 A2A 客户端（§4.7，tasks/send）并列 carrier，均经 `unified.AgentExecutor` + 球权守卫。
- [x] 技术契约完整：WS 事件表、custody 账本事件、REST 端点、关键组件清单、§4.7 接入细节、配置/红线。
- [x] 红线已闭合：§4.7 允许受控客户端、禁 server（`s8-a2a-format/template.md:32/44-46` 已对齐；AGENTS.md 红旗 53/55 已改写）。
- [x] 死代码已清：`CustodyService` 孤儿簇、`internal/a2a/compressor.go` 已删除；`pkg/a2a` 已激活复用（非死代码）。
- [ ] 单元测试覆盖率达到团队基线（`execution_*`/`mention_router`/`hold_scheduler`/`briefing`/`ball_ledger`/`a2a/adapter_test` 等已覆盖核心链路）。
- [ ] 静态代码扫描无 P0/P1 级安全漏洞（`go build ./...` + `go test ./...` + `tsc -b` 全绿为现状基线）。
- [ ] 监控告警与降级开关在预发/灰度环境验证正常（当前依赖 Ops 端点人工观察，自动告警待补）。

---

## 6. 已知缺口处置总览（截至 2026-08-17）

| # | 缺口/议题 | 状态 | 证据 |
|---|-----------|------|------|
| A-1 | 外部 A2A 仅同步单轮 `tasks/send` | ⚠️ **部分（与 clowder Phase 3 同档）** | `adapter.go:112` 同步调用；流式订阅/AgentCard 自动发现未实现（留 §4.7 框架内扩展，不撞红线） |
| A-2 | `s8-a2a-format/template.md` 与 §4.7 冲突 | ✅ **已修复（文档）** | `template.md:32/44-46` 已对齐 §4.7（旧 HTML 报告的"过时误判"以本文为准） |
| A-3 | `custody` 包注释「只写不读」过时 | ✅ **已修复（注释）** | `ball_ledger.go` 包注释 + `nextGeneration` 注释已改写（读驱动） |
| A-4 | `CustodyService` 孤儿实现 | ✅ **已删除（代码）** | 全仓 Grep "CustodyService" 无 Go 命中（仅旧 HTML 提及） |
| A-5 | `internal/a2a/compressor.go` 死代码 | ✅ **已删除（代码）** | `Platform.Compressor` 字段已移除（`platform.go` 三处） |
| A-6 | `pkg/a2a` 曾零引用死代码 | ✅ **已激活（代码）** | `adapter.go:30` import 复用；全仓仅 2 处引用 |
| A-7 | 无内部 A2A server / 不入站控制面 | ✅ **红线禁止（设计约束）** | AGENTS.md:53/55；不可逆决策 §4.7 |
| A-8 | CVO 升级链路前端死功能（按钮空函数、无事件源、徽标永不亮） | ✅ **已修复（全栈，2026-08-22）** | 协议 `pkg/protocol/event.go:217-252` + `execution.go:891` 发射 + `ws_handler.go:427` 决策重派 + 前端 `CvoEscalation.tsx`/`escalations` store 接通；AC-14 与 `useChatStore.test.ts`（含多级升级部分解决用例）锁定行为。详见 §4.8 |

**无 P0/P1 阻塞项**：A2A 主链路（含外部客户端）无阻断性缺口。仅 A-1（流式订阅/自动发现）为特性增强，可在 §4.7 框架内排期，不构成红线冲突。

---

> 关联文档：`docs/designs/FT-ORC-001-multi-agent-orchestration.md`（编排总览）、`不可逆决策`(§4.7)、`平台能力清单`(§6.2 A2A Protocol Client)、`packs/default/hooks/s8-a2a-format/template.md`、`docs/reference/API.md`(Pack API)。
