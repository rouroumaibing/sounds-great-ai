# 愿景合规（Vision Compliance）

> 本文件承接原 `docs/VISION.md` §8，2026-08-17 拆出。
> VISION 现只保留方向与理念（§0–§3）；此处专述「如何保证工作与 VISION 兼容」的治理流程。

**不论是否走 spec 流程，所有工作必须与 VISION 兼容。** agent 不一定写 spec——直接改代码也受约束。

## 8.1 走 spec 流程时

Spec 必须包含 `## VISION Compatibility` 段，回答以下 7 个问题：

- [ ] 与 §0 哲学兼容？（服务用户实现力，不是替代用户）
- [ ] 与 §1 协作哲学兼容？（对等协作，非 Boss Agent）
- [ ] 与 §2 治理原则兼容？（愿景驱动、Phase 碰头、风险路由）
- [ ] 与 §3 三层原则兼容？（不把推理放进平台层）
- [ ] 与 `docs/governance/decisions/irreversible-decisions.md` 不可逆决策兼容？（不重新引入 DAG / 内置 reasoning / 非 CLI 架构）
- [ ] 在 `docs/ROADMAP.md` 路线图的哪个 Phase？该 Phase 的前置条件已满足？
- [ ] 是否引入新的不可逆决策？如果是，更新 `docs/governance/decisions/irreversible-decisions.md`。

## 8.2 不走 spec 流程时（直接改代码）

遵守 `AGENTS.md` 的 Vision Check Protocol：

- **开工前**：读 VISION.md（§0–§3）+ `docs/governance/decisions/irreversible-decisions.md`，4 项兼容性自检。不兼容时停下报告。
- **提交前**：检查代码结构 vs `docs/governance/decisions/irreversible-decisions.md` / `docs/architecture/platform-capabilities.md` / `docs/ROADMAP.md`。不兼容时不提交。
- **架构变更**：必须先更新对应文档（`docs/governance/decisions/irreversible-decisions.md` 或 `docs/architecture/platform-capabilities.md`）。未更新就改 = 违反铁律 5。

## 8.3 执行机制

| 机制 | 覆盖场景 | 状态 |
|------|----------|------|
| `AGENTS.md` Vision Check Protocol | agent 直接改代码 | **已建立** |
| Spec 模板 + §8.1 检查清单 | agent 走 spec 流程 | 待建立模板 |
| Prompt hooks（per-turn 注入） | 每轮自动注入身份 + 铁律 + 护栏 | **已实现**（§8.4） |

**已有经验**：最有效的机制是 hooks（`disableable: false`, `governanceTier: immutable`）——系统注入，agent 无法绕过。我们当前靠 AGENTS.md 自觉，未来应建立 hook 系统实现结构化强制。

## 8.4 Prompt Hooks 系统设计（已实现）

> 实现路径：`internal/hooks/` + `packs/default/hooks/`（46 个 hook 模板）。

**目标**：在 CLI adapter spawn 时，通过 stdin 注入结构化 prompt 片段，实现 agent 无法绕过的身份 + 铁律 + 护栏注入。

**注入时机**：
- `session-init`：会话启动时注入身份、铁律、限制声明
- `per-turn`：每轮注入红旗模式、Phase 约束

**核心 hook 清单**：

| hook id | 时机 | 内容 | 参考实现 |
|---------|------|------|----------------|
| `identity` | session-init | 狗狗身份 + 职责 | S1 |
| `restrictions` | session-init | 限制声明表 | S2 |
| `iron-laws` | session-init | 5 条铁律 | L4 |
| `guardrails` | session-init | 红旗模式表 | S10 |
| `roster` | session-init | 队友名册 | S5 |
| `governance` | session-init | 治理摘要（VISION §0-§3 + `docs/governance/decisions/irreversible-decisions.md`） | S9 |
| `mcp-tools` | session-init | MCP 工具列表 | S13 |
| `a2a-format` | session-init | A2A 协作格式 | S4 |
| `phase-anchor` | per-turn | 当前 Phase + 前置条件 | D14 |
| `re-anchor` | per-turn | 长任务重锚定提醒 | D1 |

> **per-turn hooks 已接入**：session-init hooks（S 系列）在会话启动时注入身份、铁律、限制、护栏、名册、治理、MCP 工具、A2A 格式。per-turn hooks（D 系列）每轮注入 Phase 锚定和重锚定提醒。session-init hooks 通过 native L0 通道（Claude `--append-system-prompt`、Codex `-c developer_instructions`）注入，压缩免疫；Gemini/OpenCode 走 stdin 前插 fallback。TraceStore（SQLite）记录每次注入的 fire/skip 事件。

**实现路径**：
1. `internal/hooks/` Registry 扫描 `packs/default/hooks/` 下的 hook.yaml 定义
2. Pipeline 按 `stage` 和 `order` 过滤并执行适用的 hooks
3. `ws_handler.go` 执行 session-init + per-turn hooks，按 CLI 类型路由注入
4. Native L0：Claude `--append-system-prompt`、Codex `-c developer_instructions`（压缩免疫）
5. Fallback：Gemini/OpenCode 走 stdin 前插
6. `disableable: false` 的 hook 不可被 agent 跳过
7. `TraceStore`（SQLite）记录每次注入的 fire/skip 事件

**与 AGENTS.md 的关系**：AGENTS.md 是 hook 内容的**真相源**。hook 系统实现后，AGENTS.md 的"长任务重锚定"段从自觉规则升级为系统注入。
