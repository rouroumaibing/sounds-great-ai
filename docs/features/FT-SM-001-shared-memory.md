# [FT-SM-001] [Tech Story] 共享记忆（Shared Memory）特性梳理与设计故事

> 本文是一份**代码实读**的设计故事（story）：梳理 sounds-great-ai（SG）的 Shared Memory 特性，覆盖后端 `internal/memory/` + `internal/transport/` + `internal/prompt/` + `internal/capability/` 与前端的 `web/src/.../drawer/tabs/Memory*`、服务接线 `cmd/server/routes.go` / `internal/platform/platform.go`。
> 所有现状判断均来自实际代码，每条附 `文件:行号`，本文只描述 **SG 自身**。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [x] Tech Story（架构 / 核心能力）
- **责任人**: Dev: @memory | Reviewer: @platform
- **故事点/复杂度**: L（核心主链路、跨模块、含持久化与 prompt 注入）
- **一句话需求**: 让犬（dog agent）团队拥有"**确定性候选生产 → 人审批 → 召回注入 prompt**"的共享长期记忆，跨会话、跨犬复用经验与事实，且所有供给/推理**零 LLM**（VISION §3 红线）。
- **As a** 犬（执行 agent） / 平台（编排者） / 用户（审阅者），
  **I want** 一套可追溯、可审计、可降级的共享记忆层，
  **So that** 决策/偏好/事实在会话与犬之间不丢失、不被幻觉污染、且敏感信息按敏感度与清关等级受控可见。
- **关键指标/埋点**: recall 注入次数与采用率（recall ledger 7/14/30 天窗口）、cue 消费账本、lifecycle trace。

---

## 2. 特性全景：一段记忆的旅程（叙事）

SG 的 Shared Memory 不是单一模块，而是一条**确定性流水线**，由三段组成，类比一支犬团队的"笔记 → 长官批阅 → 上战场前复习"：

### 2.1 第一段 · 候选生产（确定性，零 LLM）
会话结束（session seal）时，`WSHandler.maybeAutoDistill`（`internal/transport/execution.go:100`）触发 fire-and-forget、fail-closed 的供给流程：`supply.Detect(sessionID, msgs)`（`execution.go:144`）用**确定性模式匹配**（非 LLM，`supply.go:17,37,46`）从会话消息中抽取决策/纠错/身份/偏好四类候选，经 `DetectAndSubmit` 落入 lane 的 **pending** 队列。此外 `repo_scanner.go:66 RunScan` 周期性扫描代码仓，把散落的决策/教训也补成候选（`repo_scanner.go:23 Scan`）。

> 红线印证：`supply.go:17` 注释明确"These are deterministic pattern matches, NOT LLM judgments (VISION §3)"。运行时：`maybeAutoDistill` 在 goroutine 中 `Detect → Produce → SubmitCandidates`（`execution.go:144-154`，defer recover fail-closed），候选真正落 pending。

### 2.2 第二段 · 人审批（disposition）
候选进入 pending 后，由人或长官在 UI 上处置。后端 `DispositionRecorder.Record`（`feedback.go:48`）支持 8 种动作：`accept/reject/modify/retire/forget/defer/undo/withdraw`（`feedback.go:14-21`）。`accept` 提升为 **approved**（该 lane 的 canonical truth，`lanes.go:105 Approve`，并自动退役重叠旧 truth `:106`）；`reject` 直接遗忘（`:121`）；`defer` 稍后（not-now，`:167`）；`withdraw` 重新打开（`:203`）。每条处置记录 `HumanDisposition`，可追溯。

### 2.3 第三段 · 召回注入 prompt（cue-plane）
每次犬 spawn，prompt 构建器 `builder.go:222 buildIdentity` 在身份段后调用 `laneCue.CueMemoryRanked(20, operator, hint)`（`builder.go:291`），按"机会分"从 approved+可见 truth 中挑出最相关片段拼入 system prompt；注入后立即 `RecordCueEvents(hits, operator)`（`:298`）写**消费账本**（fail-open）。这就是 cue-plane 的 SG 实现——记忆真正"进入犬的大脑"。

---

## 3. 后端架构（代码锚点）

### 3.1 数据与生命周期
- **7 条 typed lane**：`taste/profile/entity/person/event/decision/lesson`（`lanes.go:15-21`）——每条 lane 拥有自己的 canonical truth，仅 approved 才是 truth（`lanes.go:11,50`）。
- **状态机**：`pending → approved → retired/forgotten`，外加 `defer/withdraw` 中转（`lanes.go:28-31,105-222`）。
- **`LaneEntry`**（`lanes.go:36`）：含 `id/lane/content/source/status/operator_id/sensitivity/collection_id`。

### 3.2 关系图（edge / marker 一等公民）
- **10 种关系**：`evolved_from/blocked_by/supersedes/invalidates/related/related_to/promoted_from/wikilink/doc_link/feature_ref`（`lane_graph.go:20-29`）。
- **边级元数据**：`LaneEdge` 带 `edge_sensitivity/provenance/traversal_count/last_traversed_at/operator_id`（`lane_graph.go:47-55`）。`AddEdgeFull`（`lane_graph.go:220`）落库；`TouchEdge`（`lane_graph.go:264`）在被召回遍历时计数。
- **marker**：`LaneMarker`（`lane_graph.go:64`）记录 captured→normalized→approved/rejected。

### 3.3 敏感度 ACL + 清关（SG 独有正交轴）
- **4 级敏感度**：`public/internal/private/restricted`，`SensitivityRank` 排序（`lane_acl.go:9-20`）。
- **`EntryVisible(e, operator)`**（`lane_acl.go:110`）：双轴判定 = `collectionAllowed`（owner∨collection grant，`lane_acl.go:61`）+ `ClearanceFor(operator)`（清关等级，`lane_acl.go:91`）。
- **`ClearanceFor`**（`lane_acl.go:91`，空 operator=3 admin、具名默认 1）：SG 多犬共享一库，需要"即便 owner/成员、清关不够也看不到"的正交闸门。
- **放宽护栏**：`SetSensitivity`（`lanes_graph_handler.go:103`）放宽（rank 变小=更宽）而无 `confirm_visibility_widening` → 409 + `current/requested/confirm_field`（`lanes_graph_handler.go:119-126`），并在 lifecycle trace 留审计（`lanes_graph_handler.go:135-140`）。

### 3.4 向量 / 混合检索（纯 Go 仿真）
- `HybridSearch`（`lane_hybrid.go:163`）：RRF(k=60) 融合 **entry-NN + passage-NN + BM25 词法**（`lane_hybrid.go:57-59`），CJK 单字加权（`lexicalScores` `lane_hybrid.go:21`）。
- **passage 向量**：`StorePassages` 把 approved truth 切块各自嵌入（`lane_hybrid.go:106`，落 `lane_passage_vec` 表 `lane_vector.go:48`）。
- **embedMode**：`SetEmbedMode(off/shadow/on)`（`lane_hybrid.go:84`），经 `cmd/server/routes.go:208` 读 `SG_EMBED_MODE` 覆盖；无 embedder 时 `SemanticSearch` 降级 501、词法 FTS5 仍可用（`memory_embed.go:52-54`）。
- 纯 Go 仿真（不依赖 C 扩展），构建安全取舍。

### 3.5 召回账本 + 三轴语义
- `RecallEvent`（`recall.go:48`）+ `RecallStore.Record`（`recall.go:99`）：每次注入记一条。
- **三轴**：`beneficial/unmet/attention`（`recall.go:28-30`），由消费结果推导（used→beneficial，ignored→attention，unverified→unmet，`recall.go:130-136`）。
- **成熟度**：`measured/estimated/lower_bound/none`（`recall.go:36-39`），标注每个测量的可信度。
- `MarkOutcome`（`recall.go:155`，接受 `operator` 写 `operator_id`）+ `Ledger(windows)`（`recall.go:217`）产出 7/14/30 天窗口统计。
- **生命周期轨迹**：`RecordLifecycle`/`RecentLifecycle`（`recall.go:286/303`），append-only `lifecycle_trace` 表（`lane_migrations.go:108`）。

### 3.6 cue 消费账本
- `CueHit`（`memory_cue.go:58`）+ `CueMemoryRanked`（`memory_cue.go:93`）+ `RecordCueEvents`（`memory_cue.go:174`）追加写 `mem_cue_event`（`memory_cue.go:163`）。
- 后端 API `GET /api/memory/lanes/cue/events`（`lanes_handler.go:102`）。**前端不暴露**（用户决定），保留供审计。

### 3.7 多操作员显式归因
- `requestOperator`（`lanes_handler.go:117`：X-Operator > ?operator= > defaultOperator）用于读取；
- `explicitOperator`（`lanes_handler.go:132`：**不回退 default**，避免无作用域写入覆盖归属）用于写入（`SetSensitivity` `lanes_graph_handler.go:135` + `MarkOutcome` `lanes_recall_handler.go:82`）。
- 每条 entry/edge/recall 落 `operator_id`，逐记录可追溯。

### 3.8 受控 LLM 反省（合规，非记忆推理）
- `POST /api/memory/lanes/reflect` → `memory_reflect.go:62 Reflect`：仅在 approved truth 上摘要，产出 **pending** 候选，不自动成为 truth（VISION §3 合规：推理在 `internal/capability/`，**不碰 `internal/memory/`**）。

---

## 4. 持久化与迁移

- **存储**：`lane_persister.go`——SQLite 优先（pure-Go `modernc.org/sqlite`，WAL，`lane_persister.go:12,50`）+ JSON 兜底（`writeAtomic`，`:103`）。三个独立库：`lanes.json`(主) / `.graph.db`(边) / `.vec.db`(向量)。
- **迁移框架**：`allMigrations`（`lane_migrations.go:18`），当前 **v1–v6**：
  - v1 `lane_entry` 表；v2+ 加 `operator_id/sensitivity/collection_id`；
  - **v6**（P1）加 recall 三轴 `axis/maturity` 列 + `lifecycle_trace` 追加写表（`lane_migrations.go:94-111`）。
- `addColumnIfMissing` 对旧 `.graph.db` 做 best-effort 迁移（`lane_graph.go` 内）。

---

## 5. 服务接线（运行时）

- `platform.go:351 NewLaneRegistryAt(...)` 初始化 `SharedMemory`（`platform.go:97`）、`LaneDispositions`（`platform.go:103`）、`LaneRecall`（`platform.go:107`），挂到 Platform（`platform.go:491-494`）。
- `routes.go:199 NewLanesHandler(...)` → `mux.Handle("/api/memory/lanes", auth.Wrap(...))`（`routes.go:213-214`）。
- 可选 embedder：`routes.go:203 SetEmbedder(NewMemoryEmbed(emb))`；embedMode 由 `SG_EMBED_MODE` 覆盖（`routes.go:208`）。
- prompt 注入：`builder.go:121 SetLaneCue` + `:291 CueMemoryRanked` + `:298 RecordCueEvents`。
- 操作员下拉数据来自 `GET /api/people-memory/operators`（`people_memory_handler.go:131,189`）。

### 5.1 HTTP 端点清单（`lanes_handler.go:78-103`）
`GET pending/truth`、`GET/POST recall/events|ledger|{id}/outcome|pull`、`POST search|reflect`、`POST {id}/approve|reject|modify|retire|forget|defer|undo|withdraw|link|mark`、`GET {id}/graph|sensitivity`、`POST search/semantic|reindex`、`GET cue/events|lifecycle`。

---

## 6. 前端（代码锚点）

- **`MemoryTab.tsx`**：待审批/truth/召回记录/账本四区 + **全文搜索结果面板**（2026-08-23 接入：`searchLanes` 命中渲染为天蓝面板——类型徽章 + 内容截断 + 状态色点，Enter 触发/可关闭；此前调用后丢弃结果只报"0 结果"）；**操作员作用域下拉**（`:26-30`，`breedService.getBreeds()` 供给 +「默认(系统)」项 `:131-140`），随 `link/setSensitivity/markOutcome` 透传；召回账本显示三轴+成熟度（`:337,285`）。
- **`MemoryGapPanel.tsx`**：① **SVG 径向关系图**（`REL_COLOR` + `DIM` 隐藏 private/restricted 边，`:27,41,227-256`）；② 10 关系下拉 + 边敏感度 `<select>`（`:57-58`）；③ 敏感度放宽确认流（`widen` 状态 + `onConfirmWiden`，`:65`）；④ 生命周期轨迹自动加载（`:72,92`）；⑤ 全部写操作经 `operator` 透传（`:106,124,138`）。**cue 账本列表已移除**（用户决定）。
- **`memoryService.ts`**：`linkEntries/setSensitivity/markRecallOutcomeDetailed` 接 `operator` 并走 `?operator=`（`:94,112,139`）；`getLifecycle` 保留；`getCueEvents` 已删。
- **`types/api.ts`**：`RecallWindowStatApi`(三轴+maturity `:387-400`)、`LaneEdgeApi`(边级字段 `:443-450`)、`LANE_RELATIONS`(10 `:472`)、`SensitivityLevel`(4 `:469`)。
- **i18n**：`zh-CN.ts`/`en.ts` 双语文案，`lane.*/axis*/maturity*/edgeSensitivity*/widen*/operator*` 键齐全；删除未用 `modify/linkRelation/cueEvents*`。

---

## 7. 验收标准 (Acceptance Criteria)

- [ ] **AC-01（正常路径）**: Given 会话结束，When `maybeAutoDistill` 触发，Then 确定性候选落入 pending 且可从 `GET /api/memory/lanes/pending` 读到；人 approve 后 `GET /api/memory/lanes/truth` 可见，且下次 spawn 的 prompt 含该 truth（cue 注入）。
- [ ] **AC-02（异常/边界）**: Given 无 `SG_EMBED_API_KEY`，When `POST search/semantic`，Then 返回 501 且词法检索仍可用；Given 放宽敏感度无确认，When `POST {id}/sensitivity`，Then 返回 409 + `confirm_field`。
- [ ] **AC-03（权限/安全）**: 未授权访问 `/api/memory/lanes` 被 `auth.Wrap` 拦截；`restricted` 条目对非 admin 清关 operator 不可见（`EntryVisible` 双轴）；`explicitOperator` 写入不回退 default，归属不被覆盖。
- [ ] **AC-04（持久化）**: 重启后 lane/edge/vector/recall/lifecycle 均从 SQLite/JSON 恢复（v1–v6 迁移幂等）。

---

## 8. 工程护栏 (Guardrails)

- **[x] 红线合规（Security）**: `internal/memory/` **零 LLM**；所有供给/抽取为确定性模式匹配；LLM 反省仅在 `internal/capability/`。已 grep 验证（`supply.go:17,37` 注释 + 无 openai/anthropic 调用）。
- **[x] 文件规模（Maintainability）**: AGENTS.md **350 行硬限**——`internal/memory/` 全部 `.go` ≤ 342 行（最大 `recall.go:342`）；`lane_vector.go` 原 434 行已拆为 `lane_vector.go(164)` + `lane_hybrid.go(274)`。
- **[x] 降级/兜底（HA）**: embedder 缺失→语义检索 501、词法 FTS5 兜底（`memory_embed.go:52`）；persister SQLite 失败→JSON 兜底（`lane_persister.go:66`）；cue/lifecycle 写入 fail-open（`:298` 注释）。
- **[x] 无新顶层目录**: 全部落在既有 `internal/memory/`、`internal/transport/`、`internal/prompt/`、`internal/capability/`、`web/src/`。

---

## 9. 技术契约（节选）

- **召回账本响应**（`RecallWindowStatApi`）：`{ used, ignored, beneficial, unmet, attention, maturity: {measured, estimated, lower_bound, none} }`，按 `"7d"|"14d"|"30d"` 键（`api.ts:387-400`）。
- **边**（`LaneEdgeApi`）：`{ from_id, to_id, relation(10种), edge_sensitivity?, provenance?, traversal_count?, last_traversed_at?, operator_id? }`（`api.ts:443-450`）。
- **敏感度放宽 409**：`{ error, current, requested, confirm_field: "confirm_visibility_widening" }`（`lanes_graph_handler.go:120-125`）。

---

## 10. Story DoD Checklist

- [x] 代码实读覆盖前后端 + 服务接线（每条 file:line）。
- [x] 红线（零 LLM / 350 行 / 无新顶层目录）已 grep 验证。
- [x] 验证闸门：`go build/vet/test ./...` ✅、`web tsc -b` ✅、`vite build` ✅（延续前轮，本轮仅文档）。
- [ ] 监控告警：recall 注入 RT / 报错率尚未接监控（后续 Tech Story）。

---

## 11. 与其他文档的关系

- 深度分析（多操作员归因 / ClearanceFor / LLM 反省澄清）见 `docs/plans/` 下 shared-memory 相关计划文档。
- 接线路线图历史见 `docs/plans/shared-memory-wiring.md`（注：文中"dormant"状态已被后续接线补全——`execution.go:144` 已调 `supply.Detect`，`platform.go:351` 已建 `SharedMemory`）。

> 本文为代码实读梳理，未改动任何源码、未 `git commit`（延续多轮未提交）。
