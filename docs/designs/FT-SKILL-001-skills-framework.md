# [FT-SKILL-001] [Tech Story] 梳理并固化 Skills Framework 前后端逻辑

> 本文档基于 `sounds-great-ai` 真实源码核查（**2026-08-19 生成，反映截至本日的代码真实状态**）编写，目标是把 "Skills Framework（技能框架）" 这一治理化子系统（含扫描/持久化、多 carrier 物理挂载 + 逻辑注入、漂移检测与解决、安全/权限状态机、生产注入链路、d11 触发 resolver、REST API、前端管理面板）的**前后端协作逻辑**固化为单一可信来源（single source of truth），供后续开发、review 与新人 onboarding 使用。
>
> 关联：**对比文档** `docs/plans/skills-framework-sg-vs-clowder-2026-08-18.md`（前后端逐项对比，含外部参考）；**平台接线** `internal/platform/platform.go`；**提示构建** `internal/prompt/builder.go`。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu | QA: @demu
- **故事点/复杂度**: [ L (8分) ] —— 核心治理子系统，跨前后端 + 多包（skills / prompt / hooks / transport / platform）
- **业务/技术目标**:
  - As a **用户(Operator)/犬队成员**,
  - I want to **在一个面板里启用/禁用技能、按 carrier 挂载、检查并解决挂载漂移、查看并管理技能的安全/权限状态，且这些技能能在对话中按触发词或显式 `skill:` 指令被自动注入到对应 carrier 的 system prompt**,
  - So that **技能成为"文件即配置 + 持久化意图 + 漂移自愈 + 安全隔离"的可治理资产，而非散落在各 CLI 配置目录的手工符号链接**.
- **关键指标/埋点**: 无前端埋点；可观测性来自 `GET /api/skills`（含 `mountHealth`/`security`）、`GET /api/skills/security` 与 Ops 面板。

### 1.1 一句话定位

SG 的 Skills Framework 是一个**治理化子系统**：从 `packs/default/skills/*/SKILL.md` 扫描技能定义，把"启用/范围/挂载点"持久化到 `<ConfigRoot>/skills-config.json`，对 `claude/codex/gemini/kimi` 走 **per-skill 物理符号链接**、对其余 carrier（如 `opencode`）走 **Prompt Builder 逻辑注入**；后端有 **8 个 REST 端点 + 同步调谐器 + 7 类漂移检测 + 安全状态机**，前端有完整「技能管理」面板（含安全态可见性与批准/隔离动作），生产流按 carrier 注入 `SkillIDs` 正文，d11 动态 resolver 命中查询注入 `SKILL_NAME/SKILL_TRIGGER/SKILL_ID`。

### 1.2 端到端 Skills 逻辑总览（前后端主线）

```
┌──────────────────────────────────────────────────────────────────────────┐
│ 前端 web/src/components/settings/SkillsPanel.tsx                           │
│  load() ─ apiGet('/api/skills')                                            │
│  toggleEnabled ─ apiPatch('/api/skills/{id}', {enabled,mountPoints})       │
│  toggleCarrier  ─ apiPatch('/api/skills/{id}', {mountPoints})              │
│  checkDrift    ─ apiPost('/api/skills/drift/check')                        │
│  resolveDrift  ─ apiPost('/api/skills/drift/resolve', {strategy})          │
│  syncMounts    ─ apiPost('/api/skills/sync')                               │
│  openPreview   ─ apiGet('/api/skills/{id}')  → 安全批准/隔离               │
│  securityAction─ apiPost('/api/skills/security/{id}/{approve|quarantine|revoke}') │
└─────────────────────────────────────│──────────────────────────────────────┘
                                       ▼  HTTP (auth.Wrap)
┌──────────────────────────────────────────────────────────────────────────┐
│ 后端 internal/transport/skills_handler.go                                   │
│  Routes(): GET /api/skills · GET /api/skills/{id} · PATCH /api/skills/{id} │
│           POST /api/skills/sync · POST /api/skills/drift/check|resolve     │
│           GET /api/skills/security · POST /api/skills/security/{id}/{action}│
│   skillListItem / skillDetail（含 mountHealth + security 字段）            │
│        └─▶ internal/skills.SkillManager                                     │
│             Scan() ──▶ reconcileSecurity()（状态机 + 指纹）                 │
│             MergedIntents() / AllEnabled() / EnabledForCarrier()            │
│             SyncSkillMounts()（物理 symlink 调谐）                          │
│             DetectSkillDrift() / ResolveSkillDrift()（7 类漂移）            │
│             securityBlocks() / Approve|Quarantine|Revoke                    │
└──────────────────────────────────────────────────────────────────────────┘
                  │                                    │
   平台接线        ▼                                    ▼ 生产注入
┌──────────────────────────────────┐   ┌────────────────────────────────────────┐
│ internal/platform/platform.go     │   │ internal/transport/execution.go        │
│  NewManagerWithConfig(home,proj,  │   │  enabledSkillIDs(carrier)              │
│    {SkillsDir:"packs"})           │   │   └─ mgr.EnabledForCarrier + security  │
│  Config().Load() + Scan()         │   │  PromptBuilder.Build(BuildRequest{     │
│  Config().Watch(3s poll+30s deb)  │   │    SkillIDs})                          │
│   └─ ReloadAll()（热加载两层）     │   │   ├ buildSkillRoster()（始终注入）      │
│                                  │   │   └ buildSkills()（仅 SkillIDs 非空）   │
└──────────────────────────────────┘   └────────────────────────────────────────┘
                  │
   触发命名        ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ internal/hooks/resolver.go  SkillTriggerResolver.Resolve(Query, Carrier)   │
│   extractSkillTag("skill:<id>") 优先 → mountedToCarrier 过滤                │
│   否则子串宽松匹配 triggers/name → 注入 SKILL_NAME/SKILL_TRIGGER/SKILL_ID   │
└──────────────────────────────────────────────────────────────────────────┘
```

### 1.3 前后端职责边界（关键事实）

| 维度 | 前端(React+TS) | 后端(Go) |
|------|----------------|----------|
| 技能列表/启停/挂载点 | ✅ 面板发起 `GET`/`PATCH` | ✅ `SkillManager` 持久化意图 + 调谐 |
| 漂移治理 | ✅ 检查/解决按钮 + 策略切换 + 横幅展示 | ✅ `DetectSkillDrift`/`ResolveSkillDrift` |
| 同步挂载 | ✅ "同步挂载"按钮 | ✅ `SyncSkillMounts`（per-skill symlink） |
| 安全/权限 | ✅ 预览弹窗显示安全徽标 + 批准/隔离按钮 | ✅ `reconcileSecurity`/`securityBlocks`/`Approve`/`Quarantine`/`Revoke` |
| 生产注入 | ❌ 不拼 prompt | ✅ `enabledSkillIDs` → `PromptBuilder.Build` |
| 触发命名 | ❌ | ✅ d11 `SkillTriggerResolver`（显式 `skill:` tag + carrier 过滤） |
| 配置热加载 | ❌ | ✅ `SkillConfigStore.Watch`（3s 轮询 + 30s 防抖 → `ReloadAll`） |

---

## 2. 验收标准 (Acceptance Criteria - AC)

> 以下 AC 描述 "Skills Framework 按当前实现应当表现的行为"，可作为回归用例与 review 基线。

- [x] **AC-01 (正常路径-列表加载)**: Given 打开设置→技能页, When `load()` 调 `GET /api/skills`, Then 返回 `SkillItem[]`（含 `mountHealth`/`security`），面板渲染卡片（名称/id/分类/挂载健康徽标/安全徽标/启用开关/触发 chips/挂载按钮）。
- [x] **AC-02 (正常路径-启停)**: Given 点击某技能「启用中/已禁用」, When `toggleEnabled` 发 `PATCH /api/skills/{id}` `{enabled, mountPoints}`, Then 后端 `SetEnabled` 持久化到 `skills-config.json` 并 best-effort `SyncSkillMounts`；列表该项切换状态。
- [x] **AC-03 (正常路径-挂载点切换)**: Given 点击某 carrier 挂载按钮, When `toggleCarrier` 发 `PATCH /api/skills/{id}` `{mountPoints}`, Then `SetMountPoints` 更新意图；`mountPoints` 为空表示挂全部 carrier（默认）。
- [x] **AC-04 (正常路径-漂移检查)**: Given 点击「检查漂移」, When `POST /api/skills/drift/check`, Then `DetectSkillDrift` 返回 7 类中命中的 `DriftIssue[]`（前端按 `DRIFT_LABEL` 渲染中/英标签 + carrier + 详情）。
- [x] **AC-05 (正常路径-漂移解决)**: Given 存在漂移且点击「一键解决」, When `POST /api/skills/drift/resolve` `{strategy:'keep-project'|'use-global'}`, Then `ResolveSkillDrift` 先备份 `conflict` 占用到 `<ConfigRoot>/.drift-backup/<carrier>-<skillId>`，再 `SyncSkillMounts` 调谐；策略决定原生目录 scope。
- [x] **AC-06 (正常路径-同步挂载)**: Given 点击「同步挂载」, When `POST /api/skills/sync`, Then `SyncSkillMounts` 对 `claude/codex/gemini/kimi` 建/清 per-skill 符号链接（不目录级链接），逻辑挂载 carrier 无磁盘操作。
- [x] **AC-07 (正常路径-预览 + 安全动作)**: Given 点击卡片打开预览弹窗, When `GET /api/skills/{id}` 返回 `SkillDetail`（含 `content`/`path`/`security`）, Then 弹窗显示正文 + 安全状态徽标；点击「批准/隔离」发 `POST /api/skills/security/{id}/{action}`，状态刷新。
- [x] **AC-08 (权限与安全-状态机)**: Given 扫描到技能, When `reconcileSecurity` 分类, Then 内部可信源（`source=='packs'`）默认 `approved`；外部源（`user`/`plugin`）首次为 `pending`（注入前需人工批准）。
- [x] **AC-09 (权限与安全-指纹失配)**: Given 外部源技能正文变更, When 重扫指纹 `skillFingerprint(body)` 与已存不符, Then 该技能 `Status` 强制 `quarantined`（防上游替换下毒）。
- [x] **AC-10 (生产注入链路)**: Given 某 carrier 下技能已启用且未被安全阻断, When `execution.enabledSkillIDs(carrier)` → `PromptBuilder.Build({SkillIDs})`, Then `buildSkillRoster` **始终**注入全部技能清单（不依赖 SkillIDs），`buildSkills` **仅当** `SkillIDs` 非空才注入正文。
- [x] **AC-11 (触发命名-resolver)**: Given 用户查询含 `skill:<id>` 显式指令, When `SkillTriggerResolver.Resolve` 命中, Then 返回精确 `SKILL_NAME/SKILL_ID`（不进入子串宽松匹配）；且仅匹配 `mountedToCarrier` 当前 carrier 的技能（避免命名与挂载范围错位）。
- [x] **AC-12 (配置热加载)**: Given 外部进程编辑 `skills-config.json`, When `Watch`（3s 轮询 mtime + 30s 防抖）触发, Then `ReloadAll` 重载 global+project 两层并重扫源，内存态自动刷新（无需重启 server）。
- [x] **AC-13 (异常与边界-注入被阻断)**: Given 技能处于 `pending`/`quarantined`/`revoked`, When 生产流 `enabledSkillIDs`/`Resolve`/`AllEnabled` 经 `securityBlocks` 过滤, Then 该技能不进入注入集合（已启用但被安全态阻断）。
- [x] **AC-14 (权限与安全-接口鉴权)**: Given 未登录访问 `/api/skills*`, When 无有效 auth, Then 返回 401/403（`auth.Wrap` 包裹）。

---

## 3. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- [x] **资损与网络安全 (Security)**
  - 安全状态机: ✅ `SkillSecurityStore`（pending→approved/quarantined/revoked）+ 内容 sha256 指纹；内外源隔离（`packs` 可信默认 approved，外部源 pending/指纹失配自动隔离）。
  - 防越权注入: ✅ `securityBlocks` 在 `AllEnabled`/`EnabledForCarrier`/`Resolve` 三处统一过滤；持久化原子写（临时文件 + rename，权限 0644）。
  - 接口鉴权: ✅ `skills_handler.go` 经 `auth.Wrap` 注册（`cmd/server/routes.go`）。
- [x] **高并发与限流降级 (High Availability)**
  - Peak QPS: 配置/查询类 REST，默认普通；非核心主链路。
  - 降级/兜底: ✅ 安全态缺失（nil）视为 `approved`（与内部可信源默认一致），不阻断主链路；`SyncSkillMounts` 失败返回 error 由调用方决策，不静默。
  - 动态开关: ❌ 未用 Feature Flag；技能开关以 `skills-config.json` 的 `enabled` 字段驱动。
- [x] **可服务性与监控 (Serviceability)**
  - 核心日志与错误码: ✅ `Watch` 热加载失败经 `log.Printf("Warning: ...")`；漂移/同步以明确 `detail` 文案返回前端。
  - 监控告警: ⚠️ 依赖 Ops 端点人工观察；未配置自动 RT/错误率告警阈值（待补）。

---

## 4. 技术契约与接口设计 (Technical Contract)

### 4.1 数据模型

| 结构 | 字段 | 位置 |
|------|------|------|
| `Skill`（运行时） | `ID,Name,Description,Triggers,RequiresMcp,Category,Source,Body,FilePath` | `internal/skills/skill.go:13` |
| `SkillIntent`（持久化意图） | `enabled,scope('global'|'project'),mountPoints[],source,pluginId` | `internal/skills/config.go:11` |
| `SkillConfig`/`SkillConfigStore` | 意图集合 + 原子 `Save()` + `Watch()` | `config.go:27,35,83,178` |
| `SkillSecurityState` | `ID,Source,Trusted,Fingerprint,Status,ReviewedBy` | `internal/skills/security.go:24` |

`ParseSkill`（`skill.go:42`）解析 frontmatter（id/name/description/category/triggers/requires_mcp）+ 正文 `Body`；`Source` 由加载目录决定（`packs`/`user`/`plugin`）。

### 4.2 持久化与加载 / 热加载

- 配置文件：`<home>/.sounds-great-ai/skills-config.json`（global）与 `<workspace>/.sounds-great-ai/skills-config.json`（project）两级；`NewManagerWithConfig(homeCfg, projCfg, {SkillsDir:"packs"})`（`platform.go:318`）。
- 原子写：`SkillConfigStore.Save()`（`config.go:83`，临时文件 + rename，0644）。
- 热加载：`SkillConfigStore.Watch(onReload)`（`config.go:178`，3s 轮询 mtime + 30s 防抖）→ `ReloadAll()`（`platform.go:323`）。stop 守卫保证幂等。

### 4.3 挂载模型（物理 + 逻辑并存）

- 原生目录约定：`nativeSkillsDirConvention`（`reconciler.go:21`）：`claude→.claude/skills`、`codex→.codex/skills`、`gemini→.gemini/skills`、`kimi→.kimi/skills`。
- 落点：`carrierNativeSkillsDir(carrier, scope, ws, home)`（`reconciler.go:36`）：global→home、project→workspace。
- 调谐：`SyncSkillMounts`（`reconciler.go:131`）：算 desired（per-skill 链接，不目录级）→ 清 stale 托管 symlink（仅 `isManagedTarget`）→ 建 desired → 写回基线 hash。
- 逻辑挂载：无原生目录的 carrier（如 `opencode`）仅由 `SkillIDs` 在生产流注入，无磁盘操作。

### 4.4 漂移检测与解决（7 类）

| 类型 | 含义 | 位置 |
|------|------|------|
| `unregistered` | 源有技能但 config 未启用 | `drift.go:12,49` |
| `phantom` | config 启用但源已删 | `drift.go:13,95` |
| `conflict` | carrier 目录存在非 SG 管理的同名占用 | `drift.go:14,75` |
| `mount-missing` | enabled+carrier 但 symlink 缺失 | `drift.go:15,65` |
| `stale-mount` | symlink 存在但 config 已禁用/移除 | `drift.go:16,107` |
| `config-new` | 全局启用但项目层未收到（级联缺口） | `drift.go:17,140` |
| `config-orphan` | 项目层引用源已删技能（幽灵启用） | `drift.go:18,131` |

`DetectSkillDrift(opts)`（`drift.go:37`）；`ResolveSkillDrift(opts, strategy)`（`drift.go:162`）：先 `backupConflict` 到 `.drift-backup` 再 `SyncSkillMounts`；`resolveScope`（`drift.go:178`）按 `use-global`/`keep-project` 选原生目录 scope。

### 4.5 安全/权限层

| 组件 | 职责 | 位置 |
|------|------|------|
| `SkillSecurityStore` | 状态机 + 原子读写 `skill-security.json` | `security.go:35` |
| `reconcileSecurity` | 分类来源、维护状态（可信→approved；外部→pending；指纹失配→quarantined） | `manager.go:136` |
| `securityBlocks` | pending/quarantined/revoked 阻断注入 | `manager.go:179` |
| `ApproveSkill`/`QuarantineSkill`/`RevokeSkill` | 人工动作（带 `by`） | `manager.go:375/393/409` |

### 4.6 生产注入链路

- `execution.go:enabledSkillIDs(carrier)`（`664`）→ `mgr.EnabledForCarrier(carrier)`（`manager.go:336`，经 `effectiveEnabled` + `securityBlocks` 过滤）→ `PromptBuilder.Build(BuildRequest{SkillIDs})`（`execution.go:213,215` / `763,765`）。
- `internal/prompt/builder.go`：`BuildRequest.SkillIDs`（`129`）；`buildSkillRoster()`（`194`，**始终**注入全部技能清单，`173`）；`buildSkills(ids)`（`361`，**仅当** `len(SkillIDs)>0` 注入正文，`177`）。

### 4.7 d11 触发 resolver

- `SkillTriggerResolver.Resolve(input)`（`resolver.go:81`）：先 `extractSkillTag(q)`（`130`）取显式 `skill:<id>`（精确 `ID` 或大小写不敏感 `Name` 匹配，命中即返回）；否则对 `AllEnabled()` 做 triggers/name 子串宽松匹配。
- `mountedToCarrier(mgr, id, carrier)`（`resolver.go:155`）：仅匹配挂载到当前 carrier 的技能（G5 修复命名与挂载范围错位），显式 tag 路径（`:93`）与子串路径（`:108`）均过滤。

### 4.8 HTTP API（8 端点）

| 端点 | 方法 | 用途 | 前端调用 |
|------|------|------|----------|
| `/api/skills` | GET | 列表（含 `mountHealth`/`security`） | `SkillsPanel.load` |
| `/api/skills/{id}` | GET | 详情（含 `content`/`path`） | `openPreview` |
| `/api/skills/{id}` | PATCH | 启用/范围/挂载点（写后 best-effort sync） | `toggleEnabled`/`toggleCarrier` |
| `/api/skills/sync` | POST | 同步挂载 | `syncMounts` |
| `/api/skills/drift/check` | POST | 检测漂移 | `checkDrift` |
| `/api/skills/drift/resolve` | POST | 解决漂移（带 `strategy`） | `resolveDrift` |
| `/api/skills/security` | GET | 全部安全状态 | （面板安全徽标数据源） |
| `/api/skills/security/{id}/{approve\|quarantine\|revoke}` | POST | 安全动作 | `securityAction` |

注册：`cmd/server/routes.go` `auth.Wrap(skillsHandler.Routes())`。`skillListItem`（`skills_handler.go:50`）含 `mountHealth`（`105`）与 `security`（`:61`，nil 视为 approved）。

### 4.9 关键前端组件

| 组件/文件 | 职责 |
|-----------|------|
| `web/src/components/settings/SkillsPanel.tsx` | 技能管理面板：概览 + 搜索 + 漂移横幅(策略切换) + 卡片(启用/触发 chips/5 carrier 挂载按钮/挂载健康+安全徽标) + 预览弹窗(安全批准/隔离) |
| `web/src/types/index.ts:272-305` | `SkillItem`/`SkillDetail`/`SkillSecurityState`/`SkillDriftIssue` 接口 |
| `web/src/services/http.ts` | `apiGet`/`apiPatch`/`apiPost` 封装 |

`CARRIERS = ['claude','codex','gemini','opencode','kimi']`（`SkillsPanel.tsx:6`）；`DRIFT_LABEL`/`HEALTH_LABEL`/`SECURITY_LABEL`（`:8-42`）做中文化映射。

### 4.10 平台接线

`platform.go:318` `skills.NewManagerWithConfig(homeCfg, projCfg, map[string]string{cfg.SkillsDir:"packs"})` → `Config().Load()` + `Scan()` → `Config().Watch(... ReloadAll ...)`（`:323`）。`KnownCarriers = [claude,codex,gemini,opencode,kimi]`（`manager.go:22`）。

---

## 5. Story 级 Definition of Done (DoD Checklist)

- [x] Skills Framework 主链路文档化（扫描 → 持久化 → 多 carrier 挂载/注入 → 漂移治理 → 安全层 → 生产注入 → d11 触发 → REST API → 前端面板）已固化于本文。
- [x] 数据模型/持久化/挂载/漂移/安全/注入/resolver/API/前端 八个技术契约小节齐全，均带 `file:line` 锚点。
- [x] 红线已闭合：无新 `internal/` 顶层（scan/security 均在 `internal/skills/`）；零 LLM（纯正则/哈希）；触发走 d11 动态 resolver（非硬编码 DAG）；无新不可逆决策。
- [x] 前端可管理性完整：启停 / 挂载点 / 漂移 check+resolve（策略切换）/ 预览 / 安全批准·隔离。
- [ ] 单元测试覆盖率达到团队基线（`reconciler_test.go`/`config_test.go` 已覆盖多 carrier symlink、级联 config-new/orphan、安全阻断外部源等核心路径）。
- [ ] 静态代码扫描无 P0/P1 级安全漏洞（`go build ./...` + `go vet` + `go test ./internal/skills/...` + `tsc -b` + `vite build` 全绿为现状基线）。
- [ ] 监控告警与降级开关在预发/灰度环境验证正常（当前依赖 Ops 端点人工观察，自动告警待补）。

---

## 6. 已知缺口处置总览（截至 2026-08-19）

| # | 缺口/议题 | 状态 | 证据 |
|---|-----------|------|------|
| G-1 | **内容扫描层（content scanning）** | ⚠️ **真实缺口（开放外部源前最该补）** | `security.go` 有状态机 + 指纹隔离，但**无** `ScanSkillContent` 式正则扫描（忽略/绕过安全指令、外泄、禁用安全等）；当前仅 `packs` 内部源，威胁不成立，待"开放外部/插件 skill 源"时补 |
| G-2 | 自定义挂载点（custom mount path） | ⚠️ **扩展点（YAGNI）** | `nativeSkillsDirConvention`（`reconciler.go:21`）硬编码 4 carrier；clowder 有 `customPaths` 给未知 client 留扩展；SG 当前 carrier 固定，待支持新 carrier 类型时再做 |
| G-3 | 多项目级联 / 项目层 config-new·orphan | ✅ **已落地（按 SG 单工作区有意保留）** | `drift.go:130` 仅在存在 project 覆盖层时检测 config-new/orphan；级联随 `MergedIntents` + scope 路由实现，对 SG 单工作区不构成负担 |
| G-4 | resolver 不按 carrier 过滤（clowder 缺失项） | ✅ **SG 已领先（有 `mountedToCarrier`）** | `resolver.go:155` 显式过滤，clowder d11 hook 仅消费 `skill:` tag 不按 carrier 过滤 |
| G-5 | 配置热加载 | ✅ **SG 领先** | `config.go:178` `Watch` 已接线；clowder 为"启动时加载一次、无热加载" |
| G-6 | 前端安全态可见性 | ✅ **SG 领先** | `SkillsPanel.tsx:351-376` 渲染安全徽标 + 批准/隔离；clowder web 面板未 surface 安全态 |

**无 P0/P1 阻塞项**：Skills Framework 主链路无阻断性缺口。仅 **G-1（内容扫描）** 为"开放外部 skill 源"前的必要前置（状态机 T2 已补，但缺内容级扫描）；G-2/G-3 为按 SG 自身环境的有意选择或扩展点，不构成缺陷。

---

> 关联文档：`docs/plans/skills-framework-sg-vs-clowder-2026-08-18.md`（前后端逐项对比，含外部参考）、`internal/platform/platform.go`、`internal/prompt/builder.go`、`web/src/components/settings/SkillsPanel.tsx`。
