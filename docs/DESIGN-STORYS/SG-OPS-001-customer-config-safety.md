# SG-OPS-001 — 客户配置安全：根目录拆分 / 编辑时备份 / 追加式升级同步

- **ID**: SG-OPS-001
- **标题**: Customer Config Safety — credential root isolation, edit-time backups, additive upgrade sync
- **状态**: ✅ 已实施（2026-08-12）
- **作者**: Sounds Great AI 犬队
- **关联**: `SG-MEM-001-member-management.md`（成员/种子同步）、`SG-ACC-001-accounts-keys-auth.md`（账号/密钥根）、`internal/settings/file_store.go`、`internal/settings/credential.go`、`internal/platform/breeds_merge.go`、`cmd/server/routes.go`

---

## 1. 背景与决策

客户部署时，`~/.sounds-great-ai`（或项目下 `.sounds-great-ai`）是**客户的数据目录**，可能为空、可能已有内容（除密钥外）。基于客户配置安全考量，用户拍板三项客户安全决策：

1. **目录放置**：只有 `credentials.json`（密钥）放在全局 home `~/.sounds-great-ai`；其它内容（`dog-catalog.json` / `accounts.json`）生成到**项目下** `.sounds-great-ai`。
2. **损坏策略**：`.bak` 只在**编辑写盘时**生成，且带时间戳；加载期发现损坏**不再自动备份**，仅告警并当空处理（不掩盖损坏）。
3. **升级增强同步**：新模板犬**追加**到老客户 catalog（老客户升级后能看到新预置犬），但客户删过的犬不复活（决策 D2）。

---

## 2. 三改动明细

### 2.1 根目录拆分（凭证独立根）
- 新增 `settings.CredentialRoot()`（`file_store.go`）：解析顺序 `SOUNDS_GREAT_AI_CREDENTIAL_ROOT`（支持 `~`）→ `{home}/.sounds-great-ai`。
- `cmd/server/routes.go:114` 的 `credStore` 改为 `settings.NewFileCredentialStore(filepath.Join(settings.CredentialRoot(), settings.CredentialsFileName), true)`。
- `catalog/accounts` 仍走 `settings.ConfigRoot(projectRoot)`（项目 `.sounds-great-ai`，可被 `SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT` 覆盖）。
- **效果**：清项目配置不会误删密钥；密钥与项目配置物理隔离（凭证独立根）。

### 2.2 编辑时时间戳 .bak
- 移除 `reloadFromDisk` 的加载期 `backupCorrupt`（file_store.go / credential.go）：损坏 → `log.Printf("WARN: ... is corrupt; treating as empty (no backup written at load)")` + 当空。
- 新增 `backupBeforeWrite(path)`（file_store.go，同包，credential.go 复用）：写盘前若目标文件已存在，快照为 `<path>.bak-<YYYYMMDD-HHMMSS>`（如 `dog-catalog.json.bak-20260812-182600`）。
- `pruneBackups(path, maxBackups=5)`：保留最近 5 份，旧的不限量清理（`sort.Strings` + `os.Remove`）。
- 调用点：`flushCatalog` / `flushAccounts` / credential `flush` 顶部。首次种子化（文件不存在）不生成空 `.bak`。

### 2.3 追加式升级同步
- `internal/platform/breeds_merge.go`：`seedCatalogIfEmpty` 重构为 `SyncTemplateBreeds`（导出）。`MergedBreeds` 每次启动调用。
- 逻辑：模板犬中 `catalog 已有` 或 `在 deleted_breeds` 的跳过；其余（catalog 缺失且未被删）追加写入，强制 `Enabled=true`（默认启用，避免整队误判停用）。
- `deleted_breeds` 追踪：`SettingsStore` 接口新增 `ListDeletedBreeds() ([]string, error)`；`FileSettingsStore` 在 `catalogDocument` 增 `deleted_breeds []string` 字段、`DeleteBreed` 写入 id、`reloadFromDisk` 读回；`InMemorySettingsStore` / `SettingsStoreAdapter` 同步实现。
- **效果**：老客户升级可见新预置犬；自己删过的犬（含模板犬）不复活（D2）。幂等：重复 sync 不重复、不丢已有犬。

---

## 3. 存储模型（根目录总览）

| 文件 | 内容 | 权限 | 管理方 | 落盘根 |
|---|---|---|---|---|
| `dog-catalog.json` | breeds + breedOrder + roster + review_policy + leader + configs + `deleted_breeds` | 0644 | `FileSettingsStore` | `ConfigRoot`（项目 `.sounds-great-ai`） |
| `accounts.json` | 账号元数据 | 0644 | `FileSettingsStore` | `ConfigRoot`（项目 `.sounds-great-ai`） |
| `credentials.json` | 密钥（明文） | 0600 | `FileCredentialStore` | `CredentialRoot`（**全局 home `~/.sounds-great-ai`**） |

> 开发/本地若 `workspaceDir` 为空，`ConfigRoot` 也回退到 `{home}/.sounds-great-ai`，此时 catalog/accounts 与 credentials 同处 home（与旧行为兼容）；生产设 `workspaceDir` 后两者分离。

---

## 4. 技术契约

- 备份命名：`<原文件名>.bak-<YYYYMMDD-HHMMSS>`（如 `credentials.json.bak-20260812-182600`）。
- 保留份数：每文件最近 5 份（`maxBackups`）。
- 损坏加载：仅告警 + 当空，**不**自动备份（区别于旧 `backupCorrupt`）。
- 升级同步跳过集：`existingIDs ∪ deletedBreeds`。
- 凭证根 env 覆盖：`SOUNDS_GREAT_AI_CREDENTIAL_ROOT`；配置根 env 覆盖：`SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT`（两者独立）。

---

## 5. 验收标准 (AC)

- [x] **AC-01 (根分离)**: 生产设 `workspaceDir` 后，`credentials.json` 落 `{home}/.sounds-great-ai`，`dog-catalog.json`/`accounts.json` 落 `{workspaceDir}/.sounds-great-ai`；两者均不受对方 env 覆盖牵连（独立根）。
- [x] **AC-02 (编辑时备份)**: 对任意配置文件做二次编辑写盘，`<path>.bak-<时间戳>` 存在且含编辑前内容；加载期放入损坏文件不产生 `.bak`（仅 WARN 日志）。
- [x] **AC-03 (升级追加不复活)**: 给老客户 catalog 追加一个新模板犬后重启 → 新犬出现；该客户此前删过的模板犬重启后**不**出现，`ListDeletedBreeds` 含该 id。
- [x] **AC-04 (无回归)**: `go build ./...` 0、`go test ./...` 0（含 `internal/settings`、`internal/platform`、`cmd/server`）；`tsc -b` / `vite build` 未改前端故沿用前次绿灯。

---

## 6. 工程护栏

- **[x] 客户数据不删**：全仓库仍无 `os.Remove`/`RemoveAll` 客户目录逻辑；`deleted_breeds` 仅作「跳过集」，不物理删 catalog 条目之外的内容。
- **[x] 密钥隔离**：凭证独立根 + 0600，与 catalog/accounts 解耦；清项目配置不丢密钥。
- **[x] 可回滚**：编辑前时间戳 `.bak` 提供客户级人工回滚点；`pruneBackups` 防无限增长。

---

## 7. DoD

- [x] `CredentialRoot()` + `routes.go` 凭证根切换落地。
- [x] `backupBeforeWrite`/`pruneBackups` 落地，加载期 `backupCorrupt` 移除。
- [x] `SyncTemplateBreeds` + `ListDeletedBreeds` + `deleted_breeds` 落地（File/InMemory/Adapter 实现齐全）。
- [x] 回归测试：`internal/settings/file_store_test.go`（`TestFileStore_CorruptAccountsTreatedAsEmpty` / `TestFileStore_CorruptCredentialTreatedAsEmpty` / `TestFileStore_EditCreatesTimestampedBak`）、`internal/platform/breeds_merge_test.go`（`TestSyncTemplateBreedsSeedsEmpty` / `TestSyncTemplateBreedsIdempotent` / `TestSyncTemplateBreedsSkipsDeleted` / `TestMergedBreedsCatalogWins`）。
- [x] `go build ./...` + `go test ./...` 全绿。
- [x] 关联 story（`SG-MEM-001` / `SG-ACC-001`）同步。

---

## 8. 修订记录

- **2026-08-12（初版）**：汇总「客户配置安全」三项决策与实现（根拆分 / 编辑时 .bak / 追加式升级同步）。设计取舍：备份仅在编辑时生成、加载期损坏不自动备份（更早发现损坏、不掩盖问题）；密钥独立到全局 home 与项目配置解耦，清项目配置不丢密钥。
