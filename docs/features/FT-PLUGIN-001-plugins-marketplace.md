# [FT-PLUGIN-001] [Tech Story] 插件系统与市场（panels P1–P4）

> 本文档基于 `sounds-great-ai` 真实源码核查（**2026-08-23 生成，反映截至本日的代码真实状态**）编写，把「插件 = 自包含分发包（breed 定义 + skills）+ 市场分发（索引 + 验签）」这一子系统（panels-roadmap P1–P4，含同批交付的 concierge/voice/connectors 配置面板）的前后端协作逻辑固化为单一可信来源，供后续开发、review 与新人 onboarding 使用。
>
> 交付批次：**P1/P2（concierge + voice + connectors）2026-08-22**；**P3（插件系统）与 P4（市场）2026-08-23**。各批次逐项交付记录见 `docs/plans/panels-roadmap.md`。
>
> 关联：排期与交付记录 `docs/plans/panels-roadmap.md`（权威进度表）；skills 安全管线细节 `FT-SKILL-001`；breed 注册通道 `FT-MEM-001`；密钥纪律参照 `FT-ACC-001`。
>
> **关键背景**：P1/P2 交付前，`panels_handler.go` 六个端点全是 stub（硬编码空值、PATCH 回显不落盘、自带 `Access-Control-Allow-Origin: *`、未包 auth.Wrap）；前端预留约 110 个孤儿 i18n key。P3/P4 交付前，插件与市场在前后端均不存在。

---

## 1. 元信息与业务价值 (Context & Value)

- **类型**: [ ] Biz Story (业务)   [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu(设计/编排) | QA: @demu(测试/诊断)
- **故事点/复杂度**: L（新域 internal/plugins + internal/marketplace + skills 动态源改造 + 前端三面板）
- **业务/技术目标**:
  - As a **operator**,
  - I want to **把第三方/自制的狗狗（breed）与技能（skills）打包成一个 zip，经安全审查后一键启停；也可从市场索引经发布者验签后安装**,
  - So that **能力扩展走"安装即待审、批准才启用、卸载即清理"的可治理管线，而不是手工改配置文件（铁律 3）或信任未验证的外部代码**。
- **关键指标/埋点**: 无前端埋点；安全可观测性来自 `/api/skills/security`（插件 skills 进 pending）、插件 API 响应（installed/enabled/blocked 明细）。

### 1.1 一句话定位

插件系统由**四层**构成：

0. **配置面板层**（P1/P2，2026-08-22 交付）：concierge / voice / connectors 三个纯配置面板，`settings.PanelConfigStore` 持久化于 `<ConfigRoot>/panels/*.json`（原子写 0600、读时回填默认值）。详见 §2 AC-10~12 与 §4.1。
1. **包层**（`internal/plugins`）：manifest 校验 + zip 安装器（zip-slip 防护）+ `registry.json` 注册表。
2. **治理层**（`internal/transport/plugins_handler.go`）：四端点生命周期 + **启用门禁**（随包 skills 全部 approved 才能启用；breed 经既有校验通道注册）。
3. **分发层**（`internal/marketplace` + 同 handler 两端点）：索引缓存浏览 + **ed25519 验签安装**（验签先于解包；无受信公钥 fail-closed）。

### 1.2 端到端数据流（本地安装）

```
zip 上传 (multipart, ≤64MiB)
  → plugins.Service.Install
      临时目录解包（safeJoin 拒绝绝对路径/../；歧义 manifest 拒绝）
      → plugin.json 校验（id ^[a-z0-9][a-z0-9-]{0,63}$）
      → 原子移动 <ConfigRoot>/plugins/<id>/ + registry 登记【disabled】
      → SkillManager.AddSource(<id>/skills, "plugin") + Scan()
         （随包 skills 自动进 pending 安全审查 —— 外部源纪律）
启用 PATCH {enabled:true}
  → 门禁：随包 skill 任一非 approved → 409 列明阻塞项（fail-closed）
  → breeds/*.json 经 SettingsStore.CreateBreed/UpdateBreed（source='plugin'，走既有校验）
停用   → breed enabled=false（定义保留）；skills 源移除
卸载   → 插件 breeds 全删 + skills 源移除 + 载荷/注册表清除
重启   → platform 构造时按 registry 重挂 enabled 插件的 skills 源
```

### 1.3 市场信任模型（P4 核心）

索引经 HTTPS 拉取（5min TTL 缓存，源不可达时降级陈旧缓存并带 `note`），**但索引不承载信任**——每个压缩包必须带能通过受信公钥（env `SG_MARKETPLACE_PUBKEYS`，base64 逗号分隔；索引 URL 由 `SG_MARKETPLACE_INDEX` 指定）验证的 ed25519 签名 + sha256 digest。安装顺序：**digest（挡损坏）→ 验签（挡伪造）→ 才进 zip 解析器**。未配置公钥 → 安装 403 fail-closed（浏览不受影响）；篡改索引无法引入未签名包。

---

## 2. 验收标准 (Acceptance Criteria - AC)

- [x] **AC-01 (正常路径-安装)**: Given 合法 zip（plugin.json + skills/ + breeds/）, When `POST /api/plugins/install`, Then 落盘 + registry 登记 **disabled**，随包 skills 进 pending 审查（`TestPluginsHandlerLifecycle`）。
- [x] **AC-02 (安全-zip-slip)**: Given 含 `../escape.txt` 或绝对路径 entry 的 zip, When 安装, Then 拒绝（`TestInstallZipSlipRejected`）；歧义 manifest（多个候选 plugin.json）拒绝（`TestInstallRejectsMissingManifestAndDuplicates`）。
- [x] **AC-03 (安全-启用门禁)**: Given 随包 skill 存在 pending/quarantined, When `PATCH {enabled:true}`, Then 409 且响应列明阻塞 skill；全部 approved 后启用成功（`TestPluginsHandlerLifecycle`）。门禁走 `ApproveSkill` 真实指纹续期路径（裸 SetState 会被指纹校验打回 quarantine——测试期间实证）。
- [x] **AC-04 (正常路径-breed 注册)**: Given 插件启用, When breeds/*.json 合法, Then 经 `SettingsStore` 注册 `source='plugin'`；停用 breed `enabled=false` 保留定义；卸载删除（同上测试）。
- [x] **AC-05 (正常路径-重启恢复)**: Given 插件 enabled 且服务重启, When platform 构造, Then 随包 skills 源自动重挂（`platform.go` EnabledPlugins 循环）。
- [x] **AC-06 (正常路径-市场浏览)**: Given 索引可达, When `GET /api/marketplace?query=`, Then 服务端代理（缓存 ≤5min）返回条目 + installed 标记；不可达时有陈旧缓存用缓存（带 note），无缓存 503（`TestClientListCacheQueryAndFind`/`TestClientOfflineServesStaleCacheThenErrors`）。
- [x] **AC-07 (安全-验签安装)**: Given 签名包, When `POST /api/marketplace/install`, Then 验签通过 → 复用 P3 安装器（默认禁用）；**篡改字节 403 且插件列表零残留**；无受信公钥 403 且错误指向 env 旋钮（`TestMarketplace*` 三例）。
- [x] **AC-08 (安全-验签先于解包)**: Given 未验证字节, When 安装流程, Then 不进入 zip 解析器（`marketplaceInstall` 中 Verify 在 Install 之前——防御纵深：即使 zip 解析器未来出漏洞也接触不到未验证输入）。
- [x] **AC-09 (权限与安全)**: 全部端点 `auth.Wrap`；注册表 `registry.json` 0600（载荷目录 0755/文件 0644，非敏感内容）；上传 `MaxBytesReader` 64MiB 与解压预算一致。

**P1/P2（配置面板，2026-08-22 交付）**：

- [x] **AC-10 (P1-配置持久化)**: Given concierge/voice 任一面板, When `PATCH` 部分字段, Then 逐字段校验（concierge：hex 色 `#RRGGBB`/尺寸 16-256/档位 low|medium|high/阈值 0-20；voice：语速 0.25-4.0/glossary ≤200 条）后**合并落盘**（未传字段保持原值），新 handler 实例重读返回修改值（`TestPanelsHandler_ConciergeDefaultsAndPersistence`/`_VoicePersistenceAndValidation`）；越界值 400 且状态不变。
- [x] **AC-11 (P2-连接器掩码与探活)**: Given 带 `auth_key` 的连接器, When 任意读路径（GET 列表/POST 回显/PATCH 回显）, Then 只见 `auth_key_set`/`auth_key_preview`（前 3+…+后 2），原值永不出线（`TestPanelsHandler_ConnectorsCRUDAndMasking`）；PATCH 不带 `auth_key` 字段即保留原密钥；`POST …/{id}/test` 探活（5s 超时，配置密钥时带 Bearer）回 `{ok, latency_ms, status}` 并把摘要写回 `last_check`——鉴权失败计为探测结果（ok=false）而非传输错误（`TestPanelsHandler_ConnectorProbe`）。
- [x] **AC-12 (P2-安全修复)**: Given panels 端点, When 挂载, Then ① 已包 `auth.Wrap`（交付前是全站唯一未鉴权的 config 写入面）；② 本地 `Access-Control-Allow-Origin: *` 覆盖已删除、CORS 归中央 `CORSMiddleware`（`TestPanelsHandler_NoPermissiveCORSOverride` 回归守护）。

---

## 3. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- [x] **资损与网络安全 (Security)**
  - zip-slip：`safeJoin`（绝对路径/`..` 前缀/Rel 复核三层）+ 临时目录解包后原子移动。
  - zip 炸弹：解压总量 64MiB 上限 + 上传端同预算。
  - 信任链：ed25519 验签 + sha256 digest，公钥 env 注入不内置；无钥 fail-closed。
  - 卸载清理：breeds/skills 源/载荷/注册表四清，无残留（测试断言列表空）。
- [x] **高并发与限流降级**
  - 市场索引 5min TTL 缓存 + 陈旧降级；索引/包源不可达均为普通 HTTP 错误，不影响已装插件运行。
  - 插件启停涉及 `SkillManager.Scan()`（全源重扫，毫秒级）与 SettingsStore 写——低频 operator 操作，无限流需求。
- [x] **可服务性**
  - 安装/门禁/验签失败均带明细（阻塞 skill 清单、验签失败原因、env 指引）；测试覆盖错误文案关键串。

---

## 4. 技术契约与接口设计 (Technical Contract)

### 4.1 REST 端点

**P1/P2 配置面板**（`panels_handler.go`，全部 `auth.Wrap`）：

| 端点 | 方法 | 用途 | 关键治理 |
|------|------|------|----------|
| `/api/config/concierge` | GET/PATCH | 迎宾犬配置（头像/色/尺寸/人设/问候/主动策略） | 逐字段校验；合并持久化 |
| `/api/config/voice` | GET/PATCH | TTS/STT 配置（含 glossary） | 同上；仅配置不推理 |
| `/api/config/connectors` | GET/POST | 连接器列表/新建 | 密钥掩码读模型 |
| `/api/config/connectors/{id}` | PATCH/DELETE | 修改（nil auth_key 保留密钥）/删除 | 类型枚举+URL 校验 |
| `/api/config/connectors/{id}/test` | POST | 探活（Bearer+5s 超时） | 结果摘要写回 last_check |

**P3/P4 插件与市场**（`plugins_handler.go`）：

| 端点 | 方法 | 用途 | 关键治理 |
|------|------|------|----------|
| `/api/plugins` | GET | 已装列表（View：manifest+enabled+breeds+skills） | — |
| `/api/plugins/install` | POST (multipart `package`) | 本地 zip 安装 | 落 disabled；skills 进 pending |
| `/api/plugins/{id}` | PATCH `{enabled}` | 启停 | 启用=审查门禁+breed 注册；停用=breed off+源移除 |
| `/api/plugins/{id}` | DELETE | 卸载 | 四清（源/breed/载荷/注册表） |
| `/api/marketplace` | GET `?query=` | 索引浏览（服务端缓存） | installed 标记；禁用 503 |
| `/api/marketplace/install` | POST `{id}` | 验签安装 | digest→验签→P3 安装器；无钥 403 |

### 4.2 目录与数据模型

```
<ConfigRoot>/plugins/
  registry.json               # [{id, version, enabled, installed_at}] (0600)
  <plugin-id>/
    plugin.json               # {id,name,version,description,permissions[]}
    breeds/*.json             # BreedConfig（启用时注册，source='plugin'）
    skills/*/SKILL.md         # 安装即入 pending 审查
<ConfigRoot>/panels/          # P1/P2 配置面板（concierge/voice/connectors.json）
```

env：`SG_MARKETPLACE_INDEX`（索引 URL，空=市场禁用）、`SG_MARKETPLACE_PUBKEYS`（base64 ed25519 公钥逗号分隔，空=安装 fail-closed）。

### 4.3 关键组件

| 层 | 文件 | 职责 |
|----|------|------|
| P1/P2 存储 | `internal/settings/panel_config.go` | `PanelConfigStore`：concierge/voice/connectors 三文档读写（原子写、默认值回填）；`Connector.AuthKey` 落盘 0600、HTTP 层掩码 |
| P1/P2 端点 | `internal/transport/panels_handler.go` | 五路径六操作：逐字段校验、合并持久化、连接器 CRUD+探活（`probeConnector`）、密钥掩码（`maskKey`/`connectorView`） |
| P1/P2 前端 | `ConciergePanel` / `VoicePanel` / `ConnectorsPanel.tsx` | 悬浮球实时预览 / TTS/STT/词汇表 / 连接器探活+二次确认删除；i18n 激活沉睡的 `concierge.*`/`voice.*`/`im.*` 49 键 |
| 包层 | `internal/plugins/service.go` | Manifest/View/Entry、Install（zip-slip/歧义/重装拒绝）、SetEnabled、Uninstall、BreedConfigs、EnabledPlugins |
| 包层 | `internal/plugins/zipinput.go` | SeekableReader 契约（multipart.File 同构）+ openZip |
| 分发 | `internal/marketplace/client.go` | 索引拉取/TTL 缓存/陈旧降级/查询/Find；Download（64MiB）；TrustedKeys(env)；Verify(digest→ed25519) |
| 治理 | `internal/transport/plugins_handler.go` | 六端点；启用门禁（errApprovalGate 409 明细）；breed 注册/摘除；市场验签安装 |
| skills 联动 | `internal/skills/manager.go` `AddSource/RemoveSource` | 运行时动态源（幂等、按目录去重）；plugin 来源沿用外部源安全纪律 |
| 启动恢复 | `internal/platform/platform.go` | 构造 skillMgr 后按 registry 重挂 enabled 插件源 |
| 前端 | `web/src/components/settings/PluginsPanel.tsx` | 上传/启停/卸载/待审提示 + 市场区（搜索/一键安装/已装徽章/验证说明） |
| 前端 | `web/src/services/pluginsService.ts` + `types/plugins.ts` | API 客户端（multipart 契约/错误解包/URL 编码）与类型 |

### 4.4 与既有系统的接缝（不绕过既有校验）

- **skills**：插件 skills 不直接挂载——经 `SkillManager` 外部源管线（pending→approve→指纹续期），门禁只读 `Security().StateOf`。
- **breeds**：不走私有写——`SettingsStore.CreateBreed/UpdateBreed/DeleteBreed`（与 `/api/breeds` 同通道）。
- **catalog 装配**：插件 breed 进 dog-catalog 后即被既有 reload/校验/mention 路由覆盖，无旁路。

---

## 5. Story 级 Definition of Done (DoD Checklist)

- [x] P1–P4 全部交付（panels-roadmap 进度表 + 各段交付记录）。
- [x] 后端测试：P1/P2 `panels_handler_test.go` 6 测试函数（默认值/持久化重读/部分更新/越界拒绝且状态不变/密钥掩码/探活三态/CORS 回归）；P3 plugins 7 例；P4 marketplace 7 例 + transport 生命周期/校验/市场 8 例；前端 72+ 用例全绿。
- [x] `go build ./...` + `go test ./...` + `tsc -b` + `vitest` + `oxlint` 全绿。
- [x] 安全铁律逐条落实并测试锁定（zip-slip/炸弹/验签先于解包/无钥 fail-closed/卸载四清/连接器密钥永不回明文/CORS 归中央）。

## 6. 已知缺口与后续（登记不遗失）

| # | 项 | 状态 | 说明 |
|---|----|------|------|
| G-1 | `permissions[]` enforcement | ⏳ 声明已透传 UI，无消费方 | 待治理面（governed 工具/治理 MCP）消费声明时落地 |
| G-2 | tar.gz 支持 | ⏳ 未做 | 当前仅 zip；tar 需额外符号链接防护，独立排期 |
| G-3 | 插件内 mcp.json 随包 | ❌ 不做（无消费方） | 避免死配置；待 MCP 面有插件化需求再议 |
| G-4 | 市场 category 参数 / 按版本固定安装 | ⏳ 未做 | 现为取最高版本；有真实市场数据后按需 |
| G-5 | 真实索引与 publisher 密钥运营 | ⏳ 运营事项 | 代码就绪：发布方生成 ed25519 密钥对、公钥入 env、索引 URL 入 env |
| G-6 | 插件级遥测（安装/启用计数） | ⏳ 未做 | 市场条目有 installs 字段位；本地无埋点 |
| G-7 | Voice 真实 TTS/STT 推理（P1） | ❌ 显式不做 | `voice.json` 仅配置；推理接入待独立 Tech Story（panels-roadmap 显式不做清单） |
| G-8 | IM 消息双向桥接（P2） | ⏳ 独立排期 | 连接器现仅注册+探活（P2 范围内）；桥接另立排期（P2.5） |

---

> 关联文档：`docs/plans/panels-roadmap.md`（排期/交付记录权威源）、`FT-SKILL-001`（安全管线全貌）、`FT-MEM-001`（breed CRUD 通道）、`FT-ACC-001`（密钥掩码纪律参照）。
