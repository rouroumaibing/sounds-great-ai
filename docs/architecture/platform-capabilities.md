# 平台能力与狗狗角色（Platform Capabilities & Dog Pack）

> 本文件承接原 `docs/VISION.md` §5（狗狗角色）与 §6（平台能力清单），2026-08-17 拆出。
> VISION 现只保留方向与理念（§0–§3）。

---

## 5. Dog Pack — 狗狗角色

| 角色 | 狗狗 | 代号 | 职责 |
|------|------|------|------|
| Orchestrator | Border Collie | bianmu | 任务分解、动态路由、结果合成 |
| Code Hunter | Xigou | xigou | 代码搜索、分析、重构建议 |
| Knowledge Retriever | Golden Retriever | jinmao | RAG 检索、上下文组装 |
| Log Tracer | German Shepherd | demu | 日志追踪、错误诊断 |
| Presenter | Tibetan Mastiff | zangao | 输出格式化、markdown 渲染 |
| Safety Guard | Chinese Rural Dog | zhonghuatianyuanquan | 命令拦截、路径校验、敏感过滤 |

上表是 6 个 canonical 角色。模板另含**变体犬**（同角色不同 CLI 绑定，如 `jinmao-opencode` 金毛·开源走 opencode 客户端），完整清单以 `packs/default/breeds/dog-template.json` 与运行时 `.sounds-great-ai/dog-catalog.json` 为准。

### 5.1 首启空、按需组队

六犬是**模板 / 菜单**（canonical personas），不是出厂即部署。首次启动 `dog-catalog.json` 为空（仅 owner），用户通过「成员管理 → 从模板添加」把犬加入团队、绑定账号与密钥后，犬才进入运行时。这让「我们养的是团队」（VISION §0.2）落到行为层：团队由用户亲手组建，而非预先灌满。

- **可用性 = 用户开关 ∧ 凭据就绪**。凭据就绪：`oauth` 账号查 CLI 二进制，`api_key` 账号查 `.sounds-great-ai/credentials.json`（配置根解析见 `internal/settings` 的 `ConfigRoot`：环境变量 → `{projectRoot}/.sounds-great-ai` → `{home}/.sounds-great-ai`）。无凭据的犬显示「待配置」而非「已启用」。
- **升级新增的模板犬自动加入 catalog**（`seen_template_breeds` 机制），但同样需绑定凭据才「就绪」。
- **设置数据三文件持久化**（`.sounds-great-ai/` 下）：`accounts.json`（账号元数据）、`dog-catalog.json`（members+leader+系统配置）、`credentials.json`（密钥，0600）。内存 store + 3s 轮询 / 30s 防抖热加载。

见 `docs/designs/FT-MEM-001-member-management.md`（§6 首启空 Catalog + 凭据就绪闸门）。

---

## 6. 平台能力清单

### 6.1 核心运行时（六边形域）

| 模块 | 职责 | 包路径 |
|------|------|--------|
| Orchestration Domains | threads/routing/agents/sop/custody 等六边形域（适配器模式包装扁平包，运行时状态源） | `internal/domains/` |
| Ball Custody Ledger | 球权事件账本（append-only 事件流 + 纯函数投影状态机）+ CAS 租约；编排一等状态源（§4.5） | `internal/domains/custody/` |
| Brief & Trail API | 从球权账本投影简报/轨迹查询端点（GET /api/custody/threads/{id}/trail），支撑多 agent 协作可观测/可审计 | `internal/domains/custody/` 投影 + `cmd/server/routes.go` |
| Code-Repo Trajectory | 可配置代码库地址（默认空）+ git-ref 分支轨迹采集（append-only 事件 + 纯函数投影）。非空时 reconciler 周期采集（5min）落地 `repo-trajectory.json`，与球权事件流 + 聊天消息并列为三源 | `internal/domains/custody/services/git_ref_collector.go` `internal/domains/custody/stores/repo_trajectory_store.go` `internal/transport/repo_trajectory_handler.go` |

### 6.2 平台服务

| 模块 | 职责 | 包路径 |
|------|------|--------|
| CLI Adapter | spawn/pipe/parse 外部 CLI（claude/codex/gemini/opencode/kimi 五 adapter + unified 通道） | `internal/adapter/` |
| ACP Process Pool | F149 process pool keyed by (projectPath, providerProfile) + lease（acquire/release, TTL, LRU）+ 健康检查 + metrics | `internal/adapter/pool/` |
| Dynamic Router | 任务→agent 路由决策（扁平包 `internal/router` 已删除，运行时经 domains 适配） | `internal/domains/routing/` |
| A2A Hub | agent 间通信（扁平包保留，运行时经 domains 适配） | `internal/a2a/` |
| A2A Protocol Client | 受控的 A2A 协议客户端（§4.7）：按 `variant.client_id="a2a"` 路由，经 Google A2A Protocol `tasks/send` JSON-RPC over HTTPS 调用**外部**已部署 agent；协议类型复用 `pkg/a2a/`，外部地址经 `variant.a2a_url` 配置。与 CLI adapter 并列实现 `unified.AgentExecutor`，不引入 `internal/a2a/server|client/` | `internal/adapter/a2a/` `pkg/a2a/` |
| SOP Guardian | SOP 规则、门禁、review 策略 | `internal/sop/` |
| Identity & Event Bus | breed 类型定义已剥离至 `pkg/pack`；`internal/config` 现仅承载事件总线 | `pkg/pack/` `internal/config/event_bus.go` |
| Memory & Evidence | 证据/决策/经验存储；**落盘持久化**（重启保留 evidence/lessons/decisions，原子写 + 热加载） | `internal/memory/` |
| Relationship Capsule | 单 operator 关系画像：`profiles/<operator>/relationship/<relationshipKey>-primer.md` + provenance；身份「养熟」层，独立于 `dog-catalog.json` | `internal/settings`(ProfileRepository) |
| Continuity Bootstrap | 跨 spawn/会话续接：续接前任务 digest 注入，防长协作冷启动 | `internal/memory/` + `internal/threadstore/` |
| Memory Lanes | 6-organ typed memory lanes + delta producer + human disposition + consumption tracker + lifecycle trace + private initiative | `internal/memory/lanes.go` `internal/memory/supply.go` `internal/memory/feedback.go` `internal/memory/initiative.go` |
| Cue Plane | F287 recall opportunity catalog + cue envelope + lane resolver registry + consumption episode ledger + source invalidation (fail-closed) | `internal/cue/` |
| Skills Manifest | skill 加载/注入/管理：源扫描 + 持久化意图（`skills-config.json`，global 基线 + project 覆盖两级）+ 多 carrier 物理挂载（claude/codex/gemini/kimi 原生目录 per-skill 符号链接）+ 逻辑挂载（启用即注入 prompt）+ 安全层（内外源权限隔离 / 内容指纹 / 隔离门禁）+ sync 调谐 + 7 类漂移检测/解决（含配置层 config-new/config-orphan）+ HTTP API + 前端面板。详见 `docs/plans/skills-framework-governance-spec.md`（Phase 8） | `internal/skills/` |
| MCP Bridge | MCP server + client 桥接 | `internal/mcp/` |
| RAG Store | 向量存储、embedding、检索 | `internal/ragstore/` |
| Thread Store | 线程、会话、事件存储（扁平包保留，运行时经 domains/threads 适配） | `internal/threadstore/` |
| Settings Store | 成员、账户、系统配置；三文件落盘 + 热加载（见 §5.1） | `internal/settings/` |
| Prompt Hooks | hook 声明、注入管道、轨迹记录（见 governance/vision-compliance.md §8.4） | `internal/hooks/` |
| Eval Framework | harness eval 控制面、verdict 闭环、N-day 调度 | `internal/eval/` |
| Eval Domains | eval domain YAML 定义（5 个初始 domain） | `packs/default/evals/` |
| Ops Monitor | 运维监控、日志缓冲、健康状态 | `internal/ops/` |
| Telemetry | OpenTelemetry 可观测性：traces ring buffer + metrics + Prometheus exporter + 30s 快照 + HMAC 伪匿名化。Graceful degradation：init 失败不 crash | `internal/telemetry/` |

### 6.3 执行与基础设施

| 模块 | 职责 | 包路径 |
|------|------|--------|
| Transport | HTTP/WS handler、handoff 执行、context-transport 装配、eval/config handler | `internal/transport/` |
| Prompt Builder | prompt 组装、上下文保护（`ProtectRecentPairs`） | `internal/prompt/` |
| Agent Runtime | coder agent 执行器（**注：`internal/agent/skill_manager.go` 为死代码，Phase 8.1 废弃，skill 管理统一收归 `internal/skills/`**） | `internal/agent/` |
| Aspects | approval / command guard / tracing 切面 | `internal/aspect/` |
| Capabilities | agent_dispatch / command_check / context_assemble 等可组合能力 | `internal/capability/` |
| Components | embedding / model 组件封装（Eino component） | `internal/component/` |
| Tool Registry | fs/terminal 工具注册 | `internal/tool/` |
| Workspace | 工作区管理、PTY、sandbox | `internal/workspace/` |
| Infrastructure | config / http / websocket 基础设施 | `internal/infrastructure/` |
| Pack API | pack 资源 HTTP API | `internal/packapi/` |
| Platform Bootstrap | 平台装配与启动（breeds merge、catalog 加载） | `internal/platform/` |
| Shared | 共享错误定义 | `internal/shared/` |

### 6.4 演进记录

> 平台能力的历次扩展与缺口收口（运行时成员数据统一、编排账本、Persistent Identity 等）不在本文件展开，统一沉淀于 `docs/plans/` 与 `docs/designs/`。本文件只描述「平台是什么」，不记录「哪天改了什么」。
