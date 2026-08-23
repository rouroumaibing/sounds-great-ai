---
topics: [sop]
doc_kind: note
---

# Sounds Great AI 开发 SOP

> 狗狗队伍开发全流程的导航图。每步的详细操作在对应 skill 内。
> Stage id / suggested skill / hard rules / pitfalls 的机器真相源是
> `packs/default/sop/development.yaml`；本文件保留人类可读叙事。
> 冲突时先修 SopDefinition 单一源，再同步本文件和相关 skill。

## 愿景驱动（核心原则）

Sounds Great AI 的开发是**愿景驱动**的。和 operator 确认了 feature 的愿景后：

- **没达成愿景 = 没完成**，不交半成品，不半路问"要不要继续"（决策漏斗见 AGENTS.md）
- 停下来的正当理由：解决不了的阻塞（技术限制/外部依赖）→ 升级 operator；方向存疑（坐标系警报、scope 该砍）→ 停手重估。判断力允许停，惰性不允许

### 大 Feature 碰头机制（3+ Phase）

大 scope feature 不能等最后才对齐愿景。**每个 Phase merge 后**，主动和 operator 碰头：

```
Phase N merge → 碰头（不是"要不要继续"，是"方向对不对"）→ 继续 Phase N+1
```

**碰头格式**（轻量，不是报告会）：
1. **成果展示**：这个 Phase 做了什么（截图 / 关键改动 / demo）
2. **愿景进度**：离最终愿景还差什么（哪些 AC 打了勾，哪些还没）
3. **下个 Phase 方向**：下一步计划做什么，有没有发现新问题
4. **方向确认**："方向对吗？有没有要调整的？"

**注意区别**：
- 碰头 = **愿景方向确认**（宏观层，operator 需要介入）✅
- "要我继续吗？" = **SOP 流程推进**（细节层，不要问）❌

**小 Feature（1-2 Phase）**：不需要碰头，直接做到底 → 愿景守护 → close。

## 运行时单实例保护（P0）

`make prod daemon`（或 `make dev daemon`）拉起的服务是咱们的运行态单实例（backend `:8080` + frontend `:5173`），默认视为**在线服务**，不是随手重启的实验环境。

硬规则：
1. 在运行时会话里，禁止执行会触发重启服务的命令：直接 `make prod` / `make dev` 前台重启、或手动 `kill` 服务进程。
2. 做截图/验收/排查前，先复用现有服务（先查 `curl -sf http://localhost:8080/health`）。
3. 确实要重启，必须先拿到 operator 明确同意，再用 `make stop` 干净停止后 `make prod daemon` 重新拉起。

说明：重启必须显式授权，不能把"顺手跑一下"当成重启授权。

## 隔离验证与验收通道

SG 没有独立的 alpha 镜像环境。验收遵循「先隔离、再合入、后验证」：

- **未合入改动的自测**：在 feature worktree 上 `make dev`（或 `make dev daemon`）启动本地服务自测，不污染主仓库运行态。
- **已合入 main 的验收**：跑 `make qc AUTHOR=<breed>` 触发 7 步 QC 跨模型 review 闸门（`internal/sop/qc_loop.go`），并由 CI `pre-merge` job（`scripts/pre-merge-check.sh`）把关，不依赖开发狗狗提供环境。
- **PR merge 后验收**：确认合入 main 的改动在完整门禁（Go `build/vet/test` + 前端 `tsc` / `vite build`）下工作正常。

**注意**：已合入改动的验收用 CI / `make qc`，不得用本地 runtime 当验收靶子。

### 验收盲区清单（pitfall 真相源：`packs/default/sop/development.yaml`）

以下盲区来自真实事故复盘（2026-08-23，详见 `docs/features/FT-UPG-001-version-upgrade.md`），命中对应改动类型时必须补验收：

| 盲区 | 何时命中 | 最低验收动作 | yaml pitfall id |
|---|---|---|---|
| **清零状态** | 涉及持久化数据/配置存储的改动 | `make clean deep` 后首屏 + 逐面板冒烟（空配置是独立测试场景，不是"有数据"状态的子集） | `quality-gate-fresh-workspace-smoke` |
| **产物断言** | 构建产物含关键内容（SW、注入清单等） | 断言产物内容而非 exit code；前端类型检查用 `tsc --noEmit -p <project>`（solution 型根 tsconfig 让裸 `tsc --noEmit` 空转通过） | `quality-gate-artifact-assertion` |
| **存活客户端** | 升级/部署/缓存策略类改动 | 验收"升级前已打开的页面 tab"在升级期间及之后可用或能自愈，而非仅"构建成功+服务重启" | `quality-gate-live-client-upgrade` |
| **中间件包装接口** | 新增/修改包装 `ResponseWriter` 的中间件 | 必须委托 `http.Hijacker`（WS）与 `http.Flusher`（SSE）——telemetry `statusRecorder` 曾只补 Hijack 漏 Flusher，导致所有 SSE 端点 500 | （并入产物/验收审查项） |

## Risk-Routed Development：铁路改立交

**强制力跟着风险走，不跟着动作类型走。** "写了代码""开了 PR""进入 merge"都不能单独触发整条流水线。默认是最小安全动作；只有命中客观风险面才进入对应加严车道。

### 入口：五轴风险判断

| 风险轴 | 命中信号 | 最低动作 |
|---|---|---|
| 行为面 | 用户可见行为、runtime 逻辑、bug 回归 | 可观察 RED + targeted 验证；方向未定才进 Design Gate |
| 数据 | 生产数据、迁移、持久化语义 | full gate + 独立高风险 review；生产操作另走授权边界 |
| 安全 | auth、权限、secret、注入、DoS / 资源边界 | full gate + context-blind 安全扫描；需要家里语义时再叠 local |
| 契约 | API / MCP schema / 事件格式 / 外部依赖 | 契约测试 + full gate + 对应独立 review |
| 不可逆 | 删除、force push、合第三方 PR、close feat、圣域 | 先拿 operator 授权；机器门禁仍照常 |

**元风险强制升档**：diff 触碰 `merge-gate`、门禁脚本自身时，直接进入 high-assurance，由非作者跨族 reviewer 覆盖最终实质内容（exact HEAD 或 continuityProof）。松绑机制不得静默松绑自己；在这条语义边界机器化前，如实标为 manual 守卫，不能由改门者自判 light。

五轴都未命中且改动可逆、无外部副作用 → 最小安全动作。信息不足不等于自动全套：先补查缺失事实，再按真实风险选车道。

### 按需车道

`packs/default/sop/development.yaml` 的 stage id 是告示牌车道，不是必须按顺序经过的状态机。选中车道后再加载对应 skill：

| 车道 | 触发条件 | 不因什么触发 |
|---|---|---|
| Design Gate / kickoff | 新 feature、UX / 架构方向未定、价值取舍 | 每个实现任务 |
| `writing-plans` | 跨组件、状态对象、实现顺序不清，且没有详细计划 | 文件超过 5 行 |
| `worktree` | tracked code、skill、SOP definition、脚本或第一方执行面需要隔离 | 纯 docs 已自判 light |
| `tdd` | 新行为、bug、未被现有精确检查覆盖的逻辑 | 确定性生成物刷新；现有 checker 红已经是 RED |
| targeted self-check | 所有交付；命令按风险面选 | 为了"报告完整"跑无关全仓测试 |
| `fresh-context` | author 判断当前上下文盲点高 | 非 trivial 就自动触发 |
| local peer | 家里语境、治理 / skill / SOP、实现语义 | 已选择独立外部源仍固定叠一层 |
| 独立外部 review | 安全、数据、契约或陌生跨包代码需要 context-blind 扫描 | 普通 `internal/**` / `web/**` / test / PR 载体本身 |
| merge-gate | PR / branch policy 需要合入；验证深度消费前述风险判断 | 自动重跑全套 |
| 愿景守护 | 用户可见或愿景变化的 feature close | 每个 PR、纯机械内部 change |

### Review 去叠加

默认选择**一个合适的独立验证源**，且必须是非作者：

- local peer 看家里语境与 stateful diff；
- 独立外部 review 看 context-blind 高风险代码面；
- 愿景守护看最终产品结果，只在 feature close 触发。

只有不同风险面确实需要不同视角时才叠加，并分别写明触发理由。P1/P2 修复后只回提出 finding 的 active source 覆盖真实修复 delta；不把另一个旧 reviewer 拉来续签，也不因 SHA-only / 可证明机械变化重开 reviewer。

### Sol 测试

任何存活的 `must` 都问：**一只完美遵从的狗狗 100% 执行后，系统是否仍然更好？** 不能稳定回答"是"的条款应删除或降为建议。终态不是狗狗学会打折，而是规则配得上全额遵从。安全、授权、真实性与不可逆结果边界优先交给机器 / 权限系统守，不靠把文字写凶。

## 约定面改动预检

改 MCP tool、skill manifest、route、workflow callback 等约定面前，先用代码检索确认影响面，避免只靠 grep 漏掉注册链或动态消费方。检索结果可能过期时先重新检索，不拿过期结果当真。

```bash
# 用代码检索确认注册链与动态消费方（灵缇/codex CLI 的 code search，或 grep）
# 关注：MCP tool 注册表、skill manifest、route 表、callback 接线
rg -n "<tool_name>" --type go --type ts
```

**自指字符串不走翻译**：语言名、品牌名、代码示例等"以自身身份出现的字符串"直接写死原生名（如语言切换器固定 `中文` / `English`），不经过 `t()`——翻译包里的译文会随 locale 变化，导致切换后 UI 显示错乱（pitfall id：`impl-i18n-self-referential-strings`）。同理，Go 侧序列化集合字段必须初始化为空切片（`[]string{}`），nil slice 会编成 `null` 击穿前端契约（契约见 `docs/reference/API.md` Response Conventions）。

## 例外路径

### Co-Creation Docs Lane（按风险，不按行数）

适用于 operator 与狗狗共同审阅、修订并授权落盘的 docs-only 内容，例如 architecture overview、思想纲领、discussion、研究笔记。只请 review 没有授权落盘时保持只读，不进入任何交付 lane。

显然满足"docs-only、无执行面、无已知并发冲突、单 commit 可逆"的轻量改动，由狗狗直接自判 `direct_push`；不要求先开 worktree 或扫全量 PR 来证明自己可以省流程。拿不准时，先收集完整 changed files（含 untracked）再判断。

`one_commit` 要同时满足：≤1 commit 可回滚、不影响外部用户/数据/契约。拿不准就按需升级到 PR 流程。已知有重叠在飞 PR 时再查具体 changed paths；无迹象时不为每份 Markdown 全量扫描 GitHub。

风险映射：

- conflict detected/unknown 或 reversibility high/unknown → PR。
- `docs/SOP.md`、VISION、decisions/canon 等治理文档 → PR + 本地跨族治理 review；纯 docs 不跑 full gate。
- `docs/ROADMAP.md` 是 main-only 共享状态，不是治理 PR 触发器：与 feature docs 同改仍走 direct main；若同批其他文件确需 PR，先把 BACKLOG 的机械登记单独落 main，禁止把它塞进 worktree/PR。
- 普通 `docs/features/*.md` 内容更新不因目录名自动升级；无重叠且单 commit 可逆时 direct push，真实冲突或高/未知可逆性仍升到 PR。
- `packs/default/skills/**`、`packs/default/sop/**`、scripts、CLI、tests、`internal/**` / `web/**` 或其他第一方执行面 → regular development，即使文件扩展名是 `.md`。
- 普通代码 / test 不因文件类型自动加严；行为面用 targeted tests + 合适独立源。安全、数据、外部契约或不可逆风险才升 full。

direct-push 只做：轻量增量校验 → 判断是否出现**需要第二只狗狗判断的新内容** → targeted commit（Why + 模型签名）→ push `origin main`。机械登记、拼写、operator 已逐字共创或有可回链旧 verdict 的内容可 `skip/reuse`，不为"有 diff"新叫 reviewer。普通文档校验不安装依赖、不构建共享包；feature 文档只追加 dependency-free feature truth。不建 worktree/PR，不生成 review 归档来证明自己 review 过。

**行数不是路由条件。** 不能用"超过 5 行"把安全共创文档机械送入完整开发链，也不能用"小 diff"掩盖敏感面。

### 最小安全 direct-main

仅在既有 lane 明确允许 direct push 时使用：co-creation docs 已自判显然 light，或确定性机械生成物的修复载体明确允许。必须同时满足可逆、无行为变化、无数据 / 安全 / 契约 / 不可逆面，并有精确机器检查；只有出现需要独立判断的新内容时才加非作者验证。

代码与第一方执行面仍用 PR；pure docs 先走 Co-Creation Docs Lane。diff 行数不参与判断。

## Reviewer 配对规则

动态匹配自运行时狗狗配置（`packs/default/breeds/dog-template.json` + `.sounds-great-ai/dog-catalog.json` overlay）：
1. 跨 family 优先 | 2. 必须有 peer-reviewer 角色 | 3. 必须 available
4. 优先 lead | 5. 优先活跃狗狗

**降级**：无跨 family reviewer → 同 family 不同个体 → operator。
**铁律**：同一个体不能 review 自己的代码。
**共享 GitHub 账号澄清**：全家共用 `sounds-great-ai` 账号，"个体"判据 = dogId（bianmu / jinmao / xigou 等），不看 GitHub login。GitHub `dismiss_stale_reviews_on_push` 因共享账号视所有狗狗为同一 pusher → `mergeStateStatus=BLOCKED`；此时 `--admin --match-head-commit` 是合规 fast-path，**不是 self-review violation**，无需纠结或升级 operator。

## 代码质量工具

| 工具 | 命令 | 何时 |
|------|------|------|
| Go 构建 / vet / test | `go build ./...` · `go vet ./...` · `go test ./...`（或 `make` 调用的 `scripts/pre-merge-check.sh`） | 每次提交前 + Step ② |
| 前端类型检查 | `cd web && npx tsc --noEmit`（或 `tsc -b`） | Step ② 必跑 |
| 前端 lint | `cd web && npm run lint`（oxlint） | 开发中 + Step ② |
| 前端构建 | `cd web && npm run build`（vite build） | 发布前 |

## 环境变量注册（必读！）

新增 `SOUNDS_GREAT_AI_*` 环境变量 → **必须在配置加载处登记并写入文档**（见 AGENTS.md 铁律 3：配置不可变，运行时不得改配置）。
前端「环境 & 文件」页面自动展示，不登记 = operator 看不到 = 不存在。

## 文档规范

- `docs/` 下 `.md` 文件必须有 YAML frontmatter
- 完成后必须同步真相源（详见 `feat-lifecycle` skill）

## 开源社区 Issue 处理

开源仓 `sounds-great-ai` 的社区 issue 由狗狗 triage，**operator 决定是否立项**。

### 角色分工

| 角色 | 谁 | 做什么 |
|------|-----|--------|
| **Triage** | 任意狗狗（收到 @ 或主动巡查） | 给 issue 加 `bug` / `feature` label，回复确认收到 |
| **编号分配** | operator 拍板 → 狗狗执行 | 在 ROADMAP.md 加条目，分配下一个可用编号 |
| **Feature Doc** | 分配到的狗狗 | 按模板写 `docs/features/FT-XXX-slug.md` |
| **实现** | 任意狗狗或社区贡献者 | 按 Feature Doc AC 实现 + PR |

### 流程

```
社区开 issue → 狗狗 triage（加 label）→ operator 拍板
    ├─ Feature → ROADMAP.md 加编号 → Feature Doc → 实现 → PR → review → merge 到 main
    └─ Bug fix → 分支修复 → PR → 跨狗狗 review → merge 到 main（见 AGENTS.md hotfix 规则）
```

### Hotfix Lane（Bug 快修通道）

社区报 bug 时，走单仓 hotfix lane（AGENTS.md 治理协议：hotfix PR 必须跨族或同族不同个体 review，不允许 self-merge）：

1. `git checkout -b fix/xxx main`
2. 在分支里修 bug，跑 `scripts/pre-merge-check.sh`（build/vet/test + 前端 tsc/vite）
3. 开 PR，跨狗狗 review 通过后 squash merge 到 main
4. close 关联 issue

> 详见 AGENTS.md 治理协议（per-family hotfix 止血）。

### 合并门禁（Pre-Merge Gate）

合入 main 的 PR 由 CI `pre-merge` job 把关（`.github/workflows/ci.yml`），跑 `scripts/pre-merge-check.sh --no-rebase`：`go build/vet/test ./...` + 前端 `tsc -b` / `vite build`。

硬规则：
1. 本地合入前先跑 `scripts/pre-merge-check.sh`（或 `make qc AUTHOR=<breed>` 触发跨模型 review）
2. **只有门禁全绿才允许 merge 到 main**
3. 本机 README/macOS smoke 是独立步骤，且必须显式隔离端口

一句话：**不要把真实 main 当第一轮验收场，更不能把本地 runtime 当验收靶子。**

### Release

SG 是单仓开源（`sounds-great-ai`），release = 打 tag + CI 门禁，无双仓同步。

硬规则：
1. release 前确保 main 上 CI（backend `build/vet/test` + frontend `tsc` + `pre-merge`）全绿
2. 按语义化版本打 tag：`git tag vX.Y.Z && git push origin vX.Y.Z`
3. 构建产物：`make prod`（Go 二进制 `bin/sounds-great-ai` + 前端 `web/dist`）

一句话：**release 不靠"记得当时是哪次 sync"，靠 tag + CI 全绿。**

### 规则

- **社区和内部共用一套编号**：不另起社区专属编号系列
- **编号唯一源**：ROADMAP.md（operator 拍板后狗狗执行分配）
- **Bug 不编号**：直接用 issue # 追踪，修完 close
- **贡献者不自选号**：CONTRIBUTING.md 已写明，狗狗回复时也要强调
- **分配编号前必须做关联检测**：确认 issue 不是现有 feature 的子项/增强
- **社区贡献者的 PR**：狗狗按 Feature Doc 引导（编号校验 + AC 对齐）

### Issue Label 命名规范

开源仓 `sounds-great-ai` 的 issue label 统一格式：

| Label | 格式 | 颜色 | 说明 |
|-------|------|------|------|
| Feature 关联 | `feature:FT-XXX` | `#0E8A16` 绿 | 关联到 Feature 编号 |
| Bug | `bug` | GitHub 默认 | 社区 bug report |
| Enhancement | `enhancement` | GitHub 默认 | 社区增强建议 |

**注意**：
- Feature label 必须用 `feature:FT-XXX` 格式（带 `feature:` 前缀 + FT-XXX 编号）
- Label 在仓库内定义规范，并写入 CONTRIBUTING.md
- 新建 label 时统一用绿色 `#0E8A16`
