# [FT-MCP-001] [Tech Story] MCP Integration：可管理 MCP 服务器注册表 + 平台即 MCP 服务器

> 本文件按 `sounds-great-ai` **2026-08-21 代码实况**逐文件梳理 MCP Integration 的前后端逻辑（不臆想）。
> 后端：`internal/mcp/{registry,store,probe}.go` + `internal/mcp/governance/` + `internal/transport/mcp_handler.go` + `internal/platform/platform.go` + `internal/adapter/unified/{executor,mcp_config}.go` + `cmd/platform-mcp-server/main.go` + `cmd/gen-mcp-baseline/main.go` + 装配 `cmd/server/{main,routes}.go`。
> 前端：`web/src/types/index.ts` + `web/src/components/drawer/{ToolPanel.tsx,tabs/McpTab.tsx}` + `web/src/i18n/{zh-CN,en}.ts`。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu | QA: @demu
- **故事点/复杂度**: [ L (8分) ]
- **范围**: 平台把 MCP 服务器从「硬编码 1 个」升级为「可管理的持久化注册表」——增删改启停 + 工具真实枚举/披露 + 按犬种范围；并把 SG 自身平台能力（线程/证据/人员记忆/狗狗队伍名册/breeds）暴露为「平台即 MCP 服务器」；补齐远程 MCP（HTTP/SSE 出站）、HTTP 回调回退、工具治理注解与基线 attestation。
- **业务/技术目标**:
  - As a **平台使用者（operator）/ CLI 智能体（agent）**,
  - I want to **在能力面板统一管理 MCP 服务器（本地 stdio / 远程 HTTP-SSE）、看到每台服务器真实暴露的工具、并把平台自身的 collab/memory/people/roster/breeds 能力以 MCP 工具形式交给 CLI 智能体调用；当 MCP 传输不可用时还能走 HTTP 回调回退；工具面有治理注解 + 基线 attestation 防漂移**,
  - So that **MCP 集成从一次性硬编码变成可运营、可披露、可审计的能力面，且密钥不泄露、工具面变更可被 CI fail-closed 拦截**。
- **关键指标/埋点**: 无（内部能力管理链路，非对外曝光功能）。

---

## 2. 功能叙事 (User Journey)

MCP 集成有**两个相互独立、由同一持久化 store 串联**的面：

1. **能力面板（operator 面）**：Drawer 内 `mcp` tab（`DrawerTabType` 含 `mcp`），并列「RAG Backend」「Loaded Dynamic Skills」「MCP 服务器管理」三段。operator 在此：
   - 查看所有 MCP 服务器（含 2 个平台内置 `knowledge`/`platform`，内置只读不可删、仅可启停）；
   - 每台服务器显示**真实工具清单**（probe 枚举 + 60s TTL 缓存，`refresh` 强制重枚举）、连接状态（ok/empty/error/unknown）、传输方式（本地 stdio / 远程 remote 徽标）；
   - 增删改启停服务器：新增/编辑在 stdio（command/args/env）与 remote（url/headers）之间切换；删除内置服务器被拒；
   - 展开「HTTP 回调回退」查看当 MCP 传输不可用时可直接调用的 REST 端点（platform 服务器按治理目录 1:1 生成 curl 样例）。

2. **运行时注入（agent 面）**：server 启动时 `SeedKnowledge`/`SeedPlatform` 把两个内置服务器种子进 store；当 CLI 智能体（claude/codex/opencode）被 spawn 时，`BuildMCPConfig()` 从 registry 取所有启用且匹配犬种的服务器，经 `WriteMCPConfigFile` 落成临时 `--mcp-config` JSON 交给 CLI；CLI 按 `type`（stdio 省略 / http / sse）+ `url`/`headers` 自行连接。

两个内置服务器的语义：
- `knowledge`（RAG）：stdio MCP 子进程直连 SQLite 向量库（`--db ragDBPath`）。
- `platform`（平台即 MCP 服务器）：stdio（默认）或 `--transport http`（Streamable HTTP，loopback + 强制 token）MCP 服务器，把 11 个治理工具按 family（collab/memory/people/roster/breeds）映射到 SG REST API；其 `CallbackURL` 指向 SG 自身 `http://localhost:<PORT>`，作为 HTTP 回调回退。

---

## 3. 前端处理逻辑

相关文件：`web/src/components/drawer/tabs/McpTab.tsx`、`web/src/components/drawer/ToolPanel.tsx`、`web/src/types/index.ts`、`web/src/i18n/{zh-CN,en}.ts`、`web/src/services/http.ts`（`apiGet/apiPost/apiPut/apiDelete`）。

### 3.1 挂载

- `ToolPanel.tsx`：`activeDrawerTab === 'mcp'` 时渲染 `<McpTab />`（`DrawerTabType = 'plan' | 'mcp' | 'memory' | 'files' | 'session-chain'`）。

### 3.2 页面结构 `McpTab.tsx`

三段自上而下：
1. **RAG Backend**：`useRagBackend()` 拿 `backend.active`（memory/sqlite）+ retirees + `syncProgress`；`switchBackend` / `triggerSync` 切换/同步。
2. **Loaded Dynamic Skills**：挂载时 `apiGet<LoadedSkill[]>('/api/skills')`，列出已加载动态技能名 + source。
3. **MCP 服务器管理**：核心段，见下。

### 3.3 服务器管理段

- **数据来源**：`loadServers(refresh=false)` → `apiGet<McpServer[]>('/api/mcp/servers' + (refresh?'?refresh=1':''))`；`refresh` 传 `?refresh=1` 触发后端强制重枚举工具。
- **启停**：`toggleServer(s)` → `apiPut('/api/mcp/servers/{name}', { enabled: !s.enabled })`（后端把「仅含 enabled」的 PUT 识别为 toggle 路径，走 `SetEnabled`）。
- **删除**：`deleteServer(s)` → `window.confirm(mcp.confirmDelete)` → `apiDelete('/api/mcp/servers/{name}')`。内置服务器（`builtin`）前端不渲染删除按钮（`!s.builtin` 才显示 edit/delete）。
- **编辑保存** `saveEdit(s, form)`：
  - 公共 patch：`breeds = form.breeds.split(',').map(trim).filter(Boolean)`。
  - `transport==='remote'` → `patch.url`、`patch.headers = parseKV(form.headers)`；否则 `patch.command`、`patch.args = form.args.split(/\s+/)`、`patch.env = parseKV(form.env)`。
  - `apiPut` 提交，成功后关闭编辑态并刷新。
- **新增** `AddServerForm`：`name` + `breeds` + `TransportToggle` + （remote → url/headers；stdio → command/args/env）；`canSubmit = name 非空 && (remote? url 非空 : command 非空)`；`apiPost('/api/mcp/servers', body)`。

### 3.4 关键纯函数（密钥掩码语义）

- `parseKV(text)`：把 `KEY=VALUE` 文本解析成 map；**跳过值为 `***` 的行**——因为 API 读返回的 env/headers 值被掩码成 `***`，回显后不重写真实密钥（PATCH 语义，后端同样跳过空值删 key）。
- `kvToText(kv)`：把 map 渲染成 `KEY=***`（掩码回显，不泄露真实值）。
- `statusBadge(s)`：`ok→emerald`、`empty→amber`、`error→rose`、`unknown→slate`。
- `builtinDescKey`：`{ knowledge: 'mcp.builtin.knowledge', platform: 'mcp.builtin.platform' }`，内置服务器显示人话能力说明。

### 3.5 `ServerCard`（单台服务器卡片）

- 头部：`display_name || name`；徽标：远程 `mcp.remote`（`s.url` 为真时）、状态徽标、`mcp.builtin`（`s.builtin`）。
- 正文：`isRemote ? s.url : s.command + ' ' + args.join(' ')`；工具清单 `s.tools.join(', ')`；`s.error` 红字展示 probe 错误。
- **HTTP 回退披露**（`s.fallback_available || s.callback_url` 时显示按钮）：`toggleFallback()` 首次点击 `apiGet<McpFallback>('/api/mcp/servers/{name}/fallback')` 拉取回退视图，渲染 `callback_url` + 每个工具的 `method path (name)` + curl `sample`；失败降级 `{note:'failed to load fallback'}`。
- 操作：`ON/OFF` 启停按钮；非内置才显示 `edit`/`delete`。

### 3.6 类型 `web/src/types/index.ts`

```ts
export interface McpServer {
  name: string;
  tools: string[];
  display_name?: string;
  command?: string;
  args?: string[];
  env?: Record<string, string>;        // masked on read ("***")
  url?: string;                          // remote
  headers?: Record<string, string>;      // masked on read ("***")
  callback_url?: string;                 // HTTP fallback
  fallback_available?: boolean;
  enabled?: boolean;
  builtin?: boolean;
  breeds?: string[];
  status?: string;                       // ok | empty | error | unknown
  error?: string;
}
export interface McpFallback {
  name: string;
  callback_url?: string;
  tools?: { name: string; method: string; path: string; sample: string }[];
  note?: string;
}
```

### 3.7 i18n

`web/src/i18n/zh-CN.ts` 的 `mcp.*` 键（en.ts 对应）：`settings.mcp`/`settings.mcp.desc`、`mcp.servers`/`add`/`edit`/`save`/`cancel`/`delete`/`confirmDelete`、`mcp.command`/`args`/`env`/`breeds`、`mcp.transport`/`transport.stdio`/`transport.remote`、`mcp.url`/`urlHint`/`headers`/`remote`、`mcp.status.{online,empty,error,unknown}`、`mcp.builtin`/`builtin.knowledge`/`builtin.platform`、`mcp.fallback`/`fallbackHint`/`fallbackEmpty`/`callbackUrl`/`showFallback`/`hideFallback` 等约 40 键。

---

## 4. 后端处理逻辑

### 4.1 领域模型 `internal/mcp/registry.go`

`MCPServerConfig`（JSON tag 全 snake_case）：

| 字段 | 说明 |
|------|------|
| `Name` / `DisplayName` | 唯一名 / 展示名 |
| `Command` / `Args` / `Env` | stdio 本地服务器（command 必填之一） |
| `Enabled` | 是否注入给 CLI |
| `Breeds` | 犬种 allowlist，空 = 全部 |
| `Builtin` | 平台种子服务器标记，UI 只读、不可删 |
| `URL` / `Headers` | 远程（HTTP/SSE）传输，Headers 存 auth、掩码 + 0600 落盘 |
| `CallbackURL` | HTTP 回调回退 |

`MCPRegistry` 是内存 map；`ForBreed(breed, task)` 返回**已启用**且 `breedMatches`（`Breeds` 为空或命中 `breed.ID/breed.Name`）的服务器；`All()` 返回全部（含停用）。

### 4.2 持久化 store `internal/mcp/store.go`

`FileStore` 是 operator 管理的唯一真相源，落盘 `<ConfigRoot>/mcp-servers.json`（与 `accounts.json`/`dog-catalog.json` 同根，`settings.ConfigRoot` 三层解析）。

- `NewFileStore(configRoot, reg)`：建目录 → `load()` → `sync()`。
- `load()`：缺文件→空；JSON 损坏→ `WARN` 打印并**空启动**（fail-soft，不崩平台）。
- `save()`：**原子写**（写 `.tmp` + `os.Rename`，`0o600`）+ 编辑前备份 `mcp-servers.json.bak-<ts>`（最多保留 5 份，`pruneBackups`）+ 按 name 排序。
- `sync()`：`reg.Reset()` 后按 items 重建内存 registry（每次变更后调用）。
- **`Add`**：校验 `Name` 非空、`command XOR url`（二者皆无/皆有均报错）、重名报错；`Builtin` 强制 false；写盘 + sync。
- **`Update`**（PATCH 语义）：内置服务器只读（除 `Enabled`，且不可改 command/args/env/breeds/displayName）；`Env`/`Headers` 合并时**空值删 key**（让拿到掩码 `***` 的客户端可省略而不覆盖真实密钥）；`URL`/`CallbackURL`/`Breeds`/`DisplayName` 非空才写；`Enabled` 直接覆盖。
- **`SetEnabled`**：仅 toggle `Enabled`。
- **`Remove`**：内置服务器拒绝删除。
- **`seedBuiltin`**：内置种子；已存在则**保留当前 `Enabled`**（operator 停用后重启不再强制启用），仅全新条目默认启用；仅当 store 文件不存在时才写盘（避免覆盖 operator 状态）。
- **`SeedKnowledge(command, args)`** → `seedBuiltin("knowledge", ...)`（RAG）。
- **`SeedPlatform(command, args, env, headers, callbackURL)`** → `seedBuiltin("platform", ...)`。

### 4.3 工具枚举 `internal/mcp/probe.go`

`ProbeCache`（TTL 60s，超时 5s，`sync.Mutex`）：
- `Get(name, cfg, force)`：缓存新鲜且非 force → 命中；否则 `probe(cfg)` 后缓存。**永不向上抛错**——失败以 `status="error"` 返回，让 API 优雅降级。
- `probe(cfg)` 分发：`cfg.URL != ""` → `probeRemote`；否则 stdio。
- **stdio**：`exec.CommandContext` spawn 子进程，`mcp.IOTransport{Reader: stdout, Writer: stdin}`（go-sdk 不替 StdioTransport 拉进程，SG 自管子进程），`client.Connect` → `ListTools`；defer `Process.Kill()+Wait()` 确保僵进程被回收。
- **`probeRemote`（远程 MCP 出站客户端）**：`sse://` 前缀或 `transport=sse` → `mcp.SSEClientTransport`；否则 `mcp.StreamableClientTransport{Endpoint, HTTPClient}`；有 headers 时用 `headerInjector`（`http.RoundTripper`）注入，**token 不进 URL/日志**。
- `headerInjector.RoundTrip`：对每个 header `req.Header.Set`，保留 base transport。

### 4.4 管理路由 `internal/transport/mcp_handler.go`

`MCPHandler{store, probe}`，`Routes()` 挂到 `/api/mcp/servers`（`routes.go` 以 `auth.Wrap` 包裹）：

| 方法 | 路径 | 行为 |
|------|------|------|
| GET | `/api/mcp/servers` | `list`：`List()` → `toView`；`?refresh=1` 强制重枚举 |
| POST | `/api/mcp/servers` | `add`：decode → `Enabled=true` → `store.Add`；重名→409 |
| GET | `/api/mcp/servers/{name}/fallback` | `fallback`：见下 |
| PUT | `/api/mcp/servers/{name}` | `update`：**裸 `{enabled}` → `SetEnabled`（toggle）**；否则 `store.Update` |
| DELETE | `/api/mcp/servers/{name}` | `remove`：`store.Remove`，成功 204 |

- `mcpServerView`：`Env`/`Headers` 经 `maskEnv`/`maskHeaders` 掩码为 `***`；`Tools`/`Status`/`Error` 来自 probe；`FallbackAvailable = cfg.CallbackURL != "" || cfg.Name == "platform"`。
- **`fallback`**（HTTP 回调回退）：`name=="platform"` 时按 `governance.Catalog()` 生成 `mcpFallbackTool` 列表（`Name`/`Method`/`Path`/`Sample`），`buildFallbackSample` 渲染 `curl -X <method> '<base><path>' -H 'Authorization: Bearer <SG_API_TOKEN>'`（读端点显式带 auth 占位，避免泄露真实 token）；`CallbackURL` 为空时回退 `http://localhost:8080`。非 platform 且无 `CallbackURL` → `Note` 说明未配置回退。
- 冲突/未找到判定用 `containsSubstr`（`already exists` / `not found`），映射到 409/404。

### 4.5 注入 CLI `internal/platform/platform.go` + `internal/adapter/unified/`

- `Platform` 字段：`MCP *mcp.MCPRegistry`、`MCPStore *mcp.FileStore`；`NewPlatform` 时 `mcp.NewRegistry()` + `mcp.NewFileStore(settings.ConfigRoot(cfg.WorkspaceDir), mcpReg)`。
- **`BuildMCPConfig()`**：`MCP.ForBreed(nil, "")`（全局，非 per-breed——`nil` breed 命中空 allowlist）→ 组装 `unified.MCPServer`：
  - `s.URL != ""`（远程）：`Type = "http"`（`sse://` 或 `transport=sse` → `"sse"`），透传 `URL`/`Headers`，**清空 `Command`/`Args`/`Env`**。
  - 否则 stdio：透传 `Command`/`Args`/`Env`。
- `unified/executor.go`：`MCPServer{Name, Command, Args, Env, Type, URL, Headers}`；`unified/mcp_config.go`：`mcpConfigFile{mcpServers map}` + `mcpServerEntry{Type,Command,Args,Env,URL,Headers}`，`WriteMCPConfigFile` 写到 `os.CreateTemp("", "sg-mcp-*.json")`（**临时目录而非 workDir/.mcp-config.json**，避免把地址/token 泄进仓库/生成散落 tracked 文件），返回路径，CLI 经 `--mcp-config <path>` 消费，进程退出时由 `SpawnHandle.OnExit` 清理。
- CLI 消费点：`internal/adapter/{claude,codex,opencode}/adapter.go` 的 `buildArgs` 各自 `unified.WriteMCPConfigFile(mcp, workDir)` 后 `args = append(args, "--mcp-config", configPath)`。

### 4.6 装配 `cmd/server/{main.go, routes.go}`

- `routes.go`：共享 `mcpProbe := mcp.NewProbeCache(0,0)`（TTL/超时默认值）；`pl != nil && pl.MCPStore != nil` 时 `mux.Handle("/api/mcp/servers", auth.Wrap(...))` + 尾部斜杠 `/api/mcp/servers/`。
- `main.go`（`registry != nil && pl != nil`，即 RAG 就绪时）：
  - `SeedKnowledge(filepath.Join(workspaceDir,"bin","sounds-great-mcp-server"), []string{"--db", ragDBPath})`。
  - `SeedPlatform(filepath.Join(workspaceDir,"bin","sounds-great-platform-mcp-server"), []string{"--api-base", apiBase}, platformEnv, nil, apiBase)`，其中 `apiBase = "http://localhost:" + port`，`AUTH_TOKEN` 非空时 `platformEnv["SG_API_TOKEN"]=tok`，`CallbackURL=apiBase`。

---

## 5. 存储与数据模型

- **磁盘**：`<ConfigRoot>/mcp-servers.json`（`0o600`，原子写，编辑前 `.bak-<ts>` 备份最多 5 份）。`ConfigRoot` = env `SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT` → `<proj>/.sounds-great-ai` → `<home>/.sounds-great-ai`（`internal/settings/file_store.go`）。
- **结构**：`[]MCPServerConfig`（按 name 排序写入）；内存 `MCPRegistry` 由 `FileStore.sync()` 在每次变更后重建。
- **治理落盘**：`internal/mcp/governance/{mcp-surface-baseline.json, mcp-surface-attestation.json}`（`0o600`），由 `cmd/gen-mcp-baseline` 生成，CI 校验。

---

## 6. 平台即 MCP 服务器 + 工具治理（governance / attestation）

### 6.1 治理目录 `internal/mcp/governance/catalog.go`

`Catalog()` 返回 11 个治理工具（`ToolDefinition` 含 `ReadOnly/Destructive/Idempotent/OpenWorld` 四个治理注解 + `Family/Method/Path/PathParams/BodyParams/QueryParams/Required`），按 family：

| family | 工具 |
|--------|------|
| collab（5） | `sg_list_threads`、`sg_get_thread`、`sg_create_thread`、`sg_list_messages`、`sg_post_message` |
| memory（2） | `sg_search_evidence`、`sg_add_evidence` |
| people（1） | `sg_people_recall` |
| roster（2） | `sg_list_dogs`、`sg_get_dog` |
| breeds（1） | `sg_list_breeds` |

读工具 `ReadOnly=true`、`Idempotent=true`、`Destructive=false`；写工具 `ReadOnly=false`、`Idempotent=false`。目录是工具面**唯一真相源**。

### 6.2 基线 + attestation（防漂移）

- `baseline.go`：`BaselineEntry` 只投影 `name/family/read_only/destructive/idempotent/open_world`（**故意排除 description/path**，纯文档改动不失效契约）；`ComputeBaseline` 对 name 排序后的 canonical 条目做 `sha256` 得 `Digest`；`FeatureID = "SG-MCP-SURFACE"`、`SchemaVersion=1`。
- `attestation.go`：`Attestation{SchemaVersion, FeatureID, TargetRepository, BaselineDigest, Owner, AuthorizationRef}`；`ValidateAttestation` 强制**三方不变量**：`live catalog digest == committed baseline digest == attestation.baselineDigest`，任一漂移 fail-closed。
- `cmd/gen-mcp-baseline/main.go`：目录变更后重跑 `go run ./cmd/gen-mcp-baseline` 再生成基线 + attestation。
- 测试守卫：`attestation_test.go` 的 `TestMCPBaselineMatchesCatalog` / `TestMCPAttestationMatchesBaseline` / `TestCatalogHasGovernanceCertificate`（每个工具必须带治理证书）/ `TestBaselineFilePermissions`（0600）。

### 6.3 服务器 `cmd/platform-mcp-server/main.go`

- **flag**：`--api-base`（默认 `http://localhost:8080`）、`--api-token`（继承 `SG_API_TOKEN`）、`--transport stdio|http`、`--addr`（默认 `127.0.0.1:8090`）、`--http-token`（继承 `SG_MCP_HTTP_TOKEN`）。
- **`buildServer`**：`mcp.NewServer`，遍历 `governance.Catalog()`，每个工具 `AddTool` 时带 wire 级 `mcp.ToolAnnotations{ReadOnlyHint, IdempotentHint, DestructiveHint, OpenWorldHint}`。
- **stdio 模式**（默认）：`mcpServer.Run(ctx, &mcp.StdioTransport{})`。
- **HTTP 模式**（`--transport http`）：**fail-closed**——`--http-token` 为空直接 `log.Fatalf` 拒绝启动；`mcp.NewStreamableHTTPHandler` 挂 `/mcp`（`JSONResponse+Stateless`）+ `/health`；`authMiddleware` 用 `subtle.ConstantTimeCompare` 比对 `?token=` 或 `Authorization: Bearer`；`methodLoggedWriter` 记请求日志，`redactWriter` 对每个 `Write` 套 `SECRET_PATTERNS`（`ghp_`/`sk-`/`xox[bpo]-`/`AKIA`/`AIza` 等）脱敏输出。绑定 loopback 默认，注释明示「暴露的是 SG 自身平台能力工具，非本地 agent 供第三方 push，不违反 A2A-server 铁律」。
- **`doRequest`**：路径参数替换（缺必填 path 参数报错）→ 拼 `apiBase+path`；POST 组装 body（`tags` 兼容数组/逗号串 `normalizeTags`）；GET 拼 query；`Authorization: Bearer <token>`；响应 `>=400` 报错，否则 pretty-print JSON。平台不可达时报错提示 agent 可回退到 `GET /api/mcp/servers/platform/fallback`。

---

## 7. 端到端数据流

### 7.1 新增远程服务器

1. operator 在 McpTab `AddServerForm` 选 `remote`，填 `name`/`url`/`headers`/`breeds`。
2. 前端 `apiPost('/api/mcp/servers', body)` → `MCPHandler.add` → `store.Add`（校验 `command XOR url`）→ 原子写 `mcp-servers.json` → `sync()` 重建 registry → 返回 `toView`（此时 `probeRemote` 已枚举工具）。
3. 下次 spawn CLI agent 时 `BuildMCPConfig()` 把该服务器渲染为 `type=http|sse + url + headers` → `WriteMCPConfigFile` 临时 JSON → CLI `--mcp-config` 连接。

### 7.2 启停内置服务器

1. `McpTab` 点 `OFF` → `apiPut(...,{enabled:false})` → `update` 识别裸 enabled → `SetEnabled`。
2. `seedBuiltin` 语义保证重启后 `Enabled=false` 被保留（只有全新条目才默认启用）。

### 7.3 HTTP 回调回退

1. `McpTab` 展开回退 → `apiGet('/api/mcp/servers/platform/fallback')`。
2. `fallback` handler 按 `governance.Catalog()` 生成 11 条 `method + path + curl sample`（auth 用占位符）。
3. agent 在 MCP 传输不可用时，按这些 REST 指令直连 SG API 完成同一能力。

### 7.4 治理漂移检测

`catalog.go` 增删改工具 → 未重跑 `gen-mcp-baseline` → `TestMCPBaselineMatchesCatalog`/`TestMCPAttestationMatchesBaseline` fail（CI 拦截），提示 `go run ./cmd/gen-mcp-baseline`。

---

## 8. 技术契约与接口设计 (Technical Contract)

### 8.1 REST API（均 `auth.Wrap` 包裹）

| 方法 | 路径 | 请求 | 响应 |
|------|------|------|------|
| GET | `/api/mcp/servers` | `?refresh=1` 可选 | `mcpServerView[]`（env/headers 掩码） |
| POST | `/api/mcp/servers` | `MCPServerConfig`（`command` xor `url`） | 201 `mcpServerView`；重名 409 |
| GET | `/api/mcp/servers/{name}/fallback` | — | `mcpFallbackView{name, callback_url, tools[], note}`；未找到 404 |
| PUT | `/api/mcp/servers/{name}` | `{enabled}` 或 patch | 200 `mcpServerView`；内置只读 400；未找到 404 |
| DELETE | `/api/mcp/servers/{name}` | — | 204；内置 400；未找到 404 |

### 8.2 MCP wire（`--mcp-config` JSON）

```json
{ "mcpServers": {
    "platform": { "command": "bin/sounds-great-platform-mcp-server", "args": ["--api-base","http://localhost:8080"], "env": {"SG_API_TOKEN":"..."} },
    "remote-x": { "type": "http", "url": "https://example.com/mcp", "headers": {"Authorization":"Bearer ..."} }
} }
```

`type` 缺省 = stdio（本地）；`type=http|sse` + `url` + `headers` = 远程。

### 8.3 文件变动清单

**新增**：`internal/mcp/{store.go, probe.go, store_test.go}`、`internal/mcp/governance/{catalog.go, baseline.go, attestation.go, attestation_test.go, mcp-surface-baseline.json, mcp-surface-attestation.json}`、`internal/transport/mcp_handler{,_test}.go`、`cmd/platform-mcp-server/{main.go, main_test.go}`、`cmd/gen-mcp-baseline/main.go`。
**修改**：`internal/mcp/registry.go`、`internal/platform/{platform.go, platform_test.go}`、`internal/adapter/unified/{executor.go, mcp_config.go}`、`internal/transport/thread_handler{,_test}.go`（`POST /api/threads/{id}/messages`，供 `sg_post_message` 代理）、`cmd/server/{main.go, routes.go}`、`Makefile`（`tools` 加 `bin/sounds-great-platform-mcp-server` + `bin/gen-mcp-baseline`）、`web/src/types/index.ts`、`web/src/components/drawer/tabs/McpTab.tsx`、`web/src/i18n/{zh-CN,en}.ts`。

---

## 9. 验收标准 (Acceptance Criteria - AC)

- [ ] **AC-01（正常路径 - 列表披露）**: Given store 有内置 `knowledge`/`platform` + 用户新增的 stdio/远程服务器, When `GET /api/mcp/servers`, Then 返回每台的 `name/command|url/tools/status/enabled/builtin/breeds`，且 `env`/`headers` 值均为 `***` 掩码、`tools` 为真实枚举结果。
- [ ] **AC-02（新增校验）**: Given `command` 与 `url` 同时缺失或同时提供, When `POST /api/mcp/servers`, Then 返回 400 且不落盘（`command XOR url` 硬校验）。
- [ ] **AC-03（内置只读）**: Given 内置 `platform`, When `DELETE` 或改 `command/args/env` 的 `PUT`, Then 返回 400（`builtin` 只读，仅 `enabled` 可改）。
- [ ] **AC-04（启停持久）**: Given operator 停用内置 `platform`, When 重启 server, Then `enabled=false` 被保留（`seedBuiltin` 不强制重启用）。
- [ ] **AC-05（远程透传）**: Given `url="https://x/mcp"` 的服务器, When `BuildMCPConfig()`, Then 输出 `type=http + url + headers` 且 `command/args/env` 为空；`sse://` → `type=sse`。
- [ ] **AC-06（HTTP 回退）**: Given `platform` 服务器, When `GET /api/mcp/servers/platform/fallback`, Then 返回按 `governance.Catalog()` 生成的 11 条 REST 工具 + 带占位 auth 的 curl 样例。
- [ ] **AC-07（治理防漂移）**: Given 修改 `catalog.go` 后未重跑 `gen-mcp-baseline`, When `go test ./internal/mcp/governance/`, Then `TestMCPBaselineMatchesCatalog`/`TestMCPAttestationMatchesBaseline` fail（fail-closed）。
- [ ] **AC-08（远程枚举）**: Given 带 `Authorization` header 的远程服务器, When probe 枚举, Then token 经 `headerInjector` 注入、不出现在 URL/日志。
- [ ] **AC-09（权限与安全）**: Given 未认证请求, When 访问 `/api/mcp/servers`, Then 返回 401（`auth.Wrap`）；`platform-mcp-server --transport http` 无 `--http-token` 时拒绝启动。

---

## 10. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- **[x] 资损与网络安全**
  - 敏感数据脱敏：env/headers 读返回掩码 `***`；`platform-mcp-server` HTTP 输出经 `redactWriter` 套 `SECRET_PATTERNS` 脱敏；`WriteMCPConfigFile` 写临时目录避免 token 泄进仓库。
  - 防重/幂等：`Add` 重名 409；`SetEnabled` 幂等；`probe` 缓存 + `refresh` 强制重枚举。
- **[x] 高并发与限流降级**
  - 峰值 QPS：普通（内部管理 API）。
  - 降级/兜底：probe 失败不阻塞列表（`status="error"` 返回）；`probe` 5s 超时 + 子进程 `Kill+Wait` 回收；`store.load` 损坏文件空启动；`platform` 工具调用平台不可达时返回错误并提示回退到 HTTP callback。
  - 动态开关：无（无 Feature Flag 引入）。
- **[x] 可服务性与监控**
  - 核心日志：`platform-mcp-server` 记 `[platform-mcp] METHOD PATH` 请求行；`store.load` 损坏 WARN；`probe` 错误随视图 `error` 字段暴露。
  - 告警：无（内部链路，未接监控）。

---

## 11. Story 级 Definition of Done (DoD Checklist)

- [x] 单元测试覆盖：`internal/mcp`（registry 10 + store 6）、`internal/mcp/governance`（4）、`internal/transport`（mcp_handler 3）、`internal/platform`（BuildMCPConfig remote 等）、`cmd/platform-mcp-server`（7）——核心路径已覆盖。
- [x] 静态检查：`go build ./...`、`go vet`、`go test`（受影响 7 包）、`web tsc -b` 全绿。
- [x] 治理基线 + attestation 生成：`go run ./cmd/gen-mcp-baseline`（digest `sha256:0807dd89…`，11 tools，0600）。
- [x] 二进制命名统一：`Makefile tools` 产出 `bin/sounds-great-mcp-server` / `bin/sounds-great-platform-mcp-server`，与 `cmd/server/main.go` 引用、`sounds-great-ai` 主程序命名风格一致。

---

## 12. 修订记录 (Revision History)

| 日期 | 说明 |
|------|------|
| 2026-08-21 | 初版：按代码实况逐文件梳理 MCP Integration 前后端逻辑（注册表 CRUD + probe 枚举 + 远程 MCP + HTTP 回退 + 治理 attestation）。 |
| 2026-08-21 | 修正：统一 MCP 二进制命名为 `sounds-great-mcp-server`/`sounds-great-platform-mcp-server`；全文去除外部参照，只保留 SG 自身描述。 |
