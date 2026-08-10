# Sounds Great AI 开发 SOP

> 犬队开发全流程的导航图。每步的详细操作在对应 skill 内。
> 本文件保留人类可读叙事；机器真相源是 `internal/sop/` 中的 SOP 定义。
> 冲突时先修 SopDefinition 单一源，再同步本文件和相关 skill。

## 愿景驱动（核心原则）

Sounds Great AI 的开发是**愿景驱动**的。和用户确认了 feature 的愿景后：

- **没达成愿景 = 没完成**，不交半成品，不半路问"要不要继续"
- 停下来的正当理由：解决不了的阻塞（技术限制/外部依赖）→ 升级用户；方向存疑 → 停手重估
- 判断力允许停，惰性不允许

### 大 Feature 碰头机制（3+ Phase）

大 scope feature 不能等最后才对齐愿景。**每个 Phase merge 后**，主动和用户碰头：

```
Phase N merge → 碰头（不是"要不要继续"，是"方向对不对"）→ 继续 Phase N+1
```

**碰头格式**（轻量，不是报告会）：
1. **成果展示**：这个 Phase 做了什么
2. **愿景进度**：离最终愿景还差什么
3. **下个 Phase 方向**：下一步计划，有没有发现新问题
4. **方向确认**："方向对吗？有没有要调整的？"

**注意区别**：
- 碰头 = **愿景方向确认**（宏观层，用户需要介入）✅
- "要我继续吗？" = **SOP 流程推进**（细节层，不要问）❌

**小 Feature（1-2 Phase）**：不需要碰头，直接做到底 → 愿景守护 → close。

## Risk-Routed Development：铁路改立交

**强制力跟着风险走，不跟着动作类型走。** "写了代码""开了 PR""进入 merge"都不能单独触发整条流水线。默认是最小安全动作；只有命中客观风险面才进入对应加严车道。

### 入口：五轴风险判断

| 风险轴 | 命中信号 | 最低动作 |
|---|---|---|
| 行为面 | 用户可见行为、runtime 逻辑、bug 回归 | 可观察 RED + targeted 验证；方向未定才进 Design Gate |
| 数据 | 持久化数据、迁移、存储语义 | full gate + 独立高风险 review |
| 安全 | auth、权限、secret、注入、命令边界 | full gate + 安全扫描；zhonghuatianyuanquan 审查 |
| 契约 | API schema / MCP 工具签名 / CLI 输出格式 | 契约测试 + full gate + 对应独立 review |
| 不可逆 | 删除、force push、close feat、圣域（`internal/memory/`、`internal/ragstore/`） | 先拿用户授权；机器门禁仍照常 |

**元风险强制升档**：diff 触碰 `internal/sop/` 门禁逻辑或 VISION.md 不可逆决策时，直接进入 high-assurance，由非作者跨狗狗 reviewer 覆盖。

五轴都未命中且改动可逆、无外部副作用 → 最小安全动作。信息不足不等于自动全套：先补查缺失事实，再按真实风险选车道。

### 按需车道

| 车道 | 触发条件 | 不因什么触发 |
|---|---|---|
| Design Gate / kickoff | 新 feature、架构方向未定、价值取舍 | 每个实现任务 |
| `writing-plans` | 跨组件、状态对象、实现顺序不清 | 文件超过 5 行 |
| `worktree` | tracked code、skill、SOP definition 需要隔离 | 纯 docs 已自判 light |
| `tdd` | 新行为、bug、未被现有精确检查覆盖的逻辑 | 确定性生成物刷新 |
| targeted self-check | 所有交付；命令按风险面选 | 为了"报告完整"跑无关全仓测试 |
| local peer | 家里语境、治理 / skill / SOP、实现语义 | 已选择 cloud 仍固定叠一层 |
| cloud review | 安全、数据、契约或陌生跨包代码需要 context-blind 扫描 | 普通 test / PR 载体本身 |
| merge-gate | PR / branch policy 需要合入 | 自动重跑 local + cloud 全套 |
| 愿景守护 | 用户可见或愿景变化的 feature close | 每个 PR、纯机械内部 change |

### Review 去叠加

默认选择**一个合适的独立验证源**，且必须是非作者：

- local peer 看家里语境与 stateful diff；
- cloud 看 context-blind 高风险代码面；
- 愿景守护看最终产品结果，只在 feature close 触发。

只有不同风险面确实需要不同视角时才叠加，并分别写明触发理由。

### Sol 测试

任何存活的 `must` 都问：**一只完美遵从的犬 100% 执行后，系统是否仍然更好？** 不能稳定回答"是"的条款应删除或降为建议。

## Reviewer 配对规则

动态匹配自运行时狗狗配置（`packs/default/breeds/dog-template.json`）：
1. 跨狗狗优先 | 2. 必须有 peer-reviewer 角色 | 3. 必须 available
4. 优先 lead | 5. 优先活跃犬

**降级**：无跨狗狗 reviewer → 同狗狗不同个体 → 用户。
**铁律**：同一个体不能 review 自己的代码。

## 代码质量工具

| 工具 | 命令 | 何时 |
|------|------|------|
| Go build | `go build ./...` | 开发中 + 提交前 |
| Go test | `go test ./...` | 提交前必跑 |
| Go vet | `go vet ./...` | 提交前 |
| 文件大小 | 200 行警告 / 350 行硬限 | 新增文件时 |

## 约定面改动预检

改 CLI adapter 接口、skill manifest、路由规则、MCP 工具签名等约定面前，先查影响面，避免只靠 grep 漏掉注册链或动态消费方。

## 例外路径

### Co-Creation Docs Lane（按风险，不按行数）

适用于用户与犬共同审阅、修订并授权落盘的 docs-only 内容。只请 review 没有授权落盘时保持只读，不进入任何交付 lane。

显然满足"docs-only、无执行面、无已知并发冲突、单 commit 可逆"的轻量改动，由犬直接自判 `direct_push`；不要求先开 worktree、扫全量 PR。

**行数不是路由条件。** 不能用"超过 5 行"把安全共创文档机械送入完整开发链，也不能用"小 diff"掩盖敏感面。

### 最小安全 direct-main

仅在既有 lane 明确允许 direct push 时使用。必须同时满足可逆、无行为变化、无数据 / 安全 / 契约 / 不可逆面，并有精确机器检查。

代码与第一方执行面仍用 PR；pure docs 先走 Co-Creation Docs Lane。

## 文档规范

- `docs/` 下 `.md` 文件推荐有 YAML frontmatter
- 完成后必须同步真相源
- 教训沉淀用 LL-XXX 格式（见 `docs/public-lessons.md`）

## 环境变量注册

新增环境变量引用 → 必须在配置注册表中登记，否则用户看不到 = 不存在。

---

> **冲突时先修 SopDefinition 单一源，再同步本文件。**
