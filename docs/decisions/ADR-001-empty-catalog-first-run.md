# ADR-001: 首启空 Catalog、成员按需添加 + 凭据就绪闸门

## Status
Accepted（2026-08-13 决策；后端与前端已于 2026-08-13 落地，见 `docs/DESIGN-STORYS/SG-MEM-001-member-management.md` §6）

## Context
用户反馈：首启未配密钥却有满屏「已启用」的狗。根因：`breeds_merge.go` 首启把 `dog-template.json` 全部模板犬 `Enabled=true` 注入 `dog-catalog.json`；且 `RosterEntry.Available = breed.Enabled`，与密钥无关。这与 clowder-ai 的「模板=菜单、catalog=成员、首启空、按需添加」模型不符。

## Decision
1. **首启空 catalog**：全新安装 `dog-catalog.json` 为空（仅 owner），不注入任何模板犬；犬只经「成员管理 → 从模板添加」进入运行时。已有实例不被清空。
2. **模板退化为菜单**：`dog-template.json` 保留完整 7 犬 / 14 变体作为可选菜单；`MergedBreeds` 仅返回 catalog 犬，template 不再并入 active registry。
3. **`seen_template_breeds` 机制**：持久化"已暴露模板犬 ID"集合。首启 `seen`=全部模板 ID（故空）；升级时不在 `seen` 的新模板犬自动加入 catalog 并记入 `seen`；已见但未添加的犬不复活。`deleted_breeds` 语义并入 `seen`。
4. **凭据就绪闸门**：新增派生状态 `credential_ready`，有效可用性 = `Available ∧ credential_ready`。`oauth` 账号查 CLI 二进制（`exec.LookPath`）；`api_key` 账号查 `~/.sounds-great-ai/credentials.json`（`FileCredentialStore.Has`）；无绑定 → false。前端三态「就绪 / 待配置 / 已停用」。
5. **路由空名册友好报错**：`router.go` 空 patterns 不再回落 `bianmu`，返回空目标 + 明确提示。

## Consequences
- 正面：首启不再出现"无密钥却开启"的犬；符合"养的是团队、按需组队"的产品定位。
- 架构：`VISION.md` 新增 §5.1 定位说明；§4.3「Dog personas 保留」不变（六犬仍为菜单 personas）。
- 测试：`breeds_merge_test.go` 的"种子 Enabled"回归守卫重写；新增空 roster 与 `credential_ready` 用例。
- 既有用户：仅全新安装走空初始化；升级后新模板犬自动加入但默认「待配置」。
- 已知限制：OAuth 仅查二进制存在、不校验登录态（首次执行才报错）。
