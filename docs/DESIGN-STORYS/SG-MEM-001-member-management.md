# [SG-MEM-001] [Tech Story] 设置页「成员管理」设计

> 本文件由 `SG-MEM-001-member-management.md`（成员管理前后端）、`SG-MEM-002-empty-first-run.md`（首启空 Catalog + 凭据就绪闸门）合并而成。
> 内容按 `sounds-great-ai` **2026-08-13 代码实况**逐文件重新梳理：前端 `web/src`、后端 `internal/packapi` + `internal/transport/{settings,config}_handler.go` + `internal/settings`、`internal/platform/{breeds_merge.go,router.go}`、装配 `cmd/server/routes.go`。
> 相对三份旧文档，已修正以下与代码不符之处：
> - `settings-nav-config.ts` 实际 `DEFAULT_SECTION = 'accounts'`（旧文档误写为 `'members'`）；`RAW_SECTIONS` 顺序为 `accounts` 在前、`members` 在后。
> - **`dog-catalog.json` 磁盘上没有独立的 `breedOrder` 键**——排序真相是 `breeds[]` 数组顺序（`flushCatalog` 按内存 `breedOrder` 切片顺序写出 `breeds[]`；`GetBreedOrder` 直接返回 `breeds[]` 顺序；`SetBreedOrder` 调 `ReorderBreeds` 重排 `breeds[]` 数组）。旧文档 §5.1 的 `"breedOrder": [...]` 是误导。
> - `GET /api/breeds/templates` 实际返回**完整 `BreedConfig[]`**（供前端模板全量实例化），不是 `RoleTemplate[]`（旧文档 §7.1 schema 误标）。
> - 另补充旧文档遗漏的端点：`POST /api/breeds/{id}/bark`、`GET /api/breeds/{id}/status`、`GET /api/config/env-summary`、`PATCH /api/config/env`。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu | QA: @demu
- **故事点/复杂度**: [ M (3-5分) ]
- **范围**: 设置页 **`members` 分区**（成员 = breed 的增删改、启用停用、拖拽排序、默认犬、大当家、凭据就绪闸门、首启空 Catalog）。
- **业务/技术目标**:
  - As a **平台使用者（operator）**,
  - I want to **在设置页统一管理「犬队成员」（创建/编辑/启用停用/排序/设默认犬/绑定账号），让运行时名册与排序真正落盘；且全新安装首启为空、只有添加成员才有犬、无密钥的成员不算就绪**,
  - So that **成员与 pack 层 breed 同源、默认犬与拖拽顺序刷新不丢失、账号绑定不产生悬空引用、且后端有别名唯一性/白名单/account_ref 守护**。
- **关键指标/埋点**: 无（内部配置管理链路，非对外曝光功能）。

---

## 2. 功能叙事 (User Journey)

「成员管理」是 **Settings 内的一个分区**（nav id=`members`，与 `accounts` 并列）。设置页默认进入 `accounts`，左侧点「成员管理」→ 右侧渲染 `MemberManagement` 列表页。

> **前置依赖（与「账户与密钥」`SG-ACC-001`）**：成员的 `variant.account_ref` 必须指向《账户与密钥》中已存在的账户（或内置 OAuth client）。即「**先建凭证，再建关联 Agent 角色**」——无可用账号时，添加成员流程内联引导先建账号（见 §7.1），而非凭空创建。

页面自上而下：
1. **工具栏**：filter tabs（全部/已启用/已停用/CLI(OAuth)/CLI(配置)）+ 「+ 添加成员」按钮。
2. **全局默认犬选择器** `DefaultDogSelector`：下拉选一只 breed 设为默认犬，落盘持久化。
3. **大当家卡片** `LeaderOverviewCard`：点开 `HubLeaderEditor`（Owner / 大当家，单独模态框）。
4. **可拖拽成员行** `MemberOverviewCard`：启用段可拖拽排序（落盘），每行有启用/停用开关 + 删除按钮；点行打开「添加成员」编辑模态框。
5. **停用段**：折叠在底部，不可拖拽。

点「+ 添加成员」→ 弹出 **`HubBreedEditor`**（全屏居中深色模态框，overlay 点击关闭）。这是**模态框而非独立路由页**，承载「新增」与「编辑」两种模式：
- 顶部可选成员模板（完整 `BreedConfig`，从 `/api/breeds/templates` 全量实例化）或「自定义」。
- 三大区块：`IdentitySection`（名称/显示名/昵称/头像/配色/角色描述/性格）、`AccountSection`（client 下拉 + 账号选择 + 默认模型）、`RoutingSection`（mention 别名 / 路由关键词）。
- `AdvancedRuntimeSection`：会话链 / CLI 运行参数 / 上下文预算 / MCP / 策略。
- 若所选 client 下**没有任何可用认证账号** → 展示空状态 CTA「新建 / 登录账号」，点击复用 `AccountAuthModal` 先建账号再继续。
- 保存后写 breed（catalog），列表刷新。

**首启空 Catalog 行为**：全新安装首启 `dog-catalog.json` 为空（仅 owner），成员列表呈现引导空态（「还没有成员，添加成员 / 从模板选择」），不崩溃。成员按派生三态显示：**就绪**（绿，已启用且凭据就绪）/ **待配置**（琥珀，已启用但缺密钥或 CLI）/ **已停用**（灰）。

---

## 3. 前端处理逻辑

相关文件：`web/src/components/settings/{MemberManagement.tsx, HubBreedEditor.tsx, breed-editor.model.ts, breed-editor.sections.tsx, breed-editor-advanced.tsx, breed-editor-fields.tsx, primitives.tsx}`、`web/src/services/{settingsService.ts, breedService.ts}`、`web/src/hooks/{useSettings.ts, useBreeds.ts, useLeaderConfig.ts}`、`web/src/store/useAppStore.ts`、`web/src/constants/clientIds.ts`。

### 3.1 导航与分区挂载

- `settings-nav-config.ts`：`RAW_SECTIONS` 仅保留 `accounts`（「账户与密钥」）与 `members`（「成员管理」）两个分区，**顺序 accounts 在前、members 在后**；`DEFAULT_SECTION = 'accounts'`（设置页默认进账户与密钥；`members` 是并列分区——旧文档称 `DEFAULT_SECTION='members'` 与代码不符，已修正）。
- `SettingsContent.tsx`：`SECTION_COMPONENTS = { members: MemberManagement, accounts: AccountKeys }`，均 `lazy`；直接用 `meta.label/description` 包 `SettingsPageHeader`，面板不自带页头。

### 3.2 列表主页 `MemberManagement.tsx`

- **数据来源**：`useBreeds()` 拿 `breeds`（**仅 catalog 运行时犬**，来自 `/api/breeds`；`MergedBreeds` 不再并入 template）；`useSettings()` 拿 `roster`（`/api/settings/roster`，每条含派生 `credential_ready`）。`members = breeds.map(b => breedToSettingsMember(b, roster[b.id]))`（`useMemo`，依赖 breeds+roster）。首启 catalog 为空时 `breeds=[]`，列表呈现引导空态。
- **`breedToSettingsMember` 投影**（核心映射）：
  - `id ← b.id`，`name←b.name`，`breed←b.display_name`，`color←b.color.primary`。
  - 取默认 variant：`variants.find(v => v.id===b.default_variant_id) ?? variants[0]`；从该 variant 取 `default_model / client_id / account_ref / provider / session_chain / cli.* / strategy / context_budget.* / mcp_support`。
  - **启用唯一真相**：`enabled = roster[id]?.available ?? b.enabled`（roster 优先）。
  - **凭据就绪（派生）**：`credentialReady = roster[id]?.credential_ready ?? false`。三态 = `enabled ∧ credentialReady`：**就绪** / **待配置** / **已停用**。有效可用性以三态为准。
  - `type`：`account_ref` 命中 `OAUTH_REFS`(claude/codex/gemini/kimi/opencode) → `'CLI (OAuth)'`，否则 `'CLI (config)'`。
  - `teamStrengths`：`b.team_strengths`（逗号串）拆数组；`mentionPatterns←b.mention_patterns`（路由真相）。
- **全局默认犬**：`useEffect` 挂载时 `settingsService.getDefaultBreed()` 取 `breed_id`；选择器 `onChange→handleDefaultDogChange(id)`：若该犬 `roster[id].available` 为假先 `updateRosterEntry(id,{available:true})` 启用，再 `setDefaultBreed(id)` 落盘（`/api/config/default-breed`），失败 toast。
- **启用/停用**：`handleToggleEnabled(m)` → `updateRosterEntry(m.id,{available:!m.enabled})`（`PATCH /api/settings/roster/{id}`，部分更新），随后 `refetchBreeds()+refetchSettings()`。
- **删除**：`handleDelete(m)` → `window.confirm(members.confirmDelete)` → `deleteBreed(m.id)`（`DELETE /api/breeds/{id}`），随后 refetch。
  > 删 breed 本身不会触发账号引用的 409——账号引用保护在「账户与密钥」页删账号时（`DeleteAccount` 扫 breeds 的 `variant.account_ref`）；成员页只负责删成员。
- **拖拽排序**：`onDragStart/onDragOver/onDrop/onDragEnd` + `reorderIds(enabledIds, src, target)`。落盘逻辑：乐观更新 `localMembers`（启用段按拖拽重排）→ `settingsService.setBreedOrder([...enabledIds, ...disabledIds])`（`PUT /api/config/breed-order`）→ refetch；失败则 `setLocalMembers(members)` 回滚到服务端顺序 + toast。
- **空状态 / 加载**：`breedsLoading` → loading strip；`localMembers.length===0` → 引导空态 `EmptyMembersState`（图标 + 「还没有成员」+ 「添加成员」CTA + 「可从模板一键实例化」提示），不崩溃。默认犬选择器在 `breeds.length===0` 时禁用并提示「添加成员后可设置全局默认犬」。

### 3.3 「添加成员」模态框 `HubBreedEditor.tsx`

`fixed inset-0 z-50` 全屏遮罩 + 居中弹层（SLATE 深色 + amber）。复用 SG 深色主题。

**两种模式**（顶部模板切换，编辑态禁用）：`isEdit = Boolean(breed)`。

- **挂载加载**：
  - `getAccounts()`（`/api/settings/accounts`）→ `profiles`（账号列表）。
  - 仅新建：`apiGet('/api/breeds/templates')` → `templates`（**完整 `BreedConfig[]`**，含 variant / CLI / role / caution；非只读 `role_templates`）。
  - `breedService.getBreeds()` → `members`（用于 `reservedPatterns` 别名去重）。
- **`availableProfiles = filterAccounts(form.clientId, profiles)`**；选中账号决定 `modelOptions` 与 `authType`。
- **`reservedPatterns`**：遍历 `members` 的 `mention_patterns`，收集除自身外的小写别名集合 → 前端别名唯一性提示（后端也有守护）。
- **模板选择** `handleTemplateSelect`：**全量实例化** —— 调 `initialState(tpl)` 把整份模板 `BreedConfig` 灌入表单，再覆盖 `dogId=autoSlug(name)` 与去重后的 `mentionPatterns`。用户只需绑定账号与默认模型即可得与模板设计一致的可用犬。
- **关闭确认**：`hasUnsavedChanges` 为真时关闭弹内确认框。

**提交校验 `handleSave`**（仅新建严格校验）：
- 必填：`name`、`roleDescription`；若所选账号 `authType==='api_key'` 还需 `defaultModel`；`mentionPatterns`（别名）至少 1 个。
- api_key 账号：`withDefaultModelMentionPattern(form)` 自动补一条 `<model>` 别名。
- 校验失败 → `setFieldErrors` + `setError('请填写必填字段：...')` 不提交。

**空账号态**：`hasEmptyCreatableAccounts = !isEdit && availableProfiles.length===0` → 渲染 CTA「新建 / 登录账号」→ 打开内嵌 `AccountAuthModal`（`onCreated(profileId)` 写 `pendingProfileIdRef` 并重新拉账号列表，回填 `accountRef`+首个 model）。

**提交动作**：`buildBreedPayload(effectiveForm, breed)` → `onSave(payload)` → 由 `MemberManagement.handleSaveBreed` 调 `updateBreed(breed.id, breed)`（编辑）或 `createBreed(breed)`（新建），随后 `refetchBreeds()+refetchSettings()` + success toast。

### 3.4 前端 API 调用层

`web/src/services/settingsService.ts`（成员相关）：
- `getRoster()` → `GET /api/settings/roster`，返回 `Record<id, RosterEntry>`（空则 `{}`）。
- `updateRosterEntry(id, patch)` → `PATCH /api/settings/roster/{id}`（部分字段合并）。
- `getReviewPolicy()/updateReviewPolicy(policy)` → `GET/PUT /api/settings/review-policy`。
- `getDefaultBreed()` → `GET /api/config/default-breed`，返回 `{breed_id, is_override}`。
- `setDefaultBreed(breedId)` → `PUT /api/config/default-breed`（`{breed_id}`）。
- `getBreedOrder()/setBreedOrder(order)` → `GET/PUT /api/config/breed-order`（`{order:[...]}`）。
- `getAccounts()` → `GET /api/settings/accounts`（调 `mapAccountApiToUi`）。

`web/src/services/breedService.ts`（ breeds CRUD，成员走这里）：
- `getBreeds()` → `GET /api/breeds`（合并 breeds）。
- `createBreed(config)` → `POST /api/breeds`。
- `updateBreed(id, updates)` → `PATCH /api/breeds/{id}`。
- `deleteBreed(id)` → `DELETE /api/breeds/{id}`。

> 注：模板接口 `GET /api/breeds/templates` **不经过 `breedService`**（该 service 仅暴露上面 4 个 CRUD 方法）。前端由 `HubBreedEditor.tsx`（约第 112 行）直接 `apiGet<BreedConfig[]>('/api/breeds/templates')` 拉取完整 `BreedConfig[]` 实例化成员，而非通过 `breedService` 封装。

`web/src/hooks/useBreeds.ts`：模块级 `breedCache` 缓存；`createBreed/updateBreed/deleteBreed` 成功后 `fetchBreeds()` 失效缓存并重设 state；错误 toast。

`web/src/hooks/useSettings.ts`：持有 `roster` state（`getRoster()`）；`fetchAll` 并行拉 `getRoster()+getAccounts()+getSystemConfig()`；返回 `{roster, accounts, config, loading, error, updateRosterEntry, addAccount, deleteAccount, refetch}`。旧 members 状态/方法已移除（成员走 breeds）。

`web/src/hooks/useLeaderConfig.ts`：`leader` + `updateLeader` → `GET/PATCH /api/config/leader`。

`web/src/store/useAppStore.ts`：**已删除 `globalDefaultDog` 字段及 `setGlobalDefaultDog`**（被默认犬端点取代）。保留 `memberFilter` / `showAddMemberModal` 等 UI state。

---

## 4. 后端处理逻辑

### 4.1 成员 = breed（方案 B 核心）

成员列表 ↔ `/api/breeds`（合并注册表：catalog 运行时 breeds）。运行时名册 ↔ `/api/settings/roster`（catalog `roster{}`）。两者在同一 `dog-catalog.json`，由 `FileSettingsStore` 管理。

注册表装配（`internal/platform`）：`MergedBreeds` **仅返回 catalog 犬**（`dog-template.json` 不再并入 active registry）；`platform.New` 用 catalog 构造 `pl.Breeds`；`SetupPack` 加载全部模板犬后再用 `MergedBreeds` 收敛（剔除模板-only 犬，仅 catalog 进 registry），`WatchBreedsFile` 热加载（变更经事件总线/`HotReloader` ~30s 防抖）。模板仅作「添加菜单」数据源（`GetTemplates`）。

### 4.2 pack-api 路由（`internal/packapi/handler.go`，挂 `/api/breeds`，**已**包 auth.Wrap）

- `GET /api/breeds`（`ListBreeds`）→ `h.pack.List()` 返回合并 breeds。
- `POST /api/breeds`（`CreateBreed`）→ 解码 `BreedConfig`；`Source` 空则补 `user`；`validateBreed` + `pack.Validate`；`pack.Register`；`persistBreed`（写 catalog）；任一失败回滚（Unregister / 500）。→ 200 `breed`。
- `PATCH /api/breeds/{id}`（`UpdateBreed`）→ 取现有 breed；**`source==system` → 403 `system breeds cannot be modified`**；逐字段部分解码（白名单：`display_name/avatar/personality/role_description/team_strengths/mention_patterns/roles/color/nickname/caution/default_variant_id/features/restrictions/relationship_key/variants`；未传字段保留）；`validateBreed` + `pack.Validate` + `persistBreed`；成功 `eventBus.Emit`。→ 200 `breed`。
- `DELETE /api/breeds/{id}`（`DeleteBreed`）→ `pack.Unregister`（system 403）；`removeBreed`（catalog 删 breed + 删对应 roster entry + 从 breedOrder 移除 + 记 `deleted_breeds`）；→ 200。
- `GET /api/breeds/templates`（`GetTemplates`）→ 返回 `dog-template.json` 的 **完整 `BreedConfig[]`**（完整 variant / CLI / role / caution，供前端「从模板添加成员」全量实例化；**只读种子**，不直接进 registry）。
- `POST /api/breeds/{id}/bark`（`BarkBreed`）→ 执行该 breed 的 Bark（旧文档遗漏）。
- `GET /api/breeds/{id}/status`（`GetBreedStatus`）→ 运行时状态（旧文档遗漏）。

**`validateBreed(cfg, excludeID)`**（集中守护）：
- `pack.CheckMentionPatternsUnique(pack.List(), cfg.MentionPatterns, excludeID)` → 别名（mention_patterns）全局唯一，冲突报 `alias %q is already used by member %q`（400）。
- 每个 variant：`settings.ValidateClientID(v.ClientID)` → 不在白名单 400。
- 每个 variant：`settings.ValidateAccountRef(h.store, v.AccountRef)` → 不存在 400。

### 4.3 settings 路由（`internal/transport/settings_handler.go`，挂 `/api/settings/`，**包 auth.Wrap**）

- `GET /api/settings/roster`（`ListRoster`）→ `store.GetRoster()` → `map[id]RosterEntry`；响应经 `enrichRoster` 附加 `credential_ready`。
- `GET /api/settings/roster/{id}`（`GetRosterEntry`）→ 命中返回 entry（含 `credential_ready`），否则 404 `roster entry not found`。
- `PATCH /api/settings/roster/{id}`（`UpdateRosterEntry`）→ **从现有 entry 起算部分更新**（未传字段保留）；`store.UpdateRosterEntry` 若 `ErrBreedNotFound` → 404 `breed not found`，否则 200。
- `GET /api/settings/review-policy` / `PUT /api/settings/review-policy`（`GetReviewPolicy`/`SetReviewPolicy`）→ pack.ReviewPolicy 读写。
- `DELETE /api/settings/accounts/{id}`（`DeleteAccount`，引用完整性）→ 见 §4.5。

### 4.4 config 路由（`internal/transport/config_handler.go`，挂 `/api/config/`，**已**包 auth.Wrap）

- `GET /api/config/default-breed`（`GetDefaultBreed`）→ 优先 `DEFAULT_BREED_ID` 环境变量（`is_override=true`），否则读 `configs["default_breed"]`；返回 `{breed_id, is_override}`。
- `PUT /api/config/default-breed`（`SetDefaultBreed`）→ 非空时 `knownBreedIDs()`（模板 union catalog）校验，未知 → 404；落 `UpdateConfig("default_breed", id)`（持久化）；env override 在读取时仍优先。
- `GET /api/config/breed-order`（`GetBreedOrder`）→ 返回 catalog `breeds[]` 顺序（空则回退模板顺序）；**排序真相 = breeds[] 数组顺序（无独立 breedOrder 键）**。
- `PUT /api/config/breed-order`（`SetBreedOrder`）→ `knownBreedIDs()` 校验未知 id（返回 `unknown breed IDs` + `missing` 列表 400）；`store.ReorderBreeds(order)` 重排 catalog `breeds[]`。
- `GET/PATCH /api/config/leader`（`GetLeader`/`UpdateLeader`）→ pack.Leader 读写（catalog 持久化）。
- `GET /api/config/env-summary` / `PATCH /api/config/env`（`GetEnvSummary`/`UpdateEnv`）→ 环境变量摘要/更新（旧文档遗漏；`UpdateEnv` 对敏感 key 要求 loopback 访问）。

### 4.5 删除账号的引用完整性（反方向保护）

`DeleteAccount`（`settings_handler.go`）：删账号前扫 `store.ListBreeds()` 各 variant 的 `account_ref` 是否命中该 id；有绑定且 query 非 `force=true` → **409 `{error, bound_member_ids}`**。这保证「成员绑定的账号」不能静默被删（成员侧只展示引用，不自动级联）。

### 4.6 白名单与 account_ref 校验

`internal/settings/validation.go`：
- `ValidCLIClientIDs = {claude, codex, gemini, opencode, kimi}`；`OAuthClientRefs` 同值。
- `ValidateClientID(id)`：空 → true（generic api_key 账号，不限厂商）；非空必须命中白名单，否则 false。
- `ValidateAccountRef(store, ref)`：空 → 放行（解绑）；命中 `OAuthClientRefs` → 放行（内置 CLI OAuth，非 catalog 账号）；否则必须命中 `store.ListAccounts()` 的 id，否则 400。**该校验复用 handler 持有的同一 `store` 实例**，绝不重新解析数据根。

前端 `filterAccounts(clientId, profiles)`：`clientId` 为空 → 返回全部账号（含无 clientId 的通用 api_key 账号）；否则返回 `profile.clientId===clientId || !profile.clientId`（匹配或通用账号均可见）。

---

## 5. 存储与数据模型

### 5.1 catalog 唯一真相

`.sounds-great-ai/dog-catalog.json`（0644，由 `FileSettingsStore` 管理，`MarshalIndent` + tmp+rename 原子写）：

```json
{
  "version": 2,
  "breeds": [ BreedConfig, ... ],     // 成员全量（含 source/enabled）；数组顺序即排序真相
  "roster": { "id": RosterEntry },    // 运行时名册（available/roles/lead/family/evaluation）
  "review_policy": { ... },
  "leader": { ... },
  "configs": [ {key,value,category}, ... ],  // default_breed 等
  "deleted_breeds": [ "id", ... ],          // 客户显式删除过的犬（升级同步跳过复活）
  "seen_template_breeds": [ "id", ... ]      // 已"见识过"的模板犬（首启空 + 升级追加同步共用）
}
```

> **重要**：磁盘上**没有独立的 `breedOrder` 键**。排序真相就是 `breeds[]` 数组的顺序——`GetBreedOrder` 直接返回 `breeds[]` 顺序；`SetBreedOrder` 调 `ReorderBreeds` 重排 `breeds[]` 数组；`flushCatalog` 按内存 `breedOrder` 切片顺序把 `breeds[]` 写出。旧文档 §5.1 画出的 `"breedOrder": [...]` 是误导，请勿据此理解存储格式。

`packs/default/breeds/dog-template.json`（0644，**只读种子**）：`version:2 + role_templates + client_defaults + leader + breeds[]`（默认 7 只 system 犬，仅作「添加菜单」数据源）。`MergedBreeds` 仅返回 catalog 犬；`SyncTemplateBreeds` 按 `seen_template_breeds` 机制同步（详见 §7.3）。

### 5.2 三文件隔离（与账户同根）

> 凭证隔离机制与 0600 权限以《账户与密钥》`SG-ACC-001` §5.1 为准；本表为成员 catalog 字段的**完整真相**（ACC §5.1 已反向指向此处，避免双写）。

| 文件 | 内容 | 权限 | 管理方 | 落盘根 |
|---|---|---|---|---|
| `dog-catalog.json` | breeds + roster + review_policy + leader + configs + deleted_breeds + seen_template_breeds | 0644 | `FileSettingsStore` | `ConfigRoot`（项目 `.sounds-great-ai`） |
| `accounts.json` | 账号元数据 | 0644 | `FileSettingsStore` | `ConfigRoot`（项目 `.sounds-great-ai`） |
| `credentials.json` | 密钥（明文） | 0600 | `FileCredentialStore` | `CredentialRoot`（**全局 home `~/.sounds-great-ai`**，独立于项目根） |

**根目录已拆分（客户配置安全）**：`catalog/accounts` 走 `ConfigRoot(projectRoot)`；`credentials` 单独走 `CredentialRoot()`（全局 home `~/.sounds-great-ai`，可被 `SOUNDS_GREAT_AI_CREDENTIAL_ROOT` 覆盖）。清项目配置**不会**误删密钥（详见《账户与密钥》设计 §5）。

### 5.3 关键结构体（`pkg/pack/breed.go`）

- `BreedConfig`：`ID, Name, DisplayName, Nickname, Avatar, Color{Primary,Secondary}, Personality, RoleDescription, TeamStrengths, MentionPatterns[], Roles[], Caution, DefaultVariantID, Variants[], Review, Features{SessionChain, MissionHub{SelfClaimScope}}, Restrictions[], RelationshipKey, DogID, Source(system/user/plugin), Enabled`。← `dog_id` / `features` / `restrictions` / `relationship_key` 保留 snake_case 以兼容 Go 解析器。
- `Variant`：`ID, VariantLabel, ClientID, DefaultModel, MCPSupport, CLI{Command,OutputFormat,DefaultArgs[],Effort}, SystemPrompt, Personality, Strengths[], TeamStrengths, Caution, ContextBudget{MaxPromptTokens,MaxContextTokens,MaxMessages}, VoiceConfig, AccountRef, Provider, SessionChain, Strategy, AutoCompactTokenLimit, Name, DisplayName, Avatar, Color{Primary,Secondary}, MentionPatterns[], RoleDescription, DogID, Restrictions[]`。← 成员绑定账号走 `variant.account_ref`；`name/display_name/avatar/color/mention_patterns/role_description/dog_id/restrictions` 为逐变体可覆盖身份字段。
- `RosterEntry`：`Family, Roles[], Lead, Available, Evaluation`。← `Available` 为启用唯一真相；派生 `CredentialReady` 由 `settings_handler.enrichRoster` 在响应层附加，**不入 catalog 落盘**。
- `ReviewPolicy`：`RequireDifferentBreed, PreferActiveInThread, ExcludeUnavailable, PreferLead, PreferredRoles[]`。
- `Leader`：`Name, Nickname, Avatar, TimeZone, Aliases[], MentionPatterns[], ColorPrimary, ColorSecondary`。

### 5.4 损坏处理与编辑时备份

- **加载时损坏**：`reloadFromDisk`（`file_store.go` / `credential.go`）解析 JSON 失败 → **仅告警并当空处理**，不再自动备份（不掩盖损坏）。
- **编辑时备份**：每次写盘（`flushCatalog` / `flushAccounts` / credential `flush`）覆盖前，若目标文件已存在，先快照为 `<path>.bak-<YYYYMMDD-HHMMSS>`（保留最近 5 份，`pruneBackups`），供客户回滚到上一版本。
- 三种文件（accounts / dog-catalog / credentials）均覆盖。（详细见《账户与密钥》设计 §5.5。）

---

## 6. 首启空 Catalog + 凭据就绪闸门（D1–D4，原 SG-MEM-002）

> 用户反馈（2026-08-13）：「我没配置密钥，为什么蛮多狗狗都是开启的？正常理解我第一次用，没有密钥，狗狗清单应该也是空的。只有在成员管理添加了成员才会有狗狗。」本决策推翻了早先「种子默认启用」的首启策略。

### 6.1 决策结论

| 决策 | 结论 |
|---|---|
| **D1 首启形态** | **空 catalog + 仅 owner**（采纳建议）。首启不含任何犬。 |
| **D2 密钥闸门** | **做**。`api_key` 查 `credentials.json`；`oauth` 查 CLI 二进制是否存在。 |
| **D3 升级自动加新犬** | **保留**。升级新增的模板犬自动加入 catalog。 |
| **D4 VISION 更新** | **更新**（`VISION.md` §5.1 新增 + ADR-001）。 |

原则：全新安装首启 `dog-catalog.json` 为空（仅 owner）；犬只通过「成员管理 → 从模板添加」出现，落盘进 catalog；**已有实例不被清空**——仅全新安装走空初始化；`dog-template.json` 保留为完整「可选菜单」（7 犬 / 14 变体），不再自动进入运行时 registry。

### 6.2 seen_template_breeds 机制（统一 D1 + D3，权威描述）

`internal/platform/breeds_merge.go` 引入持久化集合 `seen_template_breeds`（语义：该用户 catalog 已经"见识过"的模板犬 ID）。`SyncTemplateBreeds` 每次启动执行：

```
若 catalog 文件不存在（首启）：
  写空 catalog（breeds: []）+ seen_template_breeds = 当前模板全部 breeds[].id
  → 首启为空，不注入任何犬（D1）
否则（已有 catalog）：
  对每个模板 breed：
    若 breed.ID ∈ catalog           → 跳过（已存在）
    若 breed.ID ∈ seen_template_breeds → 跳过（已见过但用户未添加，不复活）
    否则（升级新增的模板犬）         → 复制进 catalog（Enabled=true）+ 加入 seen
                                        → 升级自动加新犬（D3）
```

- 首启后重启（用户未添加任何犬）：catalog 空，所有模板犬都在 `seen` → 不会被倒灌。
- 用户手动从模板添加某犬 → `CreateBreed` 落 catalog（该 ID 已在 seen，无冲突）；删除后仍在 seen → 不再复活。
- `deleted_breeds` 语义被 `seen` 覆盖（`seen` 已含"已删/未添加"），可退役或做兼容迁移。

**迁移（既有实例）**：已有 catalog 的实例，`seen` 初始化为 `catalog 现有 breed ID ∪ deleted_breeds ID`（`SyncTemplateBreeds` 内 `len(seenSet)==0` 时种子），保证不复活、不重复加；升级新增犬照常自动加入。仅"无 catalog 文件"才算首启。

### 6.3 MergedBreeds 收敛

- registry 仅含 **catalog 犬**；template 不再并入 active registry（`setup.go` 注册 merged 结果，改后仅 catalog）。
- template 仅作「添加菜单」数据源——`GetTemplates` 已独立读 `dog-template.json`，不受合并改动影响。

### 6.4 credential_ready 派生状态（D2）

`RosterEntry.Available` 保留为用户开关（on/off 意图，可持久化）。新增**派生**状态（由 `settings.CredentialReady` 计算，响应层经 `enrichRoster` 附加，`breed.go`/`credential.go`）：

```
credential_ready(breed) = 依据 default variant 绑定的账号：
  account.auth_type == "oauth"（client_id ∈ {claude,codex,gemini,opencode}）
      → exec.LookPath(client_id) 二进制存在
  account.auth_type == "api_key"
      → FileCredentialStore.Has(account.ID)   // credentials.json（0600）
  无绑定账号 → false

> 注：`kimi` 走 api_key（环境变量 `KIMI_API_KEY`）而非本地 CLI 二进制，故 oauth **二进制探测集为 4 个**（claude/codex/gemini/opencode）。这与 `ValidateAccountRef` 的 5 个内置 OAuth client（含 kimi，见 §4.6）不冲突——前者指"是否查 CLI 二进制存在"，后者指"无需 catalog 账户即可被 `account_ref` 引用"，是两件事。
```

- **有效可用性 = `Available ∧ credential_ready`**。前端三态：`已启用（就绪）` / `待配置（已启用但无密钥或 CLI）` / `已停用`。
- api_key 权威检查走 `FileCredentialStore.Has(account.ID)`（`credentials.json`，0600），而非仅 `Account.KeySet` 元数据（KeySet 是展示用，密钥真相在凭证文件）。
- 默认以 `default_variant` 判定；将来可放宽为「任一 variant 就绪即就绪」。
- **已知限制**：OAuth 只查二进制存在（用户明确要求），不校验登录态——"装了 CLI 但没登录"会显示就绪、首次执行才报错。可接受，记入发布说明。

### 6.5 router 空 patterns 友好报错

`internal/platform/router.go` 的 `Route`：当 `len(r.patterns)==0`（空 catalog / 无犬）时，返回 `TargetBreeds:[]` + `Warning:"无可用犬，请先在成员管理添加成员"`，由上层显式提示；不再回落不存在的 `bianmu`。

> 注意：当**有** patterns 但用户未 @ 任何犬时，仍回落 `TargetBreeds:["bianmu"]`（硬编码默认），该行为未被本决策改变——D1/D4 只针对"空 catalog 首启"场景。

### 6.6 VISION 与 ADR

- `VISION.md` §5.1「首启空、按需组队」已更新：六犬是模板/菜单，非出厂即部署；首启 `dog-catalog.json` 为空（仅 owner），用户经「成员管理 → 从模板添加」组队。
- 本决策同时记录于 `VISION.md` §5.1 与本文 §6。

---

## 7. 端到端数据流

### 7.1 新建成员（含绑定账号）

```
[前端] HubBreedEditor.handleSave(form)
   → buildBreedPayload → onSave(breed)
   → MemberManagement.handleSaveBreed → useBreeds.createBreed(breed)
   → POST /api/breeds  (BreedConfig, source=user)
        │
[后端] auth.Wrap → CreateBreed
   → validateBreed: CheckMentionPatternsUnique + 各 variant ValidateClientID + 各 variant ValidateAccountRef
   → pack.Register → persistBreed（追加到 `dog-catalog.json` 的 `breeds[]` 数组末尾（排序真相见 §5.1，无独立 breedOrder 键）+ roster 初值 Available=Enabled）
   → 200 breed
        │
[前端] refetchBreeds + refetchSettings → 列表出现新成员
```

若所选 client 下无账号：前端空状态 CTA → `AccountAuthModal` 先 `POST /api/settings/accounts`（带 apiKey 落 credentials.json）→ `onCreated` 回填 `accountRef` + 首个 model → 再保存成员。

### 7.2 启用/停用成员

```
[前端] MemberOverviewCard 开关 → handleToggleEnabled(m) → updateRosterEntry(m.id, {available: !m.enabled})
   → PATCH /api/settings/roster/{id}  (部分更新)
        │
[后端] UpdateRosterEntry → store.UpdateRosterEntry（breed 不存在 → ErrBreedNotFound → 404）
   → flushCatalog（写 dog-catalog.json roster）
   → 200
        │
[前端] refetch → 启用状态持久化（刷新不丢）
```

### 7.3 拖拽排序

```
[前端] onDrop(target) → reorderIds(enabledIds, src, target)
   → 乐观 setLocalMembers（启用段重排）
   → settingsService.setBreedOrder([...enabled, ...disabled])
   → PUT /api/config/breed-order  {order}
        │
[后端] SetBreedOrder → knownBreedIDs 校验（未知 id → 400）
   → store.ReorderBreeds(order)（重排 catalog breeds[] 数组）
   → 200
        │
[前端] refetch → 顺序持久化（breeds[] 数组顺序）；失败则回滚到服务端顺序 + toast
```

### 7.4 设默认犬

```
[前端] DefaultDogSelector onChange → handleDefaultDogChange(id)
   → (若未启用) updateRosterEntry(id,{available:true})
   → settingsService.setDefaultBreed(id)
   → PUT /api/config/default-breed  {breed_id}
        │
[后端] SetDefaultBreed → knownBreedIDs 校验（未知 → 404）
   → UpdateConfig("default_breed", id)（持久化；env DEFAULT_BREED_ID 在读取时仍优先）
   → 200
```

---

## 8. 技术契约与接口设计 (Technical Contract)

### 8.1 接口 Schema（成员相关）

```
GET    /api/breeds                       → 200 BreedConfig[]            (合并 breeds，仅 catalog)
POST   /api/breeds                       → 200 BreedConfig             (创建 breed)
PATCH  /api/breeds/{id}                  → 200 BreedConfig | 403(system) | 404
DELETE /api/breeds/{id}                  → 200 nil | 403(system)
GET    /api/breeds/templates             → 200 BreedConfig[]           (完整模板，只读菜单)
POST   /api/breeds/{id}/bark             → 200 any                      (执行 Bark)
GET    /api/breeds/{id}/status           → 200 {id,status,...}

GET    /api/settings/roster              → 200 map[id]RosterEntry(credential_ready 附加)
GET    /api/settings/roster/{id}         → 200 RosterEntry | 404
PATCH  /api/settings/roster/{id}         → 200 nil | 404

GET    /api/settings/review-policy       → 200 ReviewPolicy
PUT    /api/settings/review-policy       → 200 nil

GET    /api/config/default-breed         → 200 {breed_id, is_override}
PUT    /api/config/default-breed         → 200 {breed_id} | 404(未知)
GET    /api/config/breed-order           → 200 {order: string[]}       (breeds[] 顺序)
PUT    /api/config/breed-order           → 200 {order} | 400(未知 id)
GET    /api/config/leader                → 200 Leader
PATCH  /api/config/leader                → 200 nil
GET    /api/config/env-summary           → 200 {...}                   (环境变量摘要)
PATCH  /api/config/env                   → 200 {updated:[...]}         (敏感 key 需 loopback)

DELETE /api/settings/accounts/{id}       → 200 nil | 409(bound_member_ids) | 404
```

### 8.2 校验错误码

| 场景 | 端点 | 响应 |
|---|---|---|
| 别名重复 | POST/PATCH /api/breeds | 400 `alias %q is already used by member %q` |
| client_id 非法 | POST/PATCH /api/breeds | 400 `invalid client_id %q; allowed: claude, codex, gemini, opencode, kimi` |
| account_ref 不存在 | POST/PATCH /api/breeds | 400 `account_ref %q not found` |
| 编辑 system breed | PATCH /api/breeds/{id} | 403 `system breeds cannot be modified` |
| roster 指向未知 breed | PATCH /api/settings/roster/{id} | 404 `breed not found` |
| 默认/排序指向未知 breed | PUT /api/config/default-breed / breed-order | 404 / 400 `unknown breed IDs` |
| 删被绑定账号 | DELETE /api/settings/accounts/{id} | 409 `{error, bound_member_ids}` |

### 8.3 文件变动

无数据库；成员全部落 `dog-catalog.json`（同 `ConfigRoot` 目录）。详见 §5。

---

## 9. 验收标准 (Acceptance Criteria - AC)

- [x] **AC-01 (正常路径)**:
  - Given 已进入成员管理分区，When 点「+ 添加成员」填名称/角色描述/选账号+模型+别名并保存，Then 200 且 catalog `breeds[]` 新增该 breed、列表出现、刷新仍在。
  - Given 已在列表，When 拖拽启用段成员到新位置，Then `breeds[]` 顺序持久化、刷新顺序保持。
  - Given 已在列表，When 切全局默认犬选择器，Then `configs["default_breed"]` 落盘、刷新默认犬不变。
  - Given 已在列表，When 拨动启用/停用开关，Then `roster[id].available` 落盘、刷新状态保持。
  - Given 全新安装首启，When 打开成员管理，Then catalog 为空、显示引导空态、不崩溃（D1）。
- [x] **AC-02 (异常与边界)**:
  - When 新建成员别名与已有成员冲突，Then 后端 400；前端 `reservedPatterns` 也提示。
  - When 成员 variant `client_id` 不在白名单且非空，Then 400。
  - When 成员绑定一个不存在的 `account_ref`（非内置 OAuth），Then 400。
  - When 编辑默认 system 犬，Then 403。
  - When 设置默认犬 / 排序传入未知 breed id，Then 404 / 400 `unknown breed IDs`。
  - When roster 指向不存在的 breed，Then PATCH 返回 404。
  - When 配置文件加载时损坏，Then 仅告警并当空处理（不 500）；编辑写盘前自动生成时间戳 `.bak`（保留最近 5 份）。
  - When 拖拽排序保存失败，Then 列表回滚到服务端顺序 + toast。
- [x] **AC-03 (权限与安全)**:
  - 生产环境设 `AUTH_TOKEN` 后，When 未带 `Authorization` 访问 `/api/breeds`、`/api/config/`、`/api/settings/*`，Then 返回 401（auth.Wrap）。
  - 密钥隔离：`credentials.json` 0600，列表响应仅回 `key_preview`；成员绑定只存 `account_ref`（账号 id）不存密钥。
  - envVars 拒绝 `SOUNDS_GREAT_AI_` 前缀（账号侧过滤、成员经账号复用）。
  - 首启空 catalog + 升级追加同步经 `seen_template_breeds` 收敛，旧实例不丢、删过的犬不复活。

---

## 10. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- **[x] 资损与网络安全 (Security)**:
  - 敏感数据脱敏: 是（密钥仅存 `credentials.json` 0600；成员只引用 `account_ref`）。
  - 防重提交/幂等: `DELETE /api/breeds/{id}` 经 `removeBreed` 吞 `ErrBreedNotFound` → 删除模板种子犬时幂等。
  - 鉴权一致：`/api/breeds` 与 `/api/config/` 已包 `auth.Wrap`，与 accounts 页一致（401 拦截）。
- **[x] 高并发与限流降级**: 普通（本地配置链路）。`FileSettingsStore` 用 `sync.RWMutex` 保护；写路径 tmp+rename 原子写；外部改动由 `HotReloader` ~30s 防抖热加载。
- **[x] 可服务性与监控**: 无独立 TraceID；关键失败返回结构化 error。成员写链路（breeds/config）现已与 accounts 一致接入 `auth.Wrap` 鉴权（401 拦截）。
- **[x] 数据质量护栏（首启空 + 升级加新犬，决策 D1/D3）**: `SyncTemplateBreeds` 按 `seen_template_breeds` 同步——**全新安装首启写空 catalog（不注入任何模板犬）**；升级时仅把不在 `seen`、也不在 catalog 的新模板犬加入并记入 `seen`，已见未添加不复活。`router.go` 空 patterns 改为友好报错。派生 `credential_ready`（oauth 查 CLI 二进制 / api_key 查 `credentials.json`）经 `enrichRoster` 附加到名册响应，前端三态「就绪/待配置/已停用」。`GetTemplates` 返回完整 `BreedConfig[]` 供前端模板全量实例化。

---

## 11. Story 级 Definition of Done (DoD Checklist)

- [x] 3-Corner 澄清通过（方案 B 决策 + 首启空决策 D1–D4，AC 已锁定）。
- [x] 单元测试覆盖：packapi 包（Create/Update/Delete breed、别名唯一、client_id、account_ref、system 403、ErrBreedNotFound）；transport 包（Roster/ReviewPolicy/BreedOrder/DefaultBreed/DeleteAccount 引用完整性）；platform 包（`TestSyncTemplateBreeds*`、`TestMergedBreeds*`、router 空名册）；settings 包（损坏当空、编辑 `.bak`、`credential_ready`）。
- [x] 静态/构建门禁：`go build ./...` 0、`go test ./...` 0、`cd web && tsc -b` 0、`vite build` ✓。
- [x] 前端三旧坑已消除：globalDefaultDog 不落盘（→ default-breed 端点）、拖拽不落盘（→ breed-order 端点）、双数据源（→ 合并 breeds 同源）。
- [x] 鉴权一致性：`/api/breeds` 与 `/api/config/` 补 `auth.Wrap`，与 accounts 页一致。
- [x] 首启空 + 凭据就绪闸门：`VISION.md` §5.1 + `ADR-001` 落地，回归测试覆盖。

---

## 12. 修订记录 (Revision History)

- **2026-08-12（成员管理）**：基于方案 B 落地后代码梳理成员管理 + 添加成员前后端；补 `/api/breeds`、`/api/config/` 的 `auth.Wrap`。
- **2026-08-12（seed 修复）**：种子化强制 `Enabled=true` 修复整队误判停用；后续被 D1 推翻。
- **2026-08-12（客户安全三改）**：根目录拆分 / 编辑时 `.bak` / 追加式升级同步（见《账户与密钥》设计 §5.4）。
- **2026-08-13（PATCH 字段白名单补全）**：`UpdateBreed` 白名单补 `color/nickname/caution/default_variant_id/features/restrictions/relationship_key`。
- **2026-08-13（首启空 + 凭据就绪闸门，原 SG-MEM-002）**：推翻「种子默认启用」，全新安装首启空 catalog（D1）；`seen_template_breeds` 统一 D1+D3；`MergedBreeds` 仅 catalog；`credential_ready` 派生三态；`router.go` 空 patterns 友好报错；`VISION.md` §5.1 + ADR-001。
- **2026-08-13（合并与代码对齐）**：合并 MEM-001 + MEM-002 为本文件；修正：`DEFAULT_SECTION='accounts'`、`RAW_SECTIONS` 顺序、`breedOrder` 无独立键（排序=breeds[] 顺序）、`GetTemplates` 返 `BreedConfig[]`、补充 `/bark` `/status` `/env*` 端点、catalog 含 `deleted_breeds`/`seen_template_breeds`。
- **2026-08-13（剔除 Makefile/守护内容）**：经用户确认，构建/守护进程生命周期（原 SG-DEV-001）应独立成文，故从本文件剔除附录 A，恢复为独立文档 `SG-DEV-001-makefile-daemon-reclaim.md`。
- **2026-08-16（与 ACC 理清先后关系 + 一致性校正）**：明确「先凭证后角色」前置依赖（§2）；§5.2 标注本表为成员 catalog 字段完整真相、反向指向 ACC §5.1；修正 §7.1「breedOrder 追加」措辞（→ 追加到 breeds[] 数组末尾，与 §5.1 排序真相一致）；§6.4 澄清 kimi 走 api_key、oauth 二进制探测集为 4 个（与 §4.6 的 5 个内置 OAuth client 不冲突）。

---

> 关联文档：`SG-ACC-001-accounts-keys-auth.md`（账户与密钥/客户配置安全，成员经 `account_ref` 反向引用）、`SG-DEV-001-makefile-daemon-reclaim.md`（构建/守护进程生命周期，独立成文）、`VISION.md` §5.1、`internal/platform/breeds_merge.go`、`internal/platform/router.go`。
