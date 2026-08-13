# [SG-ACC-001] [Tech Story] 设置页「账户与密钥」与「客户配置安全」设计

> 本文件由 `SG-ACC-001-accounts-keys-auth.md`（账户与密钥前后端）与 `SG-OPS-001-customer-config-safety.md`（客户配置安全）合并而成。
> 内容按 `sounds-great-ai` **2026-08-13 代码实况**逐文件重新梳理：前端 `web/src`、后端 `internal/transport/settings_handler.go` + `internal/settings`、装配 `cmd/server/routes.go`、存储 `internal/settings/{file_store.go,credential.go,port.go,validation.go}`。
> 相对两份旧文档，已修正以下与代码不符之处：
> - `settings-nav-config.ts` 实际 `DEFAULT_SECTION = 'accounts'`（旧文档误写为 `'members'`）；`RAW_SECTIONS` 顺序为 `accounts` 在前、`members` 在后。
> - `routes.go` 中 `/api/settings/` 的 `auth.Wrap` 在 `BuildMuxWithHandler` 内位于 `/api/breeds`、`/api/config/` 之后，凭证库 `credStore` 用 `settings.CredentialRoot()`（全局 home）。
> - `dog-catalog.json` 实际含 `deleted_breeds` 与 `seen_template_breeds` 两个集合字段（客户配置安全的「升级追加同步」与成员「首启空」共用 `seen_template_breeds`）。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu | QA: @demu
- **故事点/复杂度**: [ M (3-5分) ]
- **范围**: 设置页 **`accounts` 分区**（LLM 账号的 OAuth / API Key 认证、元数据与密钥的分离落盘、引用完整性）+ **客户配置安全**（凭证独立根 `CredentialRoot`、编辑时时间戳备份、升级追加式同步）。
- **业务/技术目标**:
  - As a **平台使用者（operator）**,
  - I want to **在设置页统一管理 LLM 账号（OAuth / API Key），安全可靠地新增/编辑/删除账号，且让成员正确绑定账号；同时客户部署时密钥与项目配置物理隔离、编辑可回滚、升级不丢既有成员**,
  - So that **账号元数据与密钥分离落盘、密钥真正持久化、成员绑定不产生悬空引用、清项目配置不误删密钥、且未授权无法篡改配置**。
- **关键指标/埋点**: 无（内部配置管理链路，非对外曝光功能）。

---

## 2. 功能叙事 (User Journey)

「账户与密钥」是 **Settings 内的一个分区**（nav id=`accounts`）。设置页默认进入此分区（`DEFAULT_SECTION = 'accounts'`），左侧导航并列 `members`（成员管理）。左侧点「账户与密钥」→ 右侧渲染 `AccountKeys` 列表（含内置 OAuth 账号 + 自定义账号）。

点右上角「+ 新增账户认证」→ 弹出 **`AccountAuthModal`**（Portal 挂到 `document.body`）。这是一个**模态框而非独立页面**，承载「新增」与「编辑」两种模式：

- 顶部切换 **OAuth / API Key** 两种认证形态（编辑态锁定，不可切换）。
- OAuth：从 `CLIENT_IDS` 下拉选 `claude / codex / gemini / opencode / kimi`。
- API Key：填 `Base URL` + `API Key` + 至少 1 个模型。
- 高级配置里可注入自定义环境变量（envVars），用于启动 agent 子进程时注入。

保存后列表刷新，显示「已配密钥」徽章。删除一个被成员绑定的账号时，后端返回 409，前端二次确认后可强制删除。

---

## 3. 前端处理逻辑

相关文件：`web/src/components/settings/{settings-nav-config.ts, SettingsContent.tsx, AccountKeys.tsx, AccountAuthModal.tsx}`、`web/src/services/settingsService.ts`、`web/src/hooks/useSettings.ts`、`web/src/constants/clientIds.ts`。

### 3.1 导航与分区挂载

- `settings-nav-config.ts`：`RAW_SECTIONS` 仅保留 `accounts`（label「账户与密钥」，icon `fa-key`，indigo）与 `members`（「成员管理」）两个分区，**顺序为 accounts 在前、members 在后**；`DEFAULT_SECTION = 'accounts'`（设置页默认进账户与密钥）。
- `SettingsContent.tsx`：`SECTION_COMPONENTS = { members: MemberManagement, accounts: AccountKeys }`，均 `lazy` 懒加载；直接用 `meta.label/description` 包 `SettingsPageHeader`，面板自身不再渲染页头。

### 3.2 账户与密钥主页 `AccountKeys.tsx`

- **加载**：`useSettings` 在 mount 时 `fetchAll()` 并行拉 `members/accounts/config`，写入各自 state。
- **分组展示**：`useMemo` 拆成 `builtinAccounts`（`a.builtin` 在前）与 `customAccounts`（在后），分别用 `SettingsRow` 卡片渲染。`accountMeta(acc)` 摘要：oauth 显示 clientId；api_key 显示 host（去协议头/尾斜杠）+ 是否已配置 + 模型数。徽章：builtin→blue「已配密钥」；custom→ oauth=amber / api_key=purple。
- **新增**：`openCreate()` → `setEditTarget(undefined); setAuthModalOpen(true)`。
- **编辑**：`openEdit(acc)` 构造 `UnifiedAuthEditData { id, displayName, baseUrl, clientId, authType, models, envVars }` 打开同一模态框。
- **删除**：`handleDeleteAccount(id)` → `deleteAccount(id)` 失败且 `ApiError.status===409` → `window.confirm(t('accounts.confirmDeleteBound'))` 二次确认 → `deleteAccount(id, true)` 强制删除；非 409 失败 → toast 报错。
- **刷新机制**：每次成功增删改后 `dispatchAccountsChanged()`（派发 `accounts-changed` 事件）+ `await refetch()` + success toast。

### 3.3 「新增账号认证」模态框 `AccountAuthModal.tsx`

`createPortal(..., document.body)`，复用 SG 深色主题（slate-900/950 + amber 主色）。

**两种模式**（顶部切换，编辑态 disabled）：
| 字段 | OAuth | API Key | 说明 |
|---|---|---|---|
| 账号名称 displayName | 必填 | 必填 | 始终显示 |
| Client 下拉 | ✅ | 向导锁定时才显示 | `CLIENT_IDS` 枚举约束 |
| Base URL | ❌ | ✅ | placeholder `https://api.openai.com/v1` |
| API Key | ❌ | ✅（编辑态留空=不变） | `type=password` |
| 可用模型 models | 可选（有推荐按钮） | **至少 1 个** | `TagEditor` 标签输入 |
| 高级配置 envVars | 可选 | 可选 | 折叠面板，注入子进程环境变量 |

- **模型推荐**：`MODEL_SUGGESTIONS`（按 client 预置），点击即加入。
- **envVars 校验**：`ENV_KEY_RE = /^[A-Z_][A-Za-z0-9_]*$/`；并拒绝系统保留前缀 `RESERVED_ENV_PREFIX = 'SOUNDS_GREAT_AI_'`（前端，与后端过滤对齐）。非法 key 标红，`buildEnvVars()` 过滤空/非法/保留前缀 key。
- **重水合**：`useEffect` 监听 `open` 上升沿，重新灌入 `editProfile` 数据，避免同 key 复用脏数据。

**提交校验 `canSubmit`**：
- OAuth：`displayName.trim()` 非空。
- API Key：`displayName` 非空 **且** `models.length>0` **且**（编辑态 **或** `baseUrl`+`apiKey` 非空）。
- 即**新建 API Key 账号必须同时给 baseUrl、apiKey、≥1 模型**。

**提交动作 `handleSubmit`**（错误显示在模态框内，不关闭）：
- **编辑（isEdit）** → `settingsService.updateAccount(id, patch, apiKey)`：`patch={displayName, models, envVars, clientId?, baseUrl?}`，`apiKey` 仅当非空作为第三参传入（留空不传=保持）。
- **新建 OAuth** → `addAccountFull({...type:'oauth', authType:'oauth', mode:'subscription'...})`，**不带 apiKey**。
- **新建 API Key** → `addAccountFull({...type:'api_key', authType:'api_key', mode:'api_key', baseUrl, models, envVars...}, apiKey)`，apiKey 作第二参。

### 3.4 前端 API 调用层

`web/src/services/settingsService.ts`：
- `getAccounts()` → `GET /api/settings/accounts`，`mapAccountApiToUi`（`hasApiKey ← key_set`）。
- `addAccountFull(account, apiKey?)` → `POST`，驼峰转 snake_case（`client_id/base_url/models/model_aliases/env_vars/auth_type/mode/builtin`），`apiKey` 作为顶层 `api_key`。
- `updateAccount(id, updates, apiKey?)` → `PATCH /api/settings/accounts/{id}`，逐字段转 snake_case；`apiKey` 仅非空时带 `api_key`。
- `deleteAccount(id, {force})` → `DELETE`，`force` 时拼 `?force=true`。

`web/src/hooks/useSettings.ts`：`deleteAccount` 成功后从本地 state 过滤；异常 re-throw 让 `AccountKeys` 处理 409。`web/src/services/http.ts` 所有请求注入 `Authorization` 头，使后端 `auth.Wrap` 在生产（`AUTH_TOKEN` 已设）下正常工作。

---

## 4. 后端处理逻辑

文件：`internal/transport/settings_handler.go`（其 `Routes()` 挂载于 `/api/settings/`，由 `routes.go` 用 `auth.Wrap` 包裹）。

白名单：`validClientIDs = {claude, codex, gemini, opencode, kimi}`（`settings.ValidateClientID` 校验；非空且不在白名单 → 400）。`OAuthClientRefs` 与 `validClientIDs` 值一致，用于「成员 account_ref 是否指向内置 OAuth（无需查 catalog）」判断（`settings.ValidateAccountRef`）。

### 4.1 路由注册与鉴权

- `routes.go`：`mux.Handle("/api/settings/", auth.Wrap(settingsHandler.Routes()))`（位于 `/api/breeds`、`/api/config/` 之后）。
- `AuthMiddleware` 由 `AUTH_TOKEN` 环境变量驱动（Bearer / `X-Auth-Token` 校验），未设 token 时自动禁用（开发模式零影响）。前端 `http.ts` 已统一注入 `Authorization`，无联动缺口。

### 4.2 GET /api/settings/accounts（`ListAccounts`）

`h.store.ListAccounts()` 全量 `[]*Account` → `respondJSON(200)`。列表**不回传明文密钥**，只回 `key_preview`（脱敏）与 `key_set` 布尔。

### 4.3 POST /api/settings/accounts（`CreateAccount`）

```
1. 解析 raw map；client_id 非法 → 400
2. provider = raw["provider"]; apiKey = raw["api_key"]
3. account = store.CreateAccount(provider, apiKey)
      → 新建 Account{ID, Provider, KeyPreview:mask(apiKey), KeySet:apiKey!="", ...}
      → flushAccounts() 写 accounts.json (0644)
4. 若 apiKey != "" 且 credStore != nil：
        credStore.Set(account.ID, apiKey)
        失败 → store.DeleteAccount(account.ID) 回滚 + 500
5. 从 raw 删除 "provider"/"api_key"，其余扩展字段经 store.UpdateAccount 应用
6. respondJSON(201, account)
```

**关键**：新建带 apiKey 的账号时，明文**会进 `credentials.json`**（0600），且密钥写失败会回滚账号并 500，杜绝「假已配密钥」状态。

### 4.4 PATCH /api/settings/accounts/{id}（`UpdateAccount`）

```
1. 解析 updates；client_id 非法 → 400
2. 若含 "api_key"：
      s==""   → credStore.Delete(id)   // 清密钥
      非空    → credStore.Set(id, s)    // 写密钥
      任一失败 → 返回 500（不静默吞掉），且不改元数据，避免"元数据改了密钥没改"
   删掉 updates["api_key"]
3. store.UpdateAccount(id, updates)   // 覆盖显式字段；不存在 → 404
4. emitEvent("account-config","key",[id])  // 事件总线 → 热加载
5. respondJSON(200, nil)
```

### 4.5 DELETE /api/settings/accounts/{id}（`DeleteAccount`）

```
1. 若 query "force" != "true"：
      breeds = store.ListBreeds()
      遍历每个 breed 的每个 variant，若 v.AccountRef == id → bound 收集该 breed.ID
      if len(bound)>0 → 409 {error, bound_member_ids}   // 引用保护
2. store.DeleteAccount(id)   // 不存在 → 404
3. credStore != nil → credStore.Delete(id)  // best-effort 清密钥
4. emitEvent(...); respondJSON(200, nil)
```

> SG **无「全局共享存储时非 force 一律 409」分支**：SG 单数据根，引用检查只看当前 store 成员列表。

### 4.6 成员保存 `account_ref` 校验

`settings.ValidateAccountRef(store, ref)`（`internal/settings/validation.go`）：
- 空 → 放行（解绑）。
- 命中 `OAuthClientRefs`（claude/codex/gemini/opencode/kimi）→ 放行（指内置 CLI OAuth，非 catalog 账号）。
- 否则 → `h.store.ListAccounts()` 命中 id 才放行，否则 **400 `account_ref "x" not found`**。

`CreateMember` / `UpdateMember`（packapi `validateBreed`）在入口对 `account_ref` 做该校验。**该校验复用 handler 持有的同一个 `h.store`（账号写入用的同一实例）**，绝不在路径维度重新解析数据根——这是规避「账号写 A 根、校验读 B 根」数据根分裂的关键设计。

---

## 5. 存储与数据模型 + 客户配置安全

文件：`internal/settings/{file_store.go, credential.go, port.go}`、`internal/platform/breeds_merge.go`。

### 5.1 三文件隔离（物理分离）

| 文件 | 内容 | 权限 | 管理方 | 落盘根 |
|---|---|---|---|---|
| `accounts.json` | 账号元数据 | 0644 | `FileSettingsStore` | `ConfigRoot`（项目 `.sounds-great-ai`） |
| `dog-catalog.json` | 成员 + leader + 系统配置 | 0644 | `FileSettingsStore` | `ConfigRoot`（项目 `.sounds-great-ai`） |
| `credentials.json` | 密钥（明文） | **0600** | `FileCredentialStore` | `CredentialRoot`（**全局 home `~/.sounds-great-ai`**，独立于项目根） |

密钥库与元数据文件物理隔离、权限不同；`maskKey` 仅在 list 响应里脱敏，密钥明文只存在于 `credentials.json`。**根目录已拆分（客户配置安全）**：`catalog/accounts` 走 `ConfigRoot(projectRoot)`（项目下 `.sounds-great-ai`）；`credentials` 单独走 `CredentialRoot()`（全局 home `~/.sounds-great-ai`，可被 `SOUNDS_GREAT_AI_CREDENTIAL_ROOT` 覆盖）。清项目配置**不会**误删密钥。

### 5.2 数据根解析（双根：ConfigRoot + CredentialRoot）

- `settings.ConfigRoot(projectRoot)`（catalog/accounts）顺序：
  1. `SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT`（支持 `~` 跨平台展开）→
  2. `{projectRoot}/.sounds-great-ai` →
  3. `{home}/.sounds-great-ai`。
- `settings.CredentialRoot()`（credentials，**仅密钥**）顺序：
  1. `SOUNDS_GREAT_AI_CREDENTIAL_ROOT`（支持 `~` 跨平台展开）→
  2. `{home}/.sounds-great-ai`（全局 home，与项目根解耦；**不回退到项目根**）。

`routes.go` 的 `credStore` 用 `CredentialRoot()`；`routes.go` 的 `settingsStore`（`NewFileSettingsStore`）用 `ConfigRoot(workspaceDir)`。账号/成员/密钥不再强制同目录——密钥独立到全局 home，清项目配置不丢密钥。

### 5.3 Account 结构体（`internal/settings/port.go`）

```go
type Account struct {
    ID, Provider, KeyPreview string
    KeySet     bool
    UpdatedAt  int64
    Name, ClientID, DisplayName, BaseURL string
    Models []string
    ModelAliases, EnvVars map[string]string
    AuthType, Mode string
    Builtin bool
}
```
`maskKey(key)`：`len<=4` → `"****"`；否则 `key[:2] + "****" + key[len-2:]`。

### 5.4 客户配置安全三改动

本分区与客户部署安全强相关，三项决策如下（详情见各子节；其中「升级追加同步」作用于成员 catalog，权威描述在《成员管理》设计 §7.3）。

**(a) 凭证独立根（§5.1 / §5.2）** —— `credentials.json` 放在全局 home，与 catalog/accounts 物理隔离；清项目配置不丢密钥。

**(b) 编辑时时间戳 .bak（§5.5）** —— 写盘前生成备份；加载期损坏仅告警当空，不自动备份。

**(c) 升级追加同步（§5.6，作用于成员 catalog）** —— 新模板犬**追加**到老客户 catalog（老客户升级后能看到新预置犬），但客户删过的犬不复活。机制为 `settings.ListSeenTemplateBreeds()/AddSeenTemplateBreeds()` 与 `catalogDocument.SeenTemplateBreeds`（由 `internal/platform/breeds_merge.go` 的 `SyncTemplateBreeds` 驱动）。该集合同时支撑「首启空 catalog」（详见《成员管理》设计 §7）。

### 5.5 损坏处理与编辑时备份

- **加载时损坏**：`reloadFromDisk`（`file_store.go` / `credential.go`）解析 JSON 失败 → **仅告警并当空处理**（`log.Printf("WARN: ... is corrupt; treating as empty (no backup written at load)")`），不再自动备份（不掩盖损坏）。
- **编辑时备份**：每次写盘（`flushCatalog` / `flushAccounts` / credential `flush`）覆盖前，若目标文件已存在，先快照为 `<path>.bak-<YYYYMMDD-HHMMSS>`（如 `dog-catalog.json.bak-20260813-182600`），保留最近 5 份（`pruneBackups`），旧的不限量清理。
- 三种文件（accounts / dog-catalog / credentials）均覆盖。

### 5.6 env_vars 保留前缀过滤

后端 `UpdateAccount` 写入 `env_vars` 时，过滤掉 `SOUNDS_GREAT_AI_` 前缀的 key；前端 `AccountAuthModal` 用 `RESERVED_ENV_PREFIX` 同前缀拒绝并标红。前后端双重防护，避免用户注入覆盖运行时系统变量。

---

## 6. 端到端数据流（新建 API Key 账号）

```
[前端] AccountAuthModal.handleSubmit(api_key 模式: baseUrl+apiKey+models 已填)
   → settingsService.addAccountFull({...api_key 字段}, apiKey)
   → POST /api/settings/accounts
        body: {name, provider, client_id?, display_name, base_url,
               models, env_vars, auth_type, mode, builtin, api_key}
        │
[后端] auth.Wrap → CreateAccount
   → validateClientID(client_id)                       // 非法 400
   → store.CreateAccount(provider, apiKey)
        → Account{ID, Provider, KeyPreview:mask(apiKey), KeySet:true}
        → flushAccounts() 写 accounts.json (0644)
   → 若 apiKey != "" 且 credStore != nil → credStore.Set(account.ID, apiKey)  // credentials.json (0600)
        → 失败 → DeleteAccount 回滚 + 500
   → 删 provider/api_key；其余字段 UpdateAccount 应用
   → 201 {account: key_set=true, key_preview:sk****xx}
        │
[前端] onCreated → refetch → 列表显示「已配密钥」徽章
   ★ 本轮 credentials.json 已落地该明文密钥
```

删除链路：
```
[前端] AccountKeys.handleDeleteAccount(id)
   → DELETE /api/settings/accounts/:id
        │
[后端] DeleteAccount
   → 有成员 variant.account_ref==id 且非 force → 409 {bound_member_ids}
        │
[前端] 捕获 409 → confirm → DELETE .../:id?force=true
   → 删账号 + credStore.Delete(id) + emitEvent
```

---

## 7. 技术契约与接口设计 (Technical Contract)

### 7.1 接口 Schema

```
GET    /api/settings/accounts            → 200 [Account]
POST   /api/settings/accounts            → 201 Account      (body 见下)
PATCH  /api/settings/accounts/{id}       → 200 nil          (body 见下)
DELETE /api/settings/accounts/{id}       → 200 nil          (?force=true 强制)
```

**POST / PATCH body（snake_case）**
```json
{
  "name": "my-openai",
  "provider": "openai",
  "client_id": "claude",
  "display_name": "my-openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "models": ["gpt-5.4"],
  "model_aliases": {},
  "env_vars": {"MY_VAR": "x"},
  "auth_type": "api_key",
  "mode": "api_key",
  "builtin": false
}
```
> `api_key` 仅在非空时传；PATCH 传空串 `""` = 清除密钥；不传 = 保留。

**响应 Account（节选）**
```json
{
  "id": "uuid",
  "provider": "openai",
  "key_set": true,
  "key_preview": "sk****xx",
  "display_name": "my-openai",
  "base_url": "https://api.openai.com/v1",
  "models": ["gpt-5.4"],
  "auth_type": "api_key",
  "builtin": false
}
```
> 明文 `api_key` 永不出现在 list/create 响应中。

### 7.2 文件变动

无数据库；三文件落盘于 `ConfigRoot` / `CredentialRoot`（见 §5.1/§5.2）。成员 catalog（`dog-catalog.json`）另含 `deleted_breeds`、`seen_template_breeds` 两集合（见《成员管理》设计 §6.1 / §6.2）。

---

## 8. 验收标准 (Acceptance Criteria - AC)

- [x] **AC-01 (正常路径)**: Given 已登录（或开发模式无 AUTH_TOKEN），When 在「新增账号认证」填 baseUrl+apiKey+≥1 模型并保存，Then 201 且 `credentials.json` 中该账号明文密钥已落地、`key_set=true`、列表显示「已配密钥」。
- [x] **AC-02 (异常与边界)**:
  - When 新建账号时 `credStore.Set` 失败，Then 账号被回滚删除且返回 500，不产生孤儿账号。
  - When PATCH 传 `api_key:""`，Then `credentials.json` 中该密钥被清除、`key_set=false`。
  - When `client_id` 不在白名单，Then 返回 400。
  - When 保存成员绑定一个不存在的 `account_ref`（非内置 OAuth），Then 返回 400 `account_ref "x" not found`。
  - When 配置文件加载时损坏，Then 仅告警并当空处理（不 500）；编辑写盘前自动生成时间戳 `.bak-<YYYYMMDD-HHMMSS>`（保留最近 5 份）供回滚。
- [x] **AC-03 (权限与安全)**:
  - 生产环境设置 `AUTH_TOKEN` 后，When 未带 `Authorization` 访问 `/api/settings/...`，Then 返回 401。
  - 列表响应不含明文密钥；`credentials.json` 权限 0600。
  - envVars 拒绝 `SOUNDS_GREAT_AI_` 前缀，避免覆盖运行时系统变量。
  - When 删除被成员绑定的账号未带 `force`，Then 返回 409 并返回 `bound_member_ids`。
- [x] **AC-04 (客户配置安全)**:
  - 生产设 `workspaceDir` 后，`credentials.json` 落 `{home}/.sounds-great-ai`，`dog-catalog.json`/`accounts.json` 落 `{workspaceDir}/.sounds-great-ai`；两者均不受对方 env 覆盖牵连（独立根）。
  - 加载期放入损坏文件不产生 `.bak`（仅 WARN 日志）；编辑写盘产生 `.bak` 且含编辑前内容。

---

## 9. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- **[x] 资损与网络安全 (Security)**
  - 敏感数据脱敏: 是（密钥仅存 `credentials.json` 0600，list 响应只回 `key_preview`）。
  - 防重提交/幂等: `credStore.Set` 幂等覆盖；`CreateAccount` 用 uuid 生成新 id，无天然幂等键（属本地配置管理，风险低）。
- **[x] 高并发与限流降级**: 普通（本地配置链路，预估 QPS 低）。`FileCredentialStore`/`FileSettingsStore` 用 `sync.RWMutex` 保护，写路径 `tmp+rename` 原子写，外部改动由 `HotReloader` ~30s 热加载。
- **[x] 可服务性与监控**: 无独立 TraceID；关键失败返回结构化 error（如 `failed to persist credential`）。建议后续在 `auth.Wrap` 与 handler 顶层加 `[ERROR_CODE_...]` 日志。

---

## 10. Story 级 Definition of Done (DoD Checklist)

- [x] 3-Corner 澄清通过（既有实现梳理 + 修复，AC 已锁定）。
- [x] 单元测试覆盖：transport 包（Create/Update/Delete account、引用完整性 409）、settings 包（损坏当空、编辑 `.bak`、凭证读写）。
- [x] 静态/构建门禁：`go build ./...` 0、`go test ./...` 0、`tsc -b` 0、`vite build` ✓。
- [x] 关联修复已在仓库代码落地并通过验收。
- [ ] 监控告警与降级开关：建议在预发环境对 `auth.Wrap` 失败率配置告警（非阻塞）。

---

## 11. 修订记录 (Revision History)

- **2026-08-12（账户与密钥）**：梳理 accounts 分区前后端；落实 F1–F6 修复（密钥落 credentials.json、回滚、api_key 全量字段、引用完整性 409、auth.Wrap、env_vars 前缀）。
- **2026-08-12（客户配置安全）**：落实三项决策——① 凭证独立根 `CredentialRoot()`；② 编辑时时间戳 `.bak`（加载期损坏仅告警当空）；③ 追加式升级同步 `SyncTemplateBreeds` / `ListDeletedBreeds` / `deleted_breeds`。
- **2026-08-13（合并与代码对齐）**：将 SG-ACC-001 与 SG-OPS-001 合并为本文件；按 2026-08-13 代码实况修正：`DEFAULT_SECTION='accounts'`、`RAW_SECTIONS` 顺序 accounts 在前、`routes.go` 中 `/api/settings/` 与 `credStore` 的实际装配位置、`dog-catalog.json` 含 `deleted_breeds`/`seen_template_breeds` 两集合。「升级追加同步」详述移至《成员管理》设计 §6.2，本文件仅保留决策级描述与跨参。

---

> 关联文档：`SG-MEM-001-member-management.md`（成员管理/首启空/构建守护，成员经 `account_ref` 反向引用）、`internal/settings/file_store.go`、`internal/settings/credential.go`、`cmd/server/routes.go`、`internal/platform/breeds_merge.go`。
