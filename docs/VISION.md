# VISION — Sounds Great AI

> **北极星文档。所有 spec 必须与本文档兼容，否则不得批准。**
>
> Status: Active | Last Updated: 2026-08-17

---

## 0. 为什么存在

### 0.1 想法和产品之间，隔着的不是程序员，是实现力

每个人脑子里都有想做的东西。卡住的从来不是想法，而是**把想法做成可演示产品的能力**。

以前传递想法靠 PPT。我们相信未来传递想法靠 POC、demo、真正能跑的东西。**Coding 是这个时代最直接的实现力。**

我们缺的不是更强的 AI 工具，我们缺的是一支能把想法变成产品的团队。

### 0.2 我们养的不是工具，是团队

人是作者，犬是共创放大器。梦是人的，判断是人的，愿景是人的。
犬做的是把你从"有想法但做不出来"，推到"能带着团队把它做出来"。

领养团队，不是配置工具。你和犬一起磨出来的 shared-rules、踩过的坑、沉淀的 skills，这些加在一起才是真正的生产力。

> *每一个词元（token）的价值，不以消耗衡量，以创造丈量。*

### 0.3 AI 不是让你退场，而是让你上场

Sounds Great AI 不是替你完成梦想，而是让你终于有机会带着自己的团队，把梦想做成能运行的东西。

以前没有资源、没有团队、没有舞台。现在犬队陪你一起把它做出来，带着作品走进人群。
**犬队不是让你远离人群——是让你终于有东西可以带着走上舞台。**

### 0.4 核心信条

> **Models set the ceiling. The platform sets the floor. Each layer is a multiplier, not addition.**
>
> Go + Eino 平台协调多个外部 CLI agent（Claude/Codex/Gemini/opencode 等），让它们像犬队一样精确协作。

---

## 1. 协作哲学

### 1.1 没有 Boss Agent

没有 Boss Agent。六只犬各有视角，自己判断该不该回应、怎么回应。
但执行有纪律：TDD、Review、门禁、愿景守护——**自由判断，结构化交付**。

### 1.2 对等协作

多犬协作不是重复劳动。不同认知风格的碰撞——归纳、发散、演绎——涌现出单一视角无法产生的洞见。
衡量团队的标准不是消耗了多少 token，而是从想法到产品之间，省掉了多少次推倒重来。

### 1.3 共享记忆，可迁移的养成经验

养成经验可以迁移。我们磨了五十多天的协作经验打包给你是 80 分，但犬会和你继续长——长成属于你自己的 100 分。
**每个人的 100 分不一样。**

### 1.4 犬队 vs 猫咖

我们用「犬队」隐喻组织工具型 agent 家族，而非沿用「猫咖」设定。原因：
- 犬有**工作本能**（herding, retrieving, guarding）——更贴合"工具型 agent"
- 犬有**团队意识**（pack）——多犬协作是天性
- 犬有**服从性**——纪律执行更可靠

---

## 2. 治理原则

### 2.1 愿景驱动

开发是**愿景驱动**的。和用户确认了 feature 的愿景后：

- **没达成愿景 = 没完成**，不交半成品，不半路问"要不要继续"
- 停下来的正当理由：解决不了的阻塞（技术限制/外部依赖）→ 升级用户；方向存疑 → 停手重估
- 判断力允许停，惰性不允许

### 2.2 Phase 碰头机制

大 scope feature（3+ Phase）不能等最后才对齐愿景。**每个 Phase merge 后**，主动和用户碰头：

```
Phase N merge → 碰头（不是"要不要继续"，是"方向对不对"）→ 继续 Phase N+1
```

**碰头格式**（轻量，不是报告会）：
1. **成果展示**：这个 Phase 做了什么
2. **愿景进度**：离最终愿景还差什么
3. **下个 Phase 方向**：下一步计划，有没有发现新问题
4. **方向确认**："方向对吗？有没有要调整的？"

小 Feature（1-2 Phase）：不需要碰头，直接做到底 → 愿景守护 → close。

### 2.3 风险路由

**强制力跟着风险走，不跟着动作类型走。** "写了代码""开了 PR""进入 merge"都不能单独触发整条流水线。默认是最小安全动作；只有命中客观风险面才进入对应加严车道。

### 2.4 教训沉淀

踩过的坑必须沉淀为可复用、可追溯的教训条目（LL-XXX 格式），避免重复踩坑。
教训必须有**可执行防护**——不能只写"注意"，必须有可执行动作。

---

## 3. 三层原则

```
+--------------------------------------------------+
|                  You (CVO)                       |
|         Vision - Decisions - Feedback            |
+----------------------+---------------------------+
                       |
+----------------------v---------------------------+
|            Dog Pack Platform Layer (Go+Eino)     |
|                                                  |
|  Identity Manager | A2A Router | Skills Manifest |
|  Memory & Evidence| SOP Guardian| MCP Bridge     |
|  Dynamic Routing  | Review Gates| CLI Adapter    |
+----+----------+----------+----------+------------+
     |          |          |          |
+----v---+ +---v----+ +---v----+ +---v------+ +---v------+
|Claude  | |Codex   | |Gemini  | |opencode  | |Kimi CLI  |
|Code CLI| |CLI     | |CLI     | |(any model)| |          |
+--------+ +--------+ +--------+ +----------+ +----------+
```

| 层 | 职责 | 不职责 |
|---|------|--------|
| **Model**（CLI 内） | 推理、生成、理解 | 长期记忆、纪律 |
| **Agent CLI** | 工具调用、文件操作、命令执行、MCP | 团队协调、review |
| **Platform**（Go+Eino） | 身份、协作、纪律、审计、路由 | 推理（那是 CLI 的事） |

> CLI 白名单见 `internal/settings/validation.go` 的 `ValidCLIClientIDs`：claude / codex / gemini / opencode / kimi 共 5 个。其中 kimi 已入白名单且 adapter 已实现（`internal/adapter/kimi`）；新增 CLI 需同时有 adapter 与白名单登记。

---

## 其余内容已拆分至专门文档

VISION 本文件只承载**方向与理念**（§0–§3）。以下主题已按需迁出，便于独立维护：

- **§4 不可逆决策**（锁定决策 / ADR）→ `docs/governance/decisions/irreversible-decisions.md`
- **§5 狗狗角色 + §6 平台能力清单**（成员与模块目录）→ `docs/architecture/platform-capabilities.md`
- **§7 路线图**（Phase 进度）→ `docs/ROADMAP.md`
- **§8 愿景合规**（spec 检查清单 / Prompt Hooks / 与 AGENTS.md 关系）→ `docs/governance/vision-compliance.md`

> 引入新的不可逆决策时，更新 `docs/governance/decisions/irreversible-decisions.md` 而非本文件。

---

> **当 agent 完美完成一次协作，终端亮起绿色爪印：Sounds Great!**
