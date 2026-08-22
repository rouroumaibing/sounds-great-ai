---
topics: [dossier, plugin, capability-profile, design, mcp]
doc_kind: design
---

# 能力画像插件（Capability Dossier Plugin）设计规格

> 自包含设计文档：定义一个可独立交付的"agent 能力画像"插件——数据格式、状态机、算法、存储、接口、分发形态与测试要求。任何宿主平台（单 agent CLI、chatbot 框架、multi-agent 编排系统）按本规格实现/接入后，其 agent 获得：证据化能力画像、路由参考、以及"真实任务事件 → 提案 → 人工审批 → 画像演化"的闭环。

---

## 1. 目标与非目标

**目标**
- 画像描述 **model 的认知能力**（擅长什么/易踩什么坑/什么任务该与不该派给用该 model 的 agent），agentId 只是索引：多 agent 共用一个 model 时画像互通。
- 画像随真实任务证据持续演化，每条结论可溯源（provenance），变更走 git（历史即审计日志）。
- 宿主无关：核心为纯函数，宿主耦合点全部隔离为适配器接口；存储可替换。

**非目标（规格级禁令）**
- 禁止性格问卷/量表测试与任何"人格总分"——被度量的对象会表演分数。
- 禁止算法路由/调度器——画像只提供判断依据（给数据不给结论），派单由 agent/人决定。
- 禁止把评测分数直接注入 agent 的 prompt——反馈必须经提案审批沉淀为叙事性画像。
- 禁止无证据的画像更新——证据引用为空时创建提案必须失败（fail-closed）。

## 2. 术语

| 术语 | 定义 |
|---|---|
| agent | 宿主平台中一个可被指派任务的执行者（CLI agent / bot / 人） |
| model | agent 背后的模型标识（如 claude-opus-4-6）；画像的真正主语 |
| operator | 人类管理员，唯一持有画像审批权 |
| 画像（profile） | 一个 agentId 的结构化能力描述（§3.2） |
| 观察（observation） | operator/同伴对某 agent 的一句话能力记录，只进暂存层，唯一用途是被提案引用为证据 |
| 机会（opportunity） | 一次任务/评审事件结束后产生的"可考虑沉淀画像"瞬态提醒 |
| 蒸馏提案（proposal） | 带证据的画像更新申请，经 operator 审批后由目标 agent 应用 |
| 三方分权 | 提案者 ≠ 审批者（operator）≠ 应用者（必须是目标 agent 本人） |

## 3. 数据规格

### 3.1 档案文件（唯一真相源）

- 位置：宿主仓库内 `dossier.md`（推荐 `docs/team/dossier.md`），**git 版本化**，不用数据库当主存储。
- 结构：每个 agentId 一节；节内含散文画像与一个机器可读 yaml 投影块。

**节头（apply 算法的锚点，格式固定）**：

```text
### {显示名} · @{mention} · `agent:{agentId}`
```

**yaml 投影块**：fenced ```yaml 块，首行标记 `# structured-profile: agent:{agentId}`：

```yaml
# structured-profile: agent:agent-001
entityId: "agent-001"                 # 必须与标记一致，否则该块解析失败（fail 整块）
oneLiner: "一句话画像（注入身份块）"
l0RosterSummary: "队友名册·擅长列（≤52 字符）"
l0RoutingNote: "队友名册·路由边界列（什么任务别派给它）"
routingSignals:
  peakCapabilities:                   # 该派给它的（1-4 条）
    - "复杂任务拆解与动态路由"
  antiSignals:                        # 不该派给它的（1-3 条）
    - "纯检索任务（派检索型 agent）"
provenance:
  version: "0.1"                      # 本画像版本
  date: "2026-08-22"                  # 最后更新日期
  primarySources:                     # 结论来源（配置基线/评审编号/观察编号…）
    - "baseline: 初始手写人设（待证据校准）"
```

**散文画像六字段**（yaml 块之外、节头之内，供按需全文阅读）：
① 原生峰值 ② 被低估能力 ③ 坏直觉（系统性弱点）④ 召唤反信号（何时不该找它）⑤ 互补 & 反模式（与谁配合好/差）⑥ 翻车熔断信号（出现即止损）。六字段强制含负面项（③④⑥），防止画像沦为自夸简历。

**解析规则**
- 只提取带 `# structured-profile:` 标记的 yaml 块；无标记块忽略。
- 字段语法限定：字符串字段 `field: "value"`（允许行尾注释）；列表字段支持多行 `- "item"` 与内联 `["a", "b"]` 两种形式；嵌套对象按缩进界定。实现不依赖通用 YAML 库（手写解析即够，减少依赖面）。
- `entityId` 与标记中的 agentId 不一致 → **该块整体判废**（防复制粘贴错位）。

### 3.2 核心对象 Schema

**DossierProfile**（解析产物）

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| agentId | string | ✓ | 身份键 |
| oneLiner | string | | 一句话画像（身份块注入用） |
| l0RosterSummary | string | | 名册擅长列（≤52 字符） |
| l0RoutingNote | string | | 名册路由边界列 |
| routingSignals.peakCapabilities | []string | | 该派给它的 |
| routingSignals.antiSignals | []string | | 不该派给它的 |
| provenance.version / date / primarySources | string / string / []string | | 溯源 |

**Observation**（暂存层，持久）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | string | `obs_` 前缀唯一 ID |
| agentId | string | 观察对象 |
| content | string | 一句话观察 |
| provenance.type | string | `operator` \| `peer` |
| provenance.author / date | string | 谁写的 / YYYY-MM-DD |
| createdAt | int64 | 毫秒时间戳 |

**Opportunity**（机会层，**瞬态、仅内存、重启可丢**——提醒不是账本）

| 字段 | 类型 | 说明 |
|---|---|---|
| opportunityId | string | `opp-N` 自增 |
| sourceEvent | string | 事件类型（§5 白名单） |
| sourceId | string | 幂等键（§5 构成规则） |
| targetAgentId | string | 提醒对象（被评审/完成任务的一方） |
| threadRef / reviewerAgentId / authorAgentId | string | 事件上下文 |
| status | string | `pending` \| `converted` \| `dismissed` |
| convertedToProposalId | string | status=converted 时填写 |

**Proposal**（账本，持久）

| 字段 | 类型 | 校验 |
|---|---|---|
| proposalId | string | `dsp_` 前缀唯一 ID |
| status | string | §3.3 状态机 |
| sourceEvent | string | ∈ 白名单（§5） |
| sourceId | string | 非空；**全局唯一**（幂等键） |
| targetAgentId | string | 非空 |
| targetFields | []string | 非空；被更新的画像字段名 |
| beforeSnapshot | string | 非空；当前档案中被替换的原文（逐字节匹配） |
| afterDraft | string | 非空；替换后文本 |
| rationale | string | 非空；审批卡展示的理由 |
| evidenceRefs | []{type,id,summary?} | **非空 fail-closed**；type ∈ {observation, review, trajectory, operator-comment}；无重复 (type,id) |
| baseHash | string | 64 位 hex；创建时档案文件的 SHA-256 |
| createdBy / createdAt | | 提案者与时间 |
| approvedBy / approvedAt / rejectedBy / rejectedAt / rejectReason / appliedBy / appliedAt / appliedCommitSha | | 状态机审计字段 |

### 3.3 提案状态机

```text
pending ──approve(operator)──> approved ──execute-apply(target agent)──> applied
   │
   └──reject(operator)──> rejected（终态，带 reason）
```

| 迁移 | 调用者 | 前置校验 | 语义 |
|---|---|---|---|
| pending→approved | operator | `approver ≠ createdBy`（否则 403） | CAS：仅当当前 status=pending |
| pending→rejected | operator | `rejecter ≠ createdBy`（否则 403） | 同上 |
| approved→applied | **必须 actor == targetAgentId**（否则 403） | §4.3 全部通过 | CAS：仅当当前 status=approved |

所有迁移用 CAS（`UPDATE … WHERE id=? AND status=<期望值>`，affected=0 视为并发冲突返回 409）。`rejected`/`applied` 为终态。

## 4. 核心算法规格

### 4.1 解析与缓存

- `parseDossierProfiles(markdown) → map[agentId]DossierProfile`：纯函数，按 §3.1 规则。
- `Loader`：进程级缓存，key 为档案路径；**文件不存在不缓存**（文件可能稍后出现，如 git pull / 首次 apply）；apply 成功后显式失效；提供 `IsAvailable`（区分"无档案=社区回退"与"有档案读不出=漂移告警"两种状态）。

### 4.2 哈希（乐观锁）

`baseHash = SHA-256(档案文件全文)`，创建提案时计算，apply 时重算比对；不一致 → `BASE_HASH_MISMATCH`，提案作废需重新基于新基线提案。

### 4.3 apply：段落锚定替换（纯函数 + 两阶段提交）

`prepareDraft(proposal, fileContent) → {modifiedContent, commitMessage}`，**无任何 I/O**：

```text
1. status == approved                       否则 NOT_APPROVED
2. sha256(fileContent) == baseHash          否则 BASE_HASH_MISMATCH
3. 定位节头：正则 (?m)^###\s+.*`agent:{targetAgentId}`
   找不到 → SECTION_NOT_FOUND（fail-closed，禁止退化为全文件搜索）
4. 节边界 = 节头行之后到下一个 ^###\s 行（或 EOF）
5. 在节边界内 indexOf(beforeSnapshot)
   找不到 → BEFORE_SNAPSHOT_NOT_FOUND（防跨节误伤：文案只存在于别节时拒绝）
6. 仅替换节内第一处出现，拼接 modifiedContent
7. 生成结构化 commitMessage：
   "docs(dossier): apply distillation to {agentId} [{targetFields}]
    Proposal: {proposalId} / Source: {sourceEvent} ({sourceId})
    Rationale: {rationale} / Approved by: {approvedBy} / Applied by: {appliedBy}"
```

`executeApply(proposalId, actor)` 编排（服务层）：

```text
读档案原文 → prepareDraft → 写文件
→ git add <档案> && git commit          # 阶段一：失败则回滚（恢复原文 + git reset HEAD -- 档案）
→ MarkApplied（CAS）                    # 阶段二：与 commit 竞态时以 commit 为准回报
→ Loader 缓存失效
git 只 add+commit，不 push —— 推送由宿主走正常评审流程
```

## 5. 事件模型

**sourceEvent 白名单**（可扩展，初始四类）：`task-complete`、`review-complete`、`phase-close`、`manual`（冷启动用，operator 手工触发首次校准）。

**触发接口**（宿主在相应生命周期点调用，best-effort、永不阻塞业务主流程、panic 必须被 recover）：

```go
OnTaskComplete(agentID, taskRef string)
OnReviewComplete(reviewerID, authorID, ref string)   // 提醒对象 = 被评审方 authorID
OnPhaseClose(agentID, phaseRef string)
```

**幂等键构成**：`{sourceEvent}:{上下文引用}:{参与者}`，如 `review-complete:{threadId}:{sha}:{reviewerId}`；同 sourceId 只产生一条机会；并发去重用 in-flight 表。

## 6. 存储规格

**SQLite（默认后端；可换 Redis/文件，只要满足语义）**：

```sql
CREATE TABLE IF NOT EXISTS dossier_observations (
  id TEXT PRIMARY KEY, agent_id TEXT NOT NULL, content TEXT NOT NULL,
  provenance_type TEXT NOT NULL, provenance_author TEXT NOT NULL,
  provenance_date TEXT NOT NULL, created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_dossier_obs_agent ON dossier_observations(agent_id, created_at);

CREATE TABLE IF NOT EXISTS dossier_distillation_proposals (
  proposal_id TEXT PRIMARY KEY, status TEXT NOT NULL,
  source_event TEXT NOT NULL, source_id TEXT NOT NULL,
  target_agent_id TEXT NOT NULL, target_fields TEXT NOT NULL,   -- JSON array
  before_snapshot TEXT NOT NULL, after_draft TEXT NOT NULL,
  rationale TEXT NOT NULL, evidence_refs TEXT NOT NULL,          -- JSON array
  base_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL, created_by TEXT NOT NULL,
  approved_by TEXT, approved_at INTEGER,
  rejected_by TEXT, rejected_at INTEGER, reject_reason TEXT,
  applied_by TEXT, applied_at INTEGER, applied_commit_sha TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dossier_proposals_source ON dossier_distillation_proposals(source_id);
CREATE INDEX IF NOT EXISTS idx_dossier_proposals_status ON dossier_distillation_proposals(status, created_at);
```

语义约束：观察与提案**永不清理**（TTL=∞）；机会层**只允许内存实现**（重启清零是设计行为——提醒可丢，账本不可丢）；存储不可用时观察/提案 API 整体拒绝注册（fail-closed），机会 API 不受影响。

## 7. 宿主适配器（宿主必须实现的 5 个接口）

| 接口 | 方法签名 | 语义 |
|---|---|---|
| EventSource | §5 三个 On* 回调的挂接点 | 把宿主已有的任务完成/评审/阶段关闭事件映射进插件 |
| IdentityProvider | `ActorID(ctx) string`；`IsOperator(actor) bool` | 解析当前调用者（agent 或人）；审批权判定 |
| ModelRegistry | `ModelOf(agentID) string` | 支撑"画像属于 model"语义与分组展示 |
| PromptProjector | `RosterRow(agentID) string`；`RoutingNote(agentID) string` | 注入宿主 prompt。fallback 链固定：`dossier.l0RosterSummary ?? 宿主config.强项 ?? 宿主config.职责描述`；路由边界 `dossier.l0RoutingNote ?? 宿主config.告示`；宿主硬限制**永远保留**（config 是 enforcement，档案只是 advice）。若宿主有多处 prompt 组装点，必须全部接同一链（防两处说不一致的话） |
| ApprovalSurface | 列 pending + approve/reject 两个动作 | 最低可用形态是 CLI；web/IM 卡片皆可 |

## 8. 对外 API 契约

### 8.1 REST（前缀 `/api/dossier`；actor 识别：body `actor` > 请求头 `X-Dossier-Actor` > 默认 operator）

| 端点 | 请求 | 成功 | 失败语义 |
|---|---|---|---|
| GET `/` | — | 200：`{modelGroups:[{model, agents:[{agentId, displayName, channel, profile|null}]}], meta:{totalAgents,totalModels,coverage,available}}` | — |
| GET `/base-hash` | — | 200：`{baseHash}` | — |
| POST `/observations` | `{agentId, content, actor?}` | 201 `{observation}` | 400 缺字段 |
| GET `/observations?agentId=&limit=` | — | 200 `{observations:[]}`（空为 `[]` 非 null） | — |
| GET `/opportunities` | — | 200 `{opportunities:[]}`；operator 全量，agent 只见指向自己的 | — |
| POST `/opportunities/{id}/dismiss` | — | 200 | 404 已处理/不存在 |
| POST `/opportunities/{id}/convert` | `{proposalId}` | 200 | 400/404 |
| POST `/proposals` | §3.2 Proposal 字段 + `actor` | **201 新建 / 200 幂等命中**（同 sourceId 返回已存在） | 400 校验失败（含 evidenceRefs 空） |
| GET `/proposals?agentId=&limit=` | — | 200 `{proposals:[]}`；无 agentId 时只回 pending | — |
| GET `/proposals/{id}` | — | 200 | 404 |
| POST `/proposals/{id}/approve` | `{actor?}` | 200 | 403 自批；409 状态不符 |
| POST `/proposals/{id}/reject` | `{actor?, reason}` | 200 | 403 自否；409 |
| POST `/proposals/{id}/execute-apply` | `{actor}` | 200 `{proposal, commitSha}` | 403 非目标 agent；409 baseHash/stale；422 锚定失败 |

**错误码表**：`VALIDATION`（400）/ `SEPARATION_OF_DUTIES`（403）/ `NOT_TARGET_AGENT`（403）/ `NOT_FOUND`（404）/ `STATE_CONFLICT`（409）/ `BASE_HASH_MISMATCH`（409，附 currentHash）/ `NOT_APPROVED`·`SECTION_NOT_FOUND`·`BEFORE_SNAPSHOT_NOT_FOUND`（422）。

### 8.2 MCP 工具集（agent 零改造接入面，REST 的同语义映射）

| 工具 | 输入 | 输出 |
|---|---|---|
| `dossier_get_profiles` | — | 模型分组画像 + 覆盖率 |
| `dossier_get_base_hash` | — | 当前档案哈希（提案必填） |
| `dossier_list_opportunities` | `actor`（可选，作用域过滤） | 指向自己的 pending 机会 |
| `dossier_propose_distillation` | §3.2 Proposal 全字段 | 提案（幂等） |
| `dossier_apply` | `proposalId`, `actor` | apply 结果 + commitSha |

## 9. 分发形态（共享同一 Core 与档案格式——格式即协议）

1. **in-process 库**：宿主 import Core + 自选存储适配。
2. **sidecar HTTP 服务**：独立进程，宿主仅 HTTP 客户端。
3. **独立 MCP server**：stdio / Streamable HTTP，任何支持 MCP 的 agent 直接调工具；档案放宿主仓库，server 持文件与 git 权限。

## 10. 冷启动与初始化

1. 初始化 = 空档案 + 每个存量 agent 一节"baseline 手写人设"（源自宿主现有配置的角色描述），provenance 标 `baseline: 待证据校准`。
2. **新 agent 试用期**：不考试——标注"画像待校准"，靠真实任务证据逐步校准；`manual` sourceEvent 供 operator 手工触发首次校准。
3. **fallback 链保证无档案不断链**：无档案的 agent 静默回退宿主 config；**内置 agent 有档案条目但缺投影字段 → 告警**（漂移信号），运行时动态 agent 静默回退。
4. 示例档案须标 `demo`，不作为他人团队默认画像。
5. 演化三态：baseline（初始）→ accumulated（按领域累积）→ evolving（事件回流持续刷新）。

## 11. 宿主接入步骤（普通 Agent 平台的最小路径）

1. 选分发形态（§9）；2. 建档案文件并初始化 baseline（§10）；3. 实现 5 个适配器（§7，最小实现约一天）；4. 选存储后端（§6）；5. agent prompt 加一行指引（"复杂协作前调 dossier_get_profiles；收到机会自行判断是否提案"）+ 注册 MCP 工具；6.（可选）审批面用 CLI 起步。

## 12. 测试要求（安全属性必须全覆盖）

| 测试 | 断言 |
|---|---|
| 解析 fail-closed | entityId 与标记不一致 → 整块丢弃；无标记块忽略 |
| 跨节锚定 | beforeSnapshot 文案只存在于**另一 agent 节**时 → BEFORE_SNAPSHOT_NOT_FOUND，文件不动 |
| 乐观锁 | 提案后档案被并发修改 → apply 409 且文件保持修改后状态 |
| 无锚拒绝 | 目标节头不存在 → SECTION_NOT_FOUND，不退化为全文件替换 |
| 三方分权 | 自批/自否 403；非目标 agent apply 403 |
| 证据 fail-closed | evidenceRefs 空/类型非法/重复 → 创建 400 |
| 幂等 | 同 sourceId 二次创建返回 200 + 同一提案 ID；同 sourceId 事件只产生一条机会 |
| commit 回滚 | git commit 失败 → 文件恢复原文 + index 清理，重试不撞 baseHash |
| 空列表语义 | 所有列表接口空结果返回 `[]` 而非 `null` |
| 瞬态/持久分层 | 重启后机会清零（by design），观察/提案完整保留 |
