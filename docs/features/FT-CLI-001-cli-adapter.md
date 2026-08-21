# [FT-CLI-001] [Tech Story] CLI Adapter 前后端逻辑全链路设计

> 本文基于 `sounds-great-ai` **2026-08-15 代码实况**，逐文件（`internal/adapter/**`、`internal/transport/**`、`pkg/protocol/**`、`web/src/**`）梳理 CLI adapter 的**后端执行链路**与**前端消费链路**，所有结论带 `文件:行号` 锚点，未臆测。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu | QA: @demu
- **故事点/复杂度**: [ L (8分) ]
- **范围**: CLI adapter 全域——5 个 provider（claude/codex/gemini/opencode/kimi）的执行入口、统一子进程底座（unified）、传输层（transport）、协议（protocol）、前端消费（web）。

- **业务/技术目标**:
  - As a **平台运行时（The Pack）**,
  - I want to **把一只 breed 的 CLI 调用（claude/codex/gemini/opencode/kimi）以 one-shot NDJSON 进程驱动，并把流式正文、工具调用、终端输出、活体探测、结构化诊断无损地送达前端时间线**,
  - So that **用户能在对话里实时看到 agent 的思考、回复、工具执行与外部 CLI 的 stdout/stderr，并在失败时看到分类脱敏的诊断，而不是黑盒或裸堆栈**。

- **关键指标/埋点**: 无（内部执行链路，非对外曝光计费功能）。

---

## 2. 功能叙事：一次 CLI 调用的端到端旅程 (User Journey)

用户向某只 breed 发消息 → 后端 `agent_executor` 路由到该 breed 的 provider adapter `Execute` → adapter 拼命令行参数并选择 carrier → 默认走 `print_sdk` 档（`ProcessTransport` 包装 `ProcessManager.Spawn` 起一个 one-shot 子进程）→ 子进程 stdout 吐 NDJSON 行 → adapter 用 `ParseNDJSON` 逐行解析并映射成 `StreamEvent`（`text`/`tool_call`/`tool_result`/`thinking`/`error`/`done`）→ 经 `convertStreamEvent` 转成 `protocol.Event` → 经 WS 推到前端 → 前端 `useChatStore` 累加语义事件 → `StreamTimeline` 渲染。进程退出后 `EmitDiagnosticsIfNeeded` 决定是否补一条结构化 `error` 事件。

```
用户 ──消息──▶ agent_executor.go ──▶ <Provider>Adapter.Execute
                                            │ 拼 CLI args / flags
                                            ▼
                                  registry==nil ? pm.Spawn : RunCarrierFallback
                                            │ 默认 [print_sdk] 单档
                                            ▼
                                  ProcessManager.Spawn  (one-shot 子进程)
                                            │ stdout NDJSON 行
                                            ▼
                                  ParseNDJSON → parse<X>Event → StreamEvent
                                            │
                                            ▼
                                  convertStreamEvent (helpers.go:134)
                                     text→AGENT_MESSAGE(live) / tool_call / error→BARK_ERROR / stall→LIVENESS / done→丢弃
                                            │ WS
                                            ▼
                                  useChatStore (事件总线, 累加语义事件)
                                            │ breed_response_live 累积
                                            ▼
                                  StreamTimeline.groupCliRuns → 各 Block 渲染
                                            │ 进程退出
                                            ▼
                                  EmitDiagnosticsIfNeeded → (失败则) BARK_ERROR
```

**语义事件 vs NDJSON 行的关键映射**（后端 `parse<X>Event` + `convertStreamEvent` 两段）：
- `assistant_response`/`message`/`text` → `StreamEvent{Type:"text"}` → `EventAgentMessage{Done:false}`（前端 `breed_response_live` 增量累积）。
- `tool_use`/`function_call`/`tool` → `StreamEvent{Type:"tool_call"}` → `EventToolCall`。**结果内嵌于 tool_call 的 `result` 字段**，后端协议无独立 `tool_result` 事件（`ToolCallPayload` 含 `result`）。
- `tool_result` → `StreamEvent{Type:"tool_result"}`（仅 claude/gemini 等显式返回）。
- `thinking` → `StreamEvent{Type:"thinking"}` → `EventThinking`（kimi 额外产出）。
- CLI 的 stderr / 终端输出 → 前端 `terminal_output` 增量累积（`StreamEvent{Type:"terminal_output"}`）。
- `result`/`done` → `StreamEvent{Type:"done"}` → `convertStreamEvent` **default 分支返回 nil，被丢弃**；终止由显式 `EventBarkResult`（`BARK_RESULT` + `Content`）承载（终态带最终文本，免 REST 水合）。

---

## 3. 前端处理逻辑 (web/src)

### 3.1 WS 连接与事件总线

- `services/ws.ts` `WsManager.connect` (16-63)：`new WebSocket(\`${base}/ws\`)``, `base` 经 `replace(/^http/, 'ws')`。**无协议级心跳/ping**：活性靠 `onclose`/`onerror` 推断；`initWebSocket` 每 500ms 轮询 `manager.readyState` 写入 `wsReadyState`（99-105）。`onmessage` (32-53) 同时支持**单事件与批量数组**（服务端 micro-batching）；畸形事件（`!type` 字符串）丢弃。重连 `scheduleReconnect` (65-74) **仅 5 次**，线性退避 `1000*attempt` ms。
- `store/useChatStore.ts` 是事件总线。`CARRIER_HEALTH` 在 `resolveThreadId` **之前**处理（191-210，全局事件无 `session_id`，避免被丢弃），写入 `carrierHealth` map（key=carrier，值含 `level/transport/reason/remainingMs/updatedAt`）。`seq` 间隙仅 `console.warn`。

### 3.2 事件分支与状态写入

| WS 事件 | 行号 | 行为 |
|---|---|---|
| `BARK_START` | 231-242 | `isGenerating[thread]=true` + append `breed_response_start` |
| `THINKING` | 244-272 | 末事件同 `step` 且 `running` 则拼接 `content`，否则新事件 |
| `TOOL_CALL` | 274-286 | append `tool_call`（含 `result`，无独立 tool_result） |
| `TERMINAL_OUTPUT` | 301-327 | 末事件同 `stream` 则拼接 `data`，否则新事件 |
| `AGENT_MESSAGE` | 342-369 | append/累积 `breed_response_live`（`done` 字段保留但未使用） |
| `AGENT_LIVENESS` | 371-383 | append `breed_stall_warning` |
| `BARK_RESULT` | 385-434 | **live→complete 桥接**：running thinking→`success`；按 breed 找在途 `breed_response_live` 原地替换，`content = p.content 非空 ? p.content : live.content`（优先服务端 Content）；`isGenerating=false` |
| `BARK_ERROR` | 436-467 | running thinking→`error`；append `error` 事件，透传 `breed/error/reason/summary/hint/excerpt/source/meta` 全字段；`isGenerating=false` |
| `BARK_REJECTED` | 469-478 | 仅 `isGenerating=false` + toast |
| `ERROR` | 480-487 | **仅 toast，不进 timeline** |
| `SYSTEM_NOTICE` | 489-493 | 写入 notice store |
| `default` | 495-497 | `console.warn('Unhandled event type')` |

**关键差异**：后端诊断走 `BARK_ERROR` → 进普通 timeline（`error` 类型事件）；而 `ERROR` 事件只弹 toast。**`system_info`/`carrier_fallback` 在前端无任何分支**，落入 `default` 仅 `console.warn`（`CARRIER_HEALTH` 之前的全局处理只覆盖 carrier 健康）。

### 3.3 时间线渲染

- `StreamTimeline.tsx`：`isCliRun` (24-26) = `tool_call||terminal_output`；`groupCliRuns` (31-51) 仅合并**相邻连续的** CLI 事件为 `{kind:'cli'}` 组（中间插入 thinking 会拆成两个组）。渲染 switch (84-159)：cli 组→`CliOutputBlock`；`error`→`ErrorBlock`；`breed_response_live` 内联带闪烁光标 (103-129)；`breed_stall_warning` 按 `active/hard` 配色 (130-154)。breed 配色来自 `getBreedColor`（`lib/breed-colors.ts`，回退 `BREED_COLOR_DEFAULTS`）。
- `ErrorBlock.tsx`（CliDiagnosticsPanel 风格）：`tier = classifyErrorTier(reason, summary||error)` (22)；headline 优先 `summary` 再 `error`，均 `sanitizePathLeaks` (27-29)；badge 显示 `reason` 否则 tier (45)；详情折叠 (74-98)：`safeExcerpt.show`→原文 `<pre>`，否则占位文案或回退 `event.error`。**元信息脱敏值并非红色**：meta 条与占位文案实际为 `text-slate-500`，红色只用于 tier 边框/底色/图标/徽章（`TIER_STYLES`）。
- `CliOutputBlock.tsx`（新建）：单卡片折叠（默认 `open=true`），头部摘要 `toolCount/stdout/stderr/errCount`，tone：err>0→rose，有 stderr→amber，否则 slate (21-30)；内部 `ToolLogBlock` 列表 + 各 stream 的 `<pre>`（stderr=rose-400，stdout=emerald-400）(60-77)。
- `ToolLogBlock.tsx`：名称+参数+状态图标（success emerald / error rose / 其他 amber spinner）+ 可展开 `result` (9-38)。
- `ThinkingBlock.tsx`：状态图标按 `error/success/running` 切换，`userOverride` ref 保留手动展开 (12-53)。
- **遗留/取代**：`TerminalOutputBlock.tsx` 已无任何引用（死代码），被 `CliOutputBlock` 取代。

### 3.4 诊断工具库 `lib/diagnostics.ts`

- `KNOWN_EXCERPT_SOURCES` (81-85) = `{'cli_stderr','cli_stream','server'}`（白名单）。
- `PATH_RE` (88) 匹配 `/Users|/home|/root|/tmp|/var|/opt|/srv` 路径。
- `sanitizePathLeaks` (93-107)：`HOME` 前缀→`~`，路径→`[path]`。
- `safeExcerpt` (112-124)：无 excerpt→`{show:false}`；source 在白名单→`{show:true, sanitizePathLeaks}`；否则→`{show:false, text:'原始错误详情已隐藏（来源未授权展示）。'}`。
- `redactMeta` (128-135)：对每个 meta 值套 `sanitizePathLeaks`。
- `classifyErrorTier` (70-76) + `TIER_STYLES`(17-42)：四档配色。

### 3.5 连接状态 UI `ConnectionStatusBar.tsx`

三态 pill：`本地 API`/`Socket` 用 `socketOnline=wsReadyState===1`；`上游模型` 用 `LevelPill`。**取数优先级** (61-91)：优先 `carrierHealth` map（取最坏 level：offline>degraded>online，理由取最新 `updatedAt`）；无 carrier 事件时回退扫描 timeline 末事件：遇 `hard` stall_warning 或 `error`→degraded，遇 `complete`/`live` 即清白停止。

---

## 4. 后端处理逻辑 (internal)

### 4.1 五个 provider adapter 的统一执行入口

五个 adapter 均实现 `unified.AgentExecutor`（`executor.go:10-19`），`Execute(ctx, req) (<-chan StreamEvent, error)`：

| adapter | 关键标志 | stream-error↔SetStreamError |
|---|---|---|
| `claude/adapter.go:49` | `--output-format stream-json --model --cwd --append-system-prompt`（L85-104）；MCP 写临时文件 `--mcp-config`（L97-101） | 是（L140） |
| `codex/adapter.go:48` | `exec --json --model -c developer_instructions=`（L81-97） | 是（L131） |
| `gemini/adapter.go:47` | `--output-format stream-json --model`（L51-54）；**不支持 MCP**（Capabilities L34） | 是（L109） |
| `opencode/adapter.go:48` | `--output ndjson --mcp-config`（L81-91） | 是（L125） |
| `kimi/adapter.go:58` | `-p <prompt> --model --cwd --output-format stream-json`（L84-94）；**prompt 走命令行参数而非 stdin**（L62 `StdinInput:""`），用信封包裹（L100-123）；额外产出 `thinking`（L169） | 是（L145） |

**统一行为（5 个一致）**：优先 `if a.registry != nil` → `unified.RunCarrierFallback(...)`；`registry==nil`（测试/legacy）直连 `pm.Spawn`。`streamEvents` 内注册 `SetOnStall`→发 `stall_warning`（非阻塞）；`ParseNDJSON` 循环遇 `IsParseError` 即 `h.SetStreamError(pe.Line)` 并转发 `error` 事件；循环结束调 `EmitDiagnosticsIfNeeded(h, ch, sawError)`。

### 4.2 统一子进程底座 `internal/adapter/unified/`

- **`process.go`** `ProcessManager.Spawn` (238-311)：先 `resolveSupervisor()` (179-193)——优先 `SupervisorBinary`，否则查同目录 `sg-cli-supervisor`，再查 PATH，都没有则 `""`。非空经 sidecar 启动（`exec.CommandContext(sup,"--",cmd,args...)` + `Setpgid`），并把真实子 PID 写 `SG_SUPERVISOR_CHILD_PID_FILE`；sidecar `Start()` 失败则**优雅回退直连**并打日志 (270-289)。`finishSpawn` (316-426)：`trackPID` 默认 `c.Process.Pid`，sidecar 模式改读真实 CLI PID；进程组 `Setpgid` 使 ctx 取消能 `-pgid` 组杀（SIGINT→`InterruptGraceMs`→SIGTERM→`KillGraceMs`→SIGKILL）。stderr 用 `cappedWriter` 限长 `StderrBufSize=16384`（只保留头部 16KB）。`SetStreamError` (74-80) 只保留**第一条**错误文本；`SetOnStall` (64-68) 桥接。`NewLivenessProbe` + `probe.Start()` (378-394)，其 `OnStall` 同时调 `pm.OnStall` 与 handle 的 `onStallCb`，`ProbeIdleSilent` 时置 `handle.stalled=true`。
  - **实测：生产代码从未设置 `SupervisorBinary`**（全仓仅测试赋值）→ 默认恒走直连路径。

- **`ndjson_parser.go`** `ParseNDJSON` (24-44)：逐行 `json.Unmarshal`，失败 yield `ParseError` 哨兵（**绝不抛错中断流**）；空行跳过；scanner buffer 上限 **1MB/行**。

- **`registry.go`** `TransportKind` 四档（bg_daemon/interactive_pty/print_sdk/api_key，L13-27）；`DefaultTransports`（L30-35）全序链。`ResolveChain`（L119-130）未注册回退默认四档；`ExecuteFrom`（L196-245）从 `startIndex` 起逐档尝试，健康度非 online 则广播 `CarrierHealthEvent` 并跳过；`nextHealthyTierIndex`（L171-189）**`api_key` 永不作为回退目标**（避免虚假 `carrier_fallback`）。

- **`resolve.go`** `ResolveCLICommand`（L26-87）：先查缓存（失效 `os.Stat` 失败则 `InvalidateCLICache` 重解）→ `LookPath` → 遍历 `knownCLIDirs`（`.local/bin`/`.claude/bin`/nvm/volta/nix 等）→ brew bin。`candidateNames`（L104-112）仅 Windows 追加 `.exe/.cmd/.bat/.ps1`；`isExecutable` Windows 按扩展名判定。

- **`diagnostics.go`** reasonCode 常量（L9-29，含 `auth_failed`/`quota_exceeded`/`cli_stall_timeout`/`cli_response_timeout`/`upstream_policy_reject`）；`ClassifyError`（L59-66）+ `classifierPatterns`（L40-57）；`SanitizeStderr`（L79-90）脱敏 JWT/PEM/cookie/query/tokenKV/path/高熵 → `[REDACTED-*]`；`BuildDiagnosticsFrom`（L120-147）**双源合并**——优先 stderr，空才用 `streamErr`（`SetStreamError` 记录），摘录超 200 字符截断；`EmitDiagnosticsIfNeeded`（L171-208）`Wait()` 后若 `stalledFlag()` 且未分类→强制 `ReasonStallTimeout`，仅当 `!sawError` 才发结构化 `error` 事件，Meta 写 `reason/summary/hint/excerpt/source`（source 硬编码 `"cli_stderr"`）。

- **`probe` 系列**：`ProbeState`（active/busy_silent/idle_silent/dead）；阈值 `ProbeSoftWarnMs=30000`/`ProbeStallWarnMs=120000`；`getProcessCPUTime`——Linux 读 `/proc/<pid>/stat`（utime+stime/CLK_TCK），Darwin `ps -o time=`，其它平台恒 0（**非 Linux/Darwin 仅存活检测有效**）。`hardStallNotified`（L29）保证硬 stall 只告警一次、恢复复位。`LivenessMessage`（L38-52）仅安全提示文案。

- **`carrier.go` / `fallback.go`** `RunCarrierFallback`（fallback.go L77-141）：**最多两次尝试**（激活档 + 单次回退跳）。`ExecuteTier` 失败→`ClassifyError`：瞬态（network/overload/timeout/stall/policy）记失败并直接透传不回退；非瞬态（quota/auth/context/model/config/overloaded/policy，`IsFatalMidStream` L19-31）记失败→`nextHealthyTierIndex` 单次跳并发 `system_info` 类型 `carrier_fallback` 事件；无更好档（或只剩 api_key）则透传原错误不发伪回退。`monitorYieldedErrors`（L147-170）：流中 `error` 若非瞬态则记失败供**下一调用**跳过——**不中途换 carrier**（部分输出已下发，中途换档会导致重复输出）。

- **`pool/warm.go`** `WarmProcess`（L28-45）持真实 OS 进程；`Acquire`（L298-373）六步：① cwd 丢失则退役(#1203) → ② session 亲和复用 → ③ key 命中复用 → ④ 死条目清理 → ⑤ 单飞 stale-lease 强释(#992) → ⑥ 超容量 LRU 后冷启。`IsCwdIntact`（L138-144）；`Release`（L418-438）**不杀进程**仅减 lease；TTL 默认 30min、`MaxLeaseMs=0`（关闭强制回收）、`SupportsMultiplexing=false`（默认单飞）。`RegisterWarmPool`（platform.go L375-387）opt-in，**全仓生产无调用点** → warm 池默认关闭。

- **`health.go` / `health_redis.go`** `CarrierHealth` 接口（L35-45）；`MemoryHealth` 默认；`reasonTier`（L56-66）映射冷却档：`quotaCooldown=4h`/`structCooldown=30min`/`transientCooldown=3min`/`transientStrikes=3`。`RedisHealth`（health_redis.go L17-30）**已编译进默认二进制**，仅当配置 Redis URL 才激活；key `sg:carrier-health:`，TTL=冷却时长。

- **`pty` 系列（`//go:build pty` 约束）**：`PtyConfig.TmuxMode` 默认 false（pty_config.go L41）。`PtyTransport.Spawn`（pty_real.go L74-152）先判 `TmuxMode && command=="claude" && tmuxAvailable()` 才走 `spawnViaTmux`，失败回退直连 pty；`readyAndBypass` 固定 grace + 可选 consent 绕过；cancel 发 ESC 保活。`pty_stub.go`（`!pty`）`Spawn` 恒 error → 注册表回退下一档——**默认构建即此 stub**。`pty_tmux.go`/`pty_tmux_stream.go`（仅 pty 编译）：claude 在 detached tmux 会话运行，读 `~/.claude/projects/<slug>/*.jsonl` transcript + `.claude/settings.json` 注入 Hook 侧信道（Stop=终止信号）；`buildClaudeTmuxCommand` 剥离 `--output-format stream-json`；`tmuxStreamCoordinator.finish` 合成 `result`、kill tmux、恢复 settings。

- **`supervisor`** `internal/supervisor`：`Spawn`（supervisor.go L125-136）`Sidecar=false` 走 in-process monitor，`Sidecar=true` re-exec `sg-cli-supervisor`。`cmd/sg-cli-supervisor/main.go`：CLI 放独立进程组（`Setpgid` L92），每 `SG_SUPERVISOR_POLL_MS` 轮询父存活（L179-188），父消失则 `signalChild(-pgid)` 组杀（L139）；退出前 `defer signalChild(SIGKILL)`；向调用方写真实子 PID 文件（L118-121）。

### 4.3 传输层与协议

- `transport/helpers.go` `convertStreamEvent`（L134-190）：`thinking`→`EventThinking`；`text`→`EventAgentMessage{Done:false}`（live 增量）；`tool_call`→`EventToolCall`；`error`→`EventBarkError`（从 `ev.Meta` 取 `reason/summary/hint/excerpt/source`，`meta` 经 `map[string]any→map[string]string` 转换，L152-159）；`stall_warning`→`EventAgentLiveness`；**其余（含 `done`）→ `default: return nil`**（传输层丢弃 done，终止由显式 `BARK_RESULT` 承载）。
- `transport/execution.go`：消费循环 `for ev := range eventCh`（L214-227），text 累积进 `responseText` 经 hold_ball 过滤；结束发 `EventBarkResult`（L274-279 hold / L294-300 正常），`Content: cleaned`（终态带最终文本）。**`breed_response_live`=AGENT_MESSAGE 流式累积，`complete`=终态 BARK_RESULT+Content**。
- `pkg/protocol/event.go` `BarkErrorPayload`（L156-171）字段 `Breed/Error/Reason/Summary/Hint/Excerpt/Source/Meta`；`EventAgentMessage`（L109）+`AgentMessagePayload`（L141-145，含 `Done` 保留位）；`EventAgentLiveness`（L113）+`LivenessPayload`（L148-153）；`BarkResultPayload.Content`（L137，optional）。

### 4.4 装配 `platform.go`

`New`（L146-352）：`pm := unified.NewProcessManager()`（L147）；adapters map 5 个（均 `claude.New(pm)` 等，**不接 registry**）；`carrierHealth` 默认 `MemoryHealth`，**仅当 `cfg.RedisURL!=""` 才换 `RedisHealth`**（L163-170）；注册 transport：`NewProcessTransport(pm)` + `NewPtyTransport()`（注释 PTY reserved，不在默认链）；给每个 adapter `SetRegistry(registry, name)`；**每个 carrier 只注册 `[TransportPrintSDK]`**（L188-194）→ 默认链恒为单档直连。`RegisterWarmPool`（L375-387）opt-in，仅测试引用。`SetHealthBroadcaster`（L364-368）把 WS hub 接入 `registry.SetBroadcaster`。

---

## 5. 技术契约与接口 (Technical Contract)

### 5.1 后端 → 前端 协议事件（经 WS，adapter 层视角）

> **完整 WS 事件契约以 `FT-ORC-001` §4.2 为单一真相**（含编排层事件与前端映射）。本节仅列出**由 CLI adapter / transport 层构造并发送**的事件，聚焦其 `pkg/protocol/event.go` 中的 payload 字段细节（adapter 开发者对照用）：

| 协议事件（adapter 构造） | Payload 关键字段（`pkg/protocol/event.go`） | 前端落点 |
|---|---|---|
| `AGENT_MESSAGE` | `AgentMessagePayload{Breed,Content,Done}` | `breed_response_live`（增量累积） |
| `TOOL_CALL` | `ToolCallPayload{Tool,Params,Result}` | `tool_call` |
| `TERMINAL_OUTPUT` | 自定义 `{Stream,Data}` | `terminal_output`（增量累积） |
| `THINKING` | `ThinkingPayload{Step,Content}` | `thinking` |
| `AGENT_LIVENESS` | `LivenessPayload{Breed,State,Hard,Message}` | `breed_stall_warning` |
| `BARK_RESULT` | `BarkResultPayload{Breed,Content,Steps}` | `breed_response_complete`（live→complete 桥接） |
| `BARK_ERROR` | `BarkErrorPayload{Breed,Error,Reason,Summary,Hint,Excerpt,Source,Meta}` | `error` 事件 |
| `CARRIER_HEALTH` | `CarrierHealthPayload{carrier,level,transport,reason,remaining_ms}` | `carrierHealth` map（ConnectionStatusBar） |
| `ERROR` | 文本 | 仅 toast |

### 5.2 前后端类型对齐

- `web/src/types/index.ts` `StreamEvent` 联合（L158-172）：**无 `done`/`tool_result` 独立类型**；`ErrorEvent`（L135-150）含 `error` 必填 + `reason/summary/hint/excerpt/source/meta` 可选；`ToolCallEvent`（L74-80）含 `result`；`TerminalOutputEvent`（L89-93）含 `stream/data`；`BreedStallWarningEvent`（L127-133）含 `state/hard/message`；`BreedResponseLiveEvent`(119-123)/`BreedResponseCompleteEvent`(108-115，`content?`)。
- `web/src/types/api.ts` `WsEvent`（L2-8，`seq?`）；`BarkErrorPayload`(198-208) 字段与 `ErrorEvent` 一致；`AgentMessagePayload`(184-188) 含 `done`；`BarkResultPayload`(175-182) 含 `content?`；`CarrierHealthPayload`(214-220，`remaining_ms`)。后端 snake_case，store 映射 camelCase（`remaining_ms`→`remainingMs`，L204）。

---

## 6. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- **[x] 资损与网络安全 (Security)**
  - 敏感数据脱敏：后端 `SanitizeStderr`（diagnostics.go L79-90）把 JWT/PEM/cookie/tokenKV/路径/高熵串 → `[REDACTED-*]`；前端 `sanitizePathLeaks`（diagnostics.ts L93-107）把 `HOME`→`~`、绝对路径→`[path]`。
  - **白名单门控原始错误披露**：`safeExcerpt`（L112-124）仅当 `source ∈ {cli_stderr, cli_stream, server}` 才展示原文，否则替换为「原始错误详情已隐藏（来源未授权展示）」。防止未分类 stderr 误泄敏感行。
  - 凭据：MCP 配置走 `os.CreateTemp` + `OnExit(os.Remove)` 清理，不留痕。

- **[x] 高并发与限流降级 (High Availability)**
  - **默认 carrier 链只含 `print_sdk`**：warm 池、PTY 均 opt-in（`RegisterWarmPool` / `-tags pty`），默认行为等价旧版 one-shot、零新增依赖。**`!pty` 默认构建恒走直连 `pty.Start` 或 stub 回退**。
  - **多重优雅回退**：supervisor 二进制缺失 → 直连（`process.go` L270-289）；`spawnViaTmux` 失败 → 直连 pty（`pty_real.go` L79-85）；`api_key` 末档永不触发回退（`registry.go` L175-176）；中途流错误不换 carrier（`monitorYieldedErrors` 只记失败供下次跳过）。
  - CLI 命令解析带缓存失效保护（`InvalidateCLICache`）防 ENOENT 死循环。
  - process 组隔离（Setpgid + `-pgid` 组杀）防孙进程孤儿；supervisor sidecar 兜底父暴毙。

- **[x] 可服务性与监控 (Serviceability)**
  - 活体探测真实读取 CPU（Linux `/proc` / Darwin `ps`），软 30s / 硬 120s，`OnStall` 经 `SetOnStall` 桥接前端 `AGENT_LIVENESS`，硬卡死只报一次（`hardStallNotified`）+ 恢复通知。
  - carrier 健康度状态机（quota 4h / structural 30min / transient 3 连击升级）→ `CARRIER_HEALTH` WS 全局广播 → 前端 `ConnectionStatusBar` 三态 pill。
  - stderr 16KB 上限（`cappedWriter`）防爆内存；NDJSON 单行 1MB 上限；解析失败时 yield 哨兵不中断流。

---

## 7. Story 级 Definition of Done (DoD Checklist)

- [x] **端到端链路有据可查**：后端 5 adapter → unified 底座 → transport → 前端 store → StreamTimeline，全链路 file:line 锚定，无臆测。
- [x] **单元测试闸门**：`go test ./...`（默认）+ `go test -tags pty ./internal/adapter/unified/... ./pkg/protocol/...` 全绿；`pkg/protocol/event_test.go` 的 `bark_error` 往返测试已改用 `reflect.DeepEqual`（Meta 为 map 不可 `!=`）。
- [x] **前端编译闸门**：`web/tsc -b` + `vite build` 全绿（`dist/` 产出）。
- [x] **脱敏与白名单门控验证**：`SanitizeStderr` / `sanitizePathLeaks` / `safeExcerpt` 逻辑已逐行核对（`ErrorBlock` 元信息脱敏值实际为 slate-500，非红色）。
- [x] **零新增依赖默认保证**：默认构建（`!pty`）恒走直连、MemoryHealth 兜底、无 supervisor 二进制时优雅回退——已源码复核。

---

## 8. 关键设计决策小结

- **默认链路为单档 `print_sdk` one-shot NDJSON**：warm 池、PTY 均 opt-in，默认行为等价旧版，零新增依赖；`!pty` 默认构建恒走直连。
- **多重优雅回退保证可用性**：supervisor 二进制缺失 → 直连；`spawnViaTmux` 失败 → 直连 pty；`api_key` 末档永不触发回退；中途流错误不换 carrier（部分输出已下发）。
- **终止语义**：`done` 事件在传输层被丢弃，终止由显式 `BARK_RESULT`+Content 承载（终态带最终文本，免 REST 水合）。
- **结构化诊断闭环**：后端 `error` StreamEvent 经 `BarkErrorPayload`（Reason/Summary/Hint/Excerpt/Source/Meta）→ WS → 前端 `ErrorBlock`（CliDiagnosticsPanel 风格，脱敏元信息 + 白名单门控可折叠 excerpt）+ `CliOutputBlock` 工具时间线，已于 2026-08-15 落地。
