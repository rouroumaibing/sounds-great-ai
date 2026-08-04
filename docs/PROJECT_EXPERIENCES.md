# PROJECT EXPERIENCES — Sounds Great AI

> 2026-07-29 ~ 2026-08-04 | 7 specs | 6 days | 1 architectural pivot
>
> 本文档总结开发过程中的经验教训，防止 recurrence。

---

## 1. 时间线

| 日期 | Spec | 做了什么 | 后续命运 |
|------|------|----------|----------|
| 07-29 | Backend Core (Phase 1-3) | Go+Eino, WebSocket, workspace, hard rails | **保留** |
| 07-30 | A2A Multi-Agent | A2A protocol, agent server/client, orchestrator | **废弃** → CLI adapter 取代 |
| 07-31 | Pack/Breed System | 犬种 JSON + **固定 DAG** + capability 适配器 | **半废弃** → DAG 消除, JSON 保留 |
| 07-31 | Server-Pack Integration | pack 接入 server | **重写** |
| 08-01 | LLM Code Search | code_search capability | **保留** |
| 08-03 | Jinmao RAG | 向量存储 + embedding + 迁移 | **保留** |
| 08-03 | Multi-breed Coordination | 9 capabilities + dispatch executor | **全部废弃** |
| 08-04 | Clowder-AI Restructuring | CLI adapter + 动态路由 + SOP + skills | 当前 |

## 2. 偏离根因

### 2.1 参考架构反转

起始：*"借鉴 clowder-ai 的安全概念"* → 终局：*"完全对齐 clowder-ai 架构"*

从"借模式"变成"抄架构"，参考关系反转。所有不符合 clowder-ai 模式的设计被判定为需重构。

### 2.2 架构决策被直接推翻

| 07-31 决策 | 08-04 决策 | 性质 |
|------------|------------|------|
| 固定 workflow DAG | 动态路由，无 DAG | 180° 反转 |
| A2A server/client (HTTP+JSON-RPC) | CLI adapter (stdin/stdout pipe) | 180° 反转 |
| capability 适配器架构 | backup + 选择性恢复 | 废弃 |
| 内置 LLM 调用 (Eino) | 外部 CLI 做 reasoning | 职责迁移 |

### 2.3 阶段顺序违反

原计划 7 phase 有明确依赖。实际把 Phase 4 (multi-agent) 和 Phase 7 (memory/RAG) 提前到 Phase 5/6 之前。在未稳定的地基上盖复杂楼层。

### 2.4 无整合检查点

每个 spec 独立设计、独立实现、独立批准。没有节点回退看整体："零件拼在一起 coherent 吗？" 到 08-03 发现不 coherent 时，已积累 145 个 Go 文件。

### 2.5 Spec 精良但缺 vision 约束

每个 spec 单看都详尽（RAG 66KB, pack/breed 30KB），但缺少顶层 vision 约束设计空间。没有文档说"我们最终做 CLI adapter 架构"，所以 A2A 设计和 DAG 设计各自合理，但与 08-04 方向完全矛盾。

## 3. 量化影响

```
145 Go 文件 (internal/ + pkg/)
├── ~40% 重构后新增 (adapter, config, router, sop, skills, mcp, memory, platform)
├── ~30% 保留 (backend core, RAG, 部分 capabilities)
└── ~30% 经历重写或废弃 (a2a server/client, orchestrator, DAG, dispatch executor)
```

废弃代码：A2A (6 文件) + orchestrator chain (15+ capability 文件) + dispatch executor ≈ 30 文件

## 4. 保留 / 废弃清单

### 保留

| 代码 | 原因 |
|------|------|
| `internal/transport/` | WebSocket 传输层，通用基础设施 |
| `internal/workspace/` | 文件系统操作，通用 |
| `internal/tool/` | 工具注册，通用 |
| `internal/aspect/` | 安全切面，通用 |
| `internal/component/` | Eino model 封装，通用 |
| `internal/ragstore/` | RAG 向量存储，已实现完整 |
| `internal/capability/code_search*` | 代码搜索，有独立价值 |
| `internal/capability/llm_chat*` | LLM 调用封装 |
| `pkg/pack/` | pack 系统（breed/capability/workflow），已重构 |

### 废弃

| 代码 | 原因 |
|------|------|
| `internal/a2a/server/`, `client/`, `orchestrator/` | 被 CLI adapter 取代 |
| `internal/capability/` 中 9 个 orchestrator chain capabilities | 被 skills + 动态路由取代 |
| `pkg/pack/dispatch_executor*` | 固定 DAG 执行器，已消除 |
| `pkg/pack/orchestrator/` | 旧编排器类型 |

### 重构后新增

| 代码 | 职责 |
|------|------|
| `internal/adapter/{claude,codex,gemini,opencode,unified}/` | CLI adapter 层 |
| `internal/config/` | breed/roster 配置系统 |
| `internal/router/` | 动态路由 |
| `internal/sop/` | SOP 门禁 |
| `internal/skills/` | skill 加载（yaml.v3, 目录格式） |
| `internal/mcp/` | MCP 桥接 |
| `internal/memory/` | 证据存储 |
| `internal/platform/` | 平台层入口 |

## 5. 可执行原则

### P1: Vision-First

> **任何 spec 批准前，先检查与 `docs/VISION.md` 兼容性。**

VISION.md 是北极星。spec 必须回答：与三层原则兼容？与不可逆决策兼容？在路线图哪个 Phase？

### P2: Integration Checkpoint

> **每完成 2 个 spec，做一次整合检查：零件拼在一起 coherent 吗？**

不是做完所有 spec 才检查。每 2 个 spec 后回退一步看整体。

### P3: No Architecture Reversal

> **架构决策一旦写入 VISION.md 的"不可逆决策"，不得在后续 spec 中推翻。**

如果确实需要推翻，必须先更新 VISION.md 并标注"决策变更 + 理由 + 影响范围"。

### P4: Phase Ordering

> **Phase N 的前置条件必须全部满足后，才能开始 Phase N+1 的 spec。**

不允许跳 phase。如果需要提前，必须在 VISION.md 中标注"Phase X 提前 + 理由"。

### P5: Spec Compatibility Check

> **新 spec 必须列出与现有代码的交互：复用哪些、替代哪些、新增哪些。**

不能假装现有代码不存在。每个 spec 开头有一个"与现有代码关系"段落。

## 6. 技术经验

### 6.1 Skill 框架升级

**问题**：原 skill 解析器只支持 `id/name/description/trigger` 单值格式，无法加载 clowder-ai 的 `triggers` 数组和多行 `description`。

**解决**：用 `yaml.v3` 替换手写解析器，支持两种格式（扁平 .md + 目录 SKILL.md），`id` 缺失时从 `name` 回退。

**教训**：解析器不要手写 YAML，用成熟库。格式设计要考虑兼容外部生态。

### 6.2 分支管理

**问题**：`feature/multi-breed-coordination` 分支积累 30 个 commit，全部基于旧架构。重构后完全冗余。

**解决**：分析确认无新框架需要的代码后，直接删除分支。

**教训**：架构 pivot 后，旧分支应尽快评估和清理，避免混淆。

### 6.3 参考代码引入

**问题**：clowder-ai 的 skill 内容深度绑定其专有概念（cats/breeds/operator/MCP 工具名/F 编号/端口）。

**解决**：全量适配——保留核心方法论，去掉专有引用，用通用概念替换。

**教训**：引入外部参考时，区分"方法论"（可复用）和"实现细节"（不可复用）。

## 7. 教训条目（LL-XXX 格式）

> 对标 clowder-ai `docs/public-lessons.md`。格式：`LL-XXX`（三位数字，递增），已发布 ID 不重排、不复用。
> 状态：`draft | validated | archived`
> 质量门槛：有来源锚点、有可执行防护、有时效性验证。

### LL-001: spec 批准前必须检查 VISION.md 兼容性
- 状态：validated
- 更新时间：2026-08-04

- 坑：spec 精良但缺顶层 vision 约束，各自合理但拼在一起与最终方向矛盾。
- 根因：没有文档说"我们最终做 CLI adapter 架构"，A2A 设计和 DAG 设计各自合理但与 08-04 方向完全矛盾。
- 触发条件：多个 spec 独立设计、独立实现、独立批准，无统一 vision 约束。
- 修复：创建 VISION.md 不可逆决策，spec 必须填 VISION Compatibility 段。
- 防护：AGENTS.md 时刻 1 自检 + spec 模板 §8.1 检查清单。
- 来源锚点：`docs/VISION.md` §4 + `docs/PROJECT_EXPERIENCES.md` §2.5

- 关联：P1 Vision-First

### LL-002: 架构决策写入不可逆后不得在后续 spec 中推翻
- 状态：validated
- 更新时间：2026-08-04

- 坑：07-31 决策（固定 DAG、A2A server/client）在 08-04 被 180° 反转。
- 根因：没有"不可逆决策"机制，每个 spec 可以自由推翻前一个 spec 的架构决策。
- 触发条件：新 spec 认为旧架构不合理，直接推翻而非先更新 vision。
- 修复：VISION.md §4 锁定 4 个不可逆决策。
- 防护：AGENTS.md 红旗模式表 + 时刻 3 架构变更检查。
- 来源锚点：`docs/VISION.md` §4 + `docs/PROJECT_EXPERIENCES.md` §2.2

- 关联：P3 No Architecture Reversal

### LL-003: Phase N 前置条件必须满足后才能开始 Phase N+1
- 状态：validated
- 更新时间：2026-08-04

- 坑：把 Phase 4 (multi-agent) 和 Phase 7 (memory/RAG) 提前到 Phase 5/6 之前，在未稳定的地基上盖复杂楼层。
- 根因：7 phase 有明确依赖但未强制执行顺序。
- 触发条件：觉得某个 Phase "看起来可以独立做"。
- 修复：VISION.md §7 路线图标注前置条件。
- 防护：AGENTS.md 红旗模式"跳 Phase" + 时刻 2 提交前检查。
- 来源锚点：`docs/VISION.md` §7 + `docs/PROJECT_EXPERIENCES.md` §2.3

- 关联：P4 Phase Ordering

### LL-004: 每完成 2 个 spec 做一次整合检查
- 状态：validated
- 更新时间：2026-08-04

- 坑：每个 spec 独立设计，到 08-03 发现不 coherent 时已积累 145 个 Go 文件。
- 根因：没有节点回退看整体——"零件拼在一起 coherent 吗？"
- 触发条件：连续做 3+ 个 spec 而不检查整体一致性。
- 修复：每 2 个 spec 后做整合检查。
- 防护：VISION.md §2.2 Phase 碰头机制 + P2 原则。
- 来源锚点：`docs/PROJECT_EXPERIENCES.md` §2.4

- 关联：P2 Integration Checkpoint

### LL-005: 新 spec 必须列出与现有代码的交互
- 状态：validated
- 更新时间：2026-08-04

- 坑：spec 假装现有代码不存在，导致重复实现或接口冲突。
- 根因：没有"与现有代码关系"段落要求。
- 触发条件：新 spec 只描述新功能，不列复用/替代/新增。
- 修复：spec 开头必须有"与现有代码关系"段落。
- 防护：P5 原则 + spec 模板要求。
- 来源锚点：`docs/PROJECT_EXPERIENCES.md` §2.5

- 关联：P5 Spec Compatibility Check

### LL-006: 解析器不要手写 YAML，用成熟库
- 状态：validated
- 更新时间：2026-08-04

- 坑：原 skill 解析器只支持单值格式，无法加载 clowder-ai 的 `triggers` 数组和多行 `description`。
- 根因：手写 YAML 解析器覆盖不了外部生态的格式变体。
- 触发条件：需要兼容外部 YAML 格式时。
- 修复：用 `yaml.v3` 替换手写解析器。
- 防护：格式设计考虑兼容外部生态；优先用成熟库而非手写。
- 来源锚点：`internal/skills/skill.go`

- 关联：§6.1 Skill 框架升级

### LL-007: 架构 pivot 后旧分支应尽快评估和清理
- 状态：validated
- 更新时间：2026-08-04

- 坑：`feature/multi-breed-coordination` 分支积累 30 个 commit，全部基于旧架构，重构后完全冗余。
- 根因：架构 pivot 后未及时清理旧分支，造成混淆。
- 触发条件：架构方向 180° 反转后。
- 修复：分析确认无新框架需要的代码后，直接删除分支。
- 防护：架构 pivot 后立即评估旧分支。
- 来源锚点：`docs/PROJECT_EXPERIENCES.md` §6.2

- 关联：§6.2 分支管理

### LL-008: 引入外部参考时区分方法论和实现细节
- 状态：validated
- 更新时间：2026-08-04

- 坑：clowder-ai 的 skill 内容深度绑定其专有概念（cats/breeds/operator/MCP 工具名/F 编号/端口），直接迁移不可用。
- 根因：未区分"方法论"（可复用）和"实现细节"（不可复用）。
- 触发条件：引入外部参考代码/文档时。
- 修复：全量适配——保留核心方法论，去掉专有引用，用通用概念替换。
- 防护：引入外部参考前先做方法论 vs 实现细节分类。
- 来源锚点：`docs/PROJECT_EXPERIENCES.md` §6.3

- 关联：§6.3 参考代码引入
