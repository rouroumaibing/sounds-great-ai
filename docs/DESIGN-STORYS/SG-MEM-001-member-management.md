# [SG-MEM-001] [Tech Story] 设置页「成员管理」与「添加成员」前后端处理设计

> 本文档基于 `sounds-great-ai` **当前代码实况**（前端 `web/src`、后端 `internal/packapi` + `internal/transport/{settings,config}_handler.go` + `internal/settings`、装配 `cmd/server/routes.go`）逐文件梳理，非臆想。
> 是对「成员管理」分区及内嵌「添加成员」模态框的前后逻辑完整设计叙事。
> 所有行为以**方案 B 落地后的当前状态**为准（成员 = breed，catalog 唯一真相，`dog-template.json` 降级为只读种子）。
> 与旧版（扁平 `Member` + `/api/settings/members`）已无任何关系——那套 HTTP 端点已在方案 B 阶段 3 删除。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu | QA: @demu
- **故事点/复杂度**: [ M (3-5分) ]
- **业务/技术目标**:
  - As a **平台使用者（operator）**,
  - I want to **在设置页统一管理「犬队成员」（创建/编辑/启用停用/排序/设默认犬/绑定账号），且让成员的运行时名册（roster）与排序真正落盘**,
  - So that **成员与 pack 层 breed 同源（clowder 同构）、默认犬与拖拽顺序刷新不丢失、账号绑定不产生悬空引用，且后端有别名唯一性/白名单/account_ref 守护**。
- **关键指标/埋点**: 无（内部配置管理链路，非对外曝光功能）。

---

## 2. 功能叙事 (User Journey)

「成员管理」是 **Settings 内的一个分区**（nav id=`members`，与 `accounts` 并列；`DEFAULT_SECTION='members'`）。左侧点「成员管理」→ 右侧渲染 `MemberManagement` 列表页。

页面自上而下：
1. **工具栏**：filter tabs（全部/已启用/已停用/CLI(OAuth)/CLI(配置)）+ 「+ 添加成员」按钮。
2. **全局默认犬选择器** `DefaultDogSelector`：下拉选一只 breed 设为默认犬，落盘持久化。
3. **大当家卡片** `LeaderOverviewCard`：点开 `HubLeaderEditor`（Owner / 大当家，单独模态框）。
4. **可拖拽成员行** `MemberOverviewCard`：启用段可拖拽排序（落盘），每行有启用/停用开关 + 删除按钮；点行打开「添加成员」编辑模态框。
5. **停用段**：折叠在底部，不可拖拽。

点「+ 添加成员」→ 弹出 **`HubBreedEditor`**（全屏居中深色模态框，overlay 点击关闭）。这是**模态框而非独立路由页**，承载「新增」与「编辑」两种模式：
- 顶部可选成员模板（只读 `role_templates`）或「自定义」。
- 三大区块：`IdentitySection`（名称/显示名/昵称/头像/配色/角色描述/性格）、`AccountSection`（client 下拉 + 账号选择 + 默认模型）、`RoutingSection`（mention 别名 / 路由关键词）。
- `AdvancedRuntimeSection`：会话链 / CLI 运行参数 / 上下文预算 / MCP / 策略。
- 若所选 client 下**没有任何可用认证账号** → 展示空状态 CTA「新建 / 登录账号」，点击复用 `AccountAuthModal` 先建账号再继续。
- 保存后写 breed（catalog），列表刷新。

---

## 3. 前端处理逻辑

相关文件：`web/src/components/settings/{MemberManagement.tsx, HubBreedEditor.tsx, breed-editor.model.ts, breed-editor.sections.tsx, breed-editor-advanced.tsx, breed-editor-fields.tsx, primitives.tsx}`、`web/src/services/{settingsService.ts, breedService.ts}`、`web/src/hooks/{useSettings.ts, useBreeds.ts, useLeaderConfig.ts}`、`web/src/store/useAppStore.ts`、`web/src/constants/clientIds.ts`。

### 3.1 导航与分区挂载

- `settings-nav-config.ts`：`RAW_SECTIONS` 仅保留 `members`（label「成员管理」）与 `accounts`（「账户与密钥」）两个分区；`DEFAULT_SECTION='members'`。
- `SettingsContent.tsx`：`SECTION_COMPONENTS = { members: MemberManagement, accounts: AccountKeys }`，均 `lazy`；直接用 `meta.label/description` 包 `SettingsPageHeader`，面板不自带页头。

### 3.2 列表主页 `MemberManagement.tsx`

- **数据来源**：`useBreeds()` 拿合并后的 `breeds`（template system + catalog runtime，来自 `/api/breeds`）；`useSettings()` 拿 `roster`（`/api/settings/roster`）。`members = breeds.map(b => breedToSettingsMember(b, roster[b.id]))`（`useMemo`，依赖 breeds+roster）。
- **`breedToSettingsMember` 投影**（核心映射）：
  - `id ← b.id`，`name←b.name`，`breed←b.display_name`，`color←b.color.primary`。
  - 取默认 variant：`variants.find(v => v.id===b.default_variant_id) ?? variants[0]`；从该 variant 取 `default_model / client_id / account_ref / provider / session_chain / cli.* / strategy / context_budget.* / mcp_support`。
  - **启用唯一真相**：`enabled = roster[id]?.available ?? b.enabled`（roster 优先）。
  - `type`：`account_ref` 命中 `OAUTH_REFS`(claude/codex/gemini/kimi/opencode) → `'CLI (OAuth)'`，否则 `'CLI (config)'`。
  - `teamStrengths`：`b.team_strengths`（逗号串）拆数组；`mentionPatterns←b.mention_patterns`（路由真相）。
- **全局默认犬**：`useEffect` 挂载时 `settingsService.getDefaultBreed()` 取 `breed_id`；选择器 `onChange→handleDefaultDogChange(id)`：若该犬 `roster[id].available` 为假先 `updateRosterEntry(id,{available:true})` 启用，再 `setDefaultBreed(id)` 落盘（`/api/config/default-breed`），失败 toast `members.saveDefaultFailed`。
- **启用/停用**：`handleToggleEnabled(m)` → `updateRosterEntry(m.id,{available:!m.enabled})`（`PATCH /api/settings/roster/{id}`，部分更新），随后 `refetchBreeds()+refetchSettings()`。
- **删除**：`handleDelete(m)` → `window.confirm(members.confirmDelete)` → `deleteBreed(m.id)`（`DELETE /api/breeds/{id}`），随后 refetch。
  > 注：删 breed 本身不会触发账号引用的 409——账号引用保护在「账户与密钥」页删账号时（`DeleteAccount` 扫 breeds 的 `variant.account_ref`）；成员页只负责删成员。
- **拖拽排序**：`onDragStart/onDragOver/onDrop/onDragEnd` + `reorderIds(enabledIds, src, target)`。落盘逻辑：乐观更新 `localMembers`（启用段按拖拽重排，停用段保持原位）→ `settingsService.setBreedOrder([...enabledIds, ...disabledIds])`（`PUT /api/config/breed-order`）→ refetch；失败则 `setLocalMembers(members)` 回滚到服务端顺序 + toast `members.reorderFailed`。**排序现在真正落盘**（旧版仅本地 state，刷新丢失）。
- **空状态 / 加载**：`breedsLoading` → loading strip；`localMembers.length===0` → `members.notFound`。

### 3.3 「添加成员」模态框 `HubBreedEditor.tsx`

`fixed inset-0 z-50` 全屏遮罩 + 居中弹层（SLATE 深色 + amber）。复用 SG 深色主题。

**两种模式**（顶部模板切换，编辑态禁用）：`isEdit = Boolean(breed)`。

- **挂载加载**：
  - `getAccounts()`（`/api/settings/accounts`）→ `profiles`（账号列表）。
  - 仅新建：`apiGet('/api/breeds/templates')` → `templates`（只读 `role_templates`）。
  - `breedService.getBreeds()` → `members`（用于 `reservedPatterns` 别名去重）。
- **`availableProfiles = filterAccounts(form.clientId, profiles)`**（D5 语义，见 §4.6）；选中账号决定 `modelOptions` 与 `authType`。
- **`reservedPatterns`**：遍历 `members` 的 `mention_patterns`，收集除自身外的小写别名集合 → 前端别名唯一性提示（后端也有守护，见 §4.4）。
- **模板选择** `handleTemplateSelect`：填 name/displayName/nickname/avatar/配色/role_description/personality + `autoSlug` 生成 `dogId` + 别名去重（与 reservedPatterns 冲突则追加数字 2..99）。
- **关闭确认**：`hasUnsavedChanges` 为真时关闭弹内确认框（SG 无全局确认 context）。

**提交校验 `handleSave`**（仅新建严格校验）：
- 必填：`name`、`roleDescription`；若所选账号 `authType==='api_key'` 还需 `defaultModel`；`mentionPatterns`（别名）至少 1 个。
- api_key 账号：`withDefaultModelMentionPattern(form)` 自动补一条 `<model>` 别名。
- 校验失败 → `setFieldErrors` + `setError('请填写必填字段：...')` 不提交。

**空账号态**：`hasEmptyCreatableAccounts = !isEdit && availableProfiles.length===0` → 渲染 CTA「新建 / 登录账号」→ 打开内嵌 `AccountAuthModal`（`onCreated(profileId)` 写 `pendingProfileIdRef` 并 `setProfilesVersion+1` 重新拉账号列表，回填 `accountRef`+首个 model）。

**提交动作**：`buildBreedPayload(effectiveForm, breed)`（见 `breed-editor.model.ts`）→ `onSave(payload)` → 由 `MemberManagement.handleSaveBreed` 调 `updateBreed(breed.id, breed)`（编辑）或 `createBreed(breed)`（新建），随后 `refetchBreeds()+refetchSettings()` + success toast。

### 3.4 前端 API 调用层

`web/src/services/settingsService.ts`（成员相关）：
- `getRoster()` → `GET /api/settings/roster`，返回 `Record<id, RosterEntry>`（空则 `{}`）。
- `updateRosterEntry(id, patch)` → `PATCH /api/settings/roster/{id}`（部分字段合并）。
- `getReviewPolicy()/updateReviewPolicy(policy)` → `GET/PUT /api/settings/review-policy`。
- `getDefaultBreed()` → `GET /api/config/default-breed`，返回 `{breed_id, is_override}`。
- `setDefaultBreed(breedId)` → `PUT /api/config/default-breed`（`{breed_id}`）。
- `getBreedOrder()/setBreedOrder(order)` → `GET/PUT /api/config/breed-order`（`{order:[...]}`）。
- `getAccounts()` → `GET /api/settings/accounts`（调 `mapAccountApiToUi`）。

`web/src/services/breedService.ts`（ breeds CRUD，方案 B 后成员走这里）：
- `getBreeds()` → `GET /api/breeds`（合并 breeds）。
- `createBreed(config)` → `POST /api/breeds`。
- `updateBreed(id, updates)` → `PATCH /api/breeds/{id}`。
- `deleteBreed(id)` → `DELETE /api/breeds/{id}`。
- `getTemplates()` → `GET /api/breeds/templates`。

`web/src/hooks/useBreeds.ts`：模块级 `breedCache` 缓存；`createBreed/updateBreed/deleteBreed` 成功后 `fetchBreeds()` 失效缓存并重设 state；错误 toast。

`web/src/hooks/useSettings.ts`：持有 `roster` state（`getRoster()`）；`fetchAll` 并行拉 `getRoster()+getAccounts()+getSystemConfig()`；返回 `{roster, accounts, config, loading, error, updateRosterEntry, addAccount, deleteAccount, refetch}`。**已移除旧 members 状态/方法**（方案 B）。

`web/src/hooks/useLeaderConfig.ts`：`leader` + `updateLeader` → `GET/PATCH /api/config/leader`。

`web/src/store/useAppStore.ts`：**已删除 `globalDefaultDog` 字段及 `setGlobalDefaultDog`**（旧版不落盘，被默认犬端点取代）。保留 `memberFilter` / `showAddMemberModal` 等 UI state。

---

## 4. 后端处理逻辑

### 4.1 成员 = breed（方案 B 核心）

成员列表 ↔ `/api/breeds`（合并注册表：`pl.Breeds` = 模板 system breeds + catalog 运行时 breeds）。运行时名册 ↔ `/api/settings/roster`（catalog `roster{}`）。两者在同一 `dog-catalog.json`，由 `FileSettingsStore` 管理。

注册表装配（`internal/platform`）：`platform.New` 从 `dog-template.json`（种子 system）+ `FileSettingsStore.ListBreeds()`（catalog runtime）bake 出 `pl.Breeds`；`SetupPack` 加载 + `WatchBreedsFile` 热加载（变更经事件总线/`HotReloader` ~30s 防抖）。

### 4.2 pack-api 路由（`internal/packapi/handler.go`，挂 `/api/breeds`，**已**包 auth.Wrap）

- `GET /api/breeds`（`ListBreeds`）→ `h.pack.List()` 返回合并 breeds。
- `POST /api/breeds`（`CreateBreed`）→ 解码 `BreedConfig`；`Source` 空则补 `user`；`validateBreed` + `pack.Validate`；`pack.Register`；`persistBreed`（写 catalog）；任一失败回滚（Unregister / 500）。→ 200 `breed`。
- `PATCH /api/breeds/{id}`（`UpdateBreed`）→ 取现有 breed；**`source==system` → 403 `system breeds cannot be modified`**；逐字段部分解码（`display_name/avatar/personality/role_description/team_strengths/mention_patterns/roles/variants`）；`validateBreed` + `pack.Validate` + `persistBreed`；成功 `eventBus.Emit`。→ 200 `breed`。
- `DELETE /api/breeds/{id}`（`DeleteBreed`）→ `pack.Unregister`（system 403）；`removeBreed`（catalog 删 breed + 删对应 roster entry + 从 breedOrder 移除）；→ 200。
- `GET /api/breeds/templates`（`GetTemplates`）→ 返回 `dog-template.json` 的 `role_templates`（**只读种子**）。

**`validateBreed(cfg, excludeID)`**（集中守护）：
- `pack.CheckMentionPatternsUnique(pack.List(), cfg.MentionPatterns, excludeID)` → 别名（mention_patterns）全局唯一，冲突报 `alias %q is already used by member %q`（400）。
- 每个 variant：`settings.ValidateClientID(v.ClientID)` → 不在白名单 400。
- 每个 variant：`settings.ValidateAccountRef(h.store, v.AccountRef)` → 不存在 400。

### 4.3 settings 路由（`internal/transport/settings_handler.go`，挂 `/api/settings/`，**包 auth.Wrap**）

- `GET /api/settings/roster`（`ListRoster`）→ `store.GetRoster()` → `map[id]RosterEntry`。
- `GET /api/settings/roster/{id}`（`GetRosterEntry`）→ 命中返回 entry，否则 404 `roster entry not found`。
- `PATCH /api/settings/roster/{id}`（`UpdateRosterEntry`）→ **从现有 entry 起算部分更新**（未传字段保留）；`store.UpdateRosterEntry` 若 `ErrBreedNotFound` → 404 `breed not found`，否则 200。
- `GET /api/settings/review-policy` / `PUT /api/settings/review-policy`（`GetReviewPolicy`/`SetReviewPolicy`）→ pack.ReviewPolicy 读写。
- `DELETE /api/settings/accounts/{id}`（`DeleteAccount`，引用完整性）→ 见 4.5。

### 4.4 config 路由（`internal/transport/config_handler.go`，挂 `/api/config/`，**已**包 auth.Wrap）

- `GET /api/config/default-breed`（`GetDefaultBreed`）→ 优先 `DEFAULT_BREED_ID` 环境变量（`is_override=true`），否则读 `configs["default_breed"]`；返回 `{breed_id, is_override}`。
- `PUT /api/config/default-breed`（`SetDefaultBreed`）→ 非空时 `knownBreedIDs()`（模板 union catalog）校验，未知 → 404；落 `UpdateConfig("default_breed", id)`（持久化）；env override 在读取时仍优先。**（修复旧版只 `os.Setenv` 不持久化）**。
- `GET /api/config/breed-order`（`GetBreedOrder`）→ 返回 catalog `breeds[]` 顺序（空则回退模板顺序）；**排序真相 = breeds[] 数组顺序**（clowder 同构）。
- `PUT /api/config/breed-order`（`SetBreedOrder`）→ `knownBreedIDs()` 校验未知 id（返回 `unknown breed IDs` + `missing` 列表 400）；`store.ReorderBreeds(order)` 重排 catalog `breeds[]`。（**修复旧版写 `configs["breed_order"]` 而该 key 不在 defaultConfig → 实际 500 的坏逻辑**）。
- `GET/PATCH /api/config/leader`（`GetLeader`/`UpdateLeader`）→ pack.Leader 读写（catalog 持久化）。

### 4.5 删除账号的引用完整性（反方向保护）

`DeleteAccount`（`settings_handler.go`，已含 F3/F6）：删账号前扫 `store.ListBreeds()` 各 variant 的 `account_ref` 是否命中该 id；有绑定且 query 非 `force=true` → **409 `{error, bound_member_ids}`**。这保证「成员绑定的账号」不能静默被删（成员侧只展示引用，不自动级联）。

### 4.6 白名单与 account_ref 校验（D5）

`internal/settings/validation.go`：
- `ValidCLIClientIDs = {claude, codex, gemini, opencode, kimi}`；`OAuthClientRefs` 同值。
- `ValidateClientID(id)`：空 → true（generic api_key 账号，不限厂商、不设白名单）；非空必须命中白名单，否则 false。
- `ValidateAccountRef(store, ref)`：空 → 放行（解绑）；命中 `OAuthClientRefs` → 放行（内置 CLI OAuth，非 catalog 账号）；否则必须命中 `store.ListAccounts()` 的 id，否则 400。**该校验复用 handler 持有的同一 `store` 实例**，绝不重新解析数据根 → 规避 clowder #1303「写 A 根、校验读 B 根」分裂。

前端 `filterAccounts(clientId, profiles)`（D5 前端语义）：`clientId` 为空 → 返回全部账号（含无 clientId 的通用 api_key 账号）；否则返回 `profile.clientId===clientId || !profile.clientId`（匹配或通用账号均可见）。

---

## 5. 存储与数据模型

### 5.1 catalog 唯一真相（clowder 同构）

`.sounds-great-ai/dog-catalog.json`（0644，由 `FileSettingsStore` 管理，`MarshalIndent` + tmp+rename 原子写）：

```json
{
  "version": 2,
  "breeds": [ BreedConfig, ... ],     // 成员全量（含 source/enabled）
  "breedOrder": [ "id", ... ],        // 排序真相（数组顺序）
  "roster": { "id": RosterEntry },    // 运行时名册（available/roles/lead/family/evaluation）
  "review_policy": { ... },
  "leader": { ... },
  "configs": [ {key,value,category}, ... ]  // default_breed 等
}
```

`packs/default/breeds/dog-template.json`（0644，**只读种子**）：`version:2 + role_templates + client_defaults + leader + breeds[]`（默认 6 只 system 犬）；首启 `seedCatalogIfEmpty` 把模板 breeds 复制进 catalog（catalog 成为全量真相），删 catalog 成员不复活模板。

### 5.2 三文件隔离（与 accounts 同根）

| 文件 | 内容 | 权限 | 管理方 |
|---|---|---|---|
| `dog-catalog.json` | breeds + breedOrder + roster + review_policy + leader + configs | 0644 | `FileSettingsStore` |
| `accounts.json` | 账号元数据 | 0644 | `FileSettingsStore` |
| `credentials.json` | 密钥（明文） | 0600 | `FileCredentialStore` |

`settings.ConfigRoot(projectRoot)` 顺序：`SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT`（支持 `~`）→ `{projectRoot}/.sounds-great-ai` → `{home}/.sounds-great-ai`。`routes.go` 只算一次 `settingsDir`，三文件均同一目录（同 §5.2 在账号 story 已述，此处复用：SG 不踩 #1303 的根因）。

### 5.3 关键结构体（`pkg/pack/breed.go`）

- `BreedConfig`：`ID, Name, DisplayName, Nickname, Avatar, Color{Primary,Secondary}, Personality, RoleDescription, TeamStrengths, MentionPatterns[], Roles[], Caution, DefaultVariantID, Variants[], Review, Source(system/user/plugin), Enabled`。
- `Variant`：`ID, VariantLabel, ClientID, DefaultModel, MCPSupport, CLI{Command,OutputFormat,DefaultArgs[],Effort}, SystemPrompt, Personality, Strengths[], TeamStrengths, Caution, ContextBudget{MaxPromptTokens,MaxContextTokens,MaxMessages}, VoiceConfig, AccountRef, Provider, SessionChain, Strategy, AutoCompactTokenLimit`。← 成员绑定账号走 `variant.account_ref`。
- `RosterEntry`：`Family, Roles[], Lead, Available, Evaluation`。← `Available` 为启用唯一真相。
- `ReviewPolicy`：`RequireDifferentBreed, PreferActiveInThread, ExcludeUnavailable, PreferredRoles[]`。
- `Leader`：`Name, Nickname, Avatar, TimeZone, Aliases[], MentionPatterns[], ColorPrimary, ColorSecondary`。

### 5.4 损坏自动备份（F4）

`file_store.go` `reloadFromDisk` 解析 catalog/accounts/credentials 失败时 `backupCorrupt` 复制为 `.bak`（0o644）后当空处理，不再 500。

---

## 6. 端到端数据流

### 6.1 新建成员（含绑定账号）

```
[前端] HubBreedEditor.handleSave(form)
   → buildBreedPayload → onSave(breed)
   → MemberManagement.handleSaveBreed → useBreeds.createBreed(breed)
   → POST /api/breeds  (BreedConfig, source=user)
        │
[后端] auth.Wrap?(是，/api/breeds 已包 → AUTH_TOKEN 模式强制鉴权) → CreateBreed
   → validateBreed: CheckMentionPatternsUnique（别名唯一）
                  + 各 variant ValidateClientID（白名单或空）
                  + 各 variant ValidateAccountRef（空/cli-oauth/账号存在）
   → pack.Register → persistBreed（写 dog-catalog.json breeds[] + breedOrder 追加 + roster 初值 Available=Enabled）
   → 200 breed
        │
[前端] refetchBreeds + refetchSettings → 列表出现新成员
```

若所选 client 下无账号：前端空状态 CTA → `AccountAuthModal` 先 `POST /api/settings/accounts`（带 apiKey 落 credentials.json，F1）→ `onCreated` 回填 `accountRef` + 首个 model → 再保存成员。

### 6.2 启用/停用成员

```
[前端] MemberOverviewCard 开关 → handleToggleEnabled(m)
   → settingsService.updateRosterEntry(m.id, {available: !m.enabled})
   → PATCH /api/settings/roster/{id}  (部分更新)
        │
[后端] UpdateRosterEntry → store.UpdateRosterEntry（breed 不存在 → ErrBreedNotFound → 404）
   → flushCatalog（写 dog-catalog.json roster）
   → 200
        │
[前端] refetch → 启用状态持久化（刷新不丢）
```

### 6.3 拖拽排序

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
[前端] refetch → 顺序持久化；失败则回滚到服务端顺序 + toast
```

### 6.4 设默认犬

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

## 7. 技术契约与接口设计 (Technical Contract)

### 7.1 接口 Schema（成员相关）

```
GET    /api/breeds                       → 200 BreedConfig[]            (合并 breeds)
POST   /api/breeds                       → 200 BreedConfig             (创建/更新 breed)
PATCH  /api/breeds/{id}                  → 200 BreedConfig | 403(system) | 404
DELETE /api/breeds/{id}                  → 200 nil | 403(system)
GET    /api/breeds/templates             → 200 RoleTemplate[]

GET    /api/settings/roster              → 200 map[id]RosterEntry
GET    /api/settings/roster/{id}         → 200 RosterEntry | 404
PATCH  /api/settings/roster/{id}         → 200 nil | 404

GET    /api/settings/review-policy       → 200 ReviewPolicy
PUT    /api/settings/review-policy       → 200 nil

GET    /api/config/default-breed         → 200 {breed_id, is_override}
PUT    /api/config/default-breed         → 200 {breed_id} | 404(未知)
GET    /api/config/breed-order           → 200 {order: string[]}
PUT    /api/config/breed-order           → 200 {order} | 400(未知 id)
GET    /api/config/leader                → 200 Leader
PATCH  /api/config/leader                → 200 nil

DELETE /api/settings/accounts/{id}       → 200 nil | 409(bound_member_ids) | 404
```

### 7.2 校验错误码

| 场景 | 端点 | 响应 |
|---|---|---|
| 别名重复 | POST/PATCH /api/breeds | 400 `alias %q is already used by member %q` |
| client_id 非法 | POST/PATCH /api/breeds | 400 `invalid client_id %q; allowed: claude, codex, gemini, opencode, kimi` |
| account_ref 不存在 | POST/PATCH /api/breeds | 400 `account_ref %q not found` |
| 编辑 system breed | PATCH /api/breeds/{id} | 403 `system breeds cannot be modified` |
| roster 指向未知 breed | PATCH /api/settings/roster/{id} | 404 `breed not found` |
| 默认/排序指向未知 breed | PUT /api/config/default-breed / breed-order | 404 / 400 `unknown breed IDs` |
| 删被绑定账号 | DELETE /api/settings/accounts/{id} | 409 `{error, bound_member_ids}` |

### 7.3 数据库/文件变动

无数据库；成员全部落 `dog-catalog.json`（同 `ConfigRoot` 目录）。详见 §5。

---

## 8. 验收标准 (Acceptance Criteria - AC)

- [x] **AC-01 (正常路径)**:
  - Given 已进入成员管理分区，When 点「+ 添加成员」填名称/角色描述/选账号+模型+别名并保存，Then 200 且 catalog `breeds[]` 新增该 breed、列表出现、刷新仍在。
  - Given 已在列表，When 拖拽启用段成员到新位置，Then `breed-order` 持久化、刷新顺序保持。
  - Given 已在列表，When 切全局默认犬选择器，Then `configs["default_breed"]` 落盘、刷新默认犬不变。
  - Given 已在列表，When 拨动启用/停用开关，Then `roster[id].available` 落盘、刷新状态保持。
- [x] **AC-02 (异常与边界)**:
  - When 新建成员别名与已有成员冲突，Then 后端 400 `alias ... already used`（前端 `reservedPatterns` 也提示）。
  - When 成员 variant `client_id` 不在白名单且非空，Then 400。
  - When 成员绑定一个不存在的 `account_ref`（非内置 OAuth），Then 400 `account_ref ... not found`。
  - When 编辑默认 system 犬，Then 403 `system breeds cannot be modified`。
  - When 设置默认犬 / 排序传入未知 breed id，Then 404 / 400 `unknown breed IDs`。
  - When roster 指向不存在的 breed，Then PATCH 返回 404。
  - When 配置文件损坏，Then 自动备份 `.bak` 且当空处理，不 500（F4）。
  - When 拖拽排序保存失败，Then 列表回滚到服务端顺序 + `members.reorderFailed` toast。
- [x] **AC-03 (权限与安全)**:
  - 生产环境设 `AUTH_TOKEN` 后，When 未带 `Authorization` 访问 `/api/settings/*`（roster/review-policy/accounts），Then 返回 401（auth.Wrap）。
  - ✅ **已闭环（2026-08-12 补）**：`/api/breeds`（成员增删改）与 `/api/config/`（默认犬/排序/leader）已在 `cmd/server/routes.go` 包 `auth.Wrap`（`mux.Handle("/api/breeds", auth.Wrap(packAPI.Routes()))`、`mux.Handle("/api/config/", auth.Wrap(configHandler.Routes()))`）。至此成员的创建/编辑/删除、设默认犬、改排序、改大当家这些写操作与生产 `AUTH_TOKEN` 模式下的「账户与密钥」页（`/api/settings/accounts`）**一致受 AuthMiddleware 保护**。`auth.Wrap` 在 `AUTH_TOKEN` 未设时为 no-op，开发与测试无感。
  - 密钥隔离：`credentials.json` 0600，列表响应仅回 `key_preview`；成员绑定只存 `account_ref`（账号 id）不存密钥。
  - envVars 拒绝 `SOUNDS_GREAT_AI_` 前缀（账号侧 F5，成员经账号复用）。

---

## 9. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- **[x] 资损与网络安全 (Security)**:
  - 敏感数据脱敏: 是（密钥仅存 `credentials.json` 0600；成员只引用 `account_ref`）。
  - 防重提交/幂等: `DELETE /api/breeds/{id}` 经 `removeBreed` 吞 `ErrBreedNotFound` → 删除模板种子犬时幂等（不报 500）。
  - ✅ **鉴权已统一**：`/api/breeds` 与 `/api/config/` 已包 `auth.Wrap`（详见 AC-03），与「账户与密钥」页一致，不再存在相对缺口。
- **[x] 高并发与限流降级**: 普通（本地配置链路）。`FileSettingsStore` 用 `sync.RWMutex` 保护；写路径 tmp+rename 原子写；外部改动由 `HotReloader` ~30s 防抖热加载。
- **[x] 可服务性与监控**: 无独立 TraceID；关键失败返回结构化 error。成员写链路（breeds/config）现已与 accounts 一致接入 `auth.Wrap` 鉴权（401 拦截）。

---

## 10. Story 级 Definition of Done (DoD Checklist)

- [x] 3-Corner 澄清通过（方案 B 决策 + 本次现状梳理，AC 已锁定）。
- [x] 单元测试覆盖：packapi 包（Create/Update/Delete breed、别名唯一、client_id、account_ref、system 403、ErrBreedNotFound）；transport 包（TestRosterEndpoints、TestReviewPolicyEndpoints、TestSetBreedOrderReordersCatalog、TestSetDefaultBreedValidatesMergedBreeds、TestDeleteAccount_ReferentialIntegrity）。
- [x] 静态/构建门禁：`go build ./...` 0、`go test ./...` 0、`cd web && tsc -b` 0、`vite build` ✓。
- [x] 前端三旧坑已消除：globalDefaultDog 不落盘（→ default-breed 端点）、拖拽不落盘（→ breed-order 端点）、双数据源（→ 合并 breeds 同构）。
- [x] 鉴权一致性：已完成 `/api/breeds` 与 `/api/config/` 补 `auth.Wrap`，与生产 `AUTH_TOKEN` 模式下 accounts 页一致（2026-08-12 实施，`go build ./...` + `go test ./...` 全绿）。

---

## 11. 修订记录 (Revision History)

- **2026-08-12（初版）**：基于方案 B 落地后代码实况梳理成员管理 + 添加成员前后端逻辑，记录 `routes.go` 中 `/api/breeds`、`/api/config/` 未包 `auth.Wrap` 的鉴权一致性缺口（AC-03 / §9）。
- **2026-08-12（补丁）**：用户确认后于 `cmd/server/routes.go:81-82,124` 为 `/api/breeds`、`/api/breeds/`、`/api/config/` 补 `auth.Wrap`，关闭 AC-03 鉴权缺口；`go build ./...` + `go test ./...` 全绿（cmd/server 与全模块均 ok）。本文档 §4.2/§4.4、数据流注释、AC-03、§9、DoD 同步更新为已闭环。

---

> 关联文档：`sg-member-catalog-plan.md`（方案 B 规划与 D1–D6 决策）、`SG-ACC-001-accounts-keys-auth.md`（同分区「账户与密钥」设计，成员经 `account_ref` 反向引用）。
> 对照样本：`readonly-docs/clowder-ai`（Fastify + `cat-catalog.json` V2，clowder 同构来源）。
> 历史备注：旧版「成员管理」基于扁平 `Member` + `/api/settings/members`，该端点在方案 B 阶段 3 已删除；本文档仅描述当前 breed 同构实现。
