---
name: feat-lifecycle
description: >
  Feature 立项、讨论、完成的全生命周期管理。
  Use when: 开个新功能、new feature、立项、feature 完成、验收通过。
  Not for: 代码实现、review、merge。
  Output: Feature 聚合文件 + BACKLOG 索引 + 真相源同步。
triggers:
  - "新功能"
  - "立项"
  - "feature 完成"
  - "feat lifecycle"
---

# Feature Lifecycle（Feature 全生命周期）

## 立项

1. **愿景对照** — 与 VISION.md §0 兼容？
2. **创建 feature 文件** — `docs/features/F<XXX>-<slug>.md`
3. **BACKLOG 索引** — 加入 BACKLOG
4. **spec 流程** — 大 feature 走 spec，小 feature 直接做

## 讨论

- 讨论记录在 feature 文件中
- 决策有 ADR 对应
- 不偏离不可逆决策

## Design Gate（设计确认）🔴

**Discussion → 动手实现之间的必经关卡。UX 没确认，不准开 worktree。User Journey 没落盘，不准过 Design Gate。**

按功能类型分流确认：

| 类型 | 判断标准 | 确认人 | 方式 |
|------|---------|--------|------|
| **前端 UI/UX** | 用户能看到的改动 | **operator** | wireframe → operator OK 后继续 |
| **纯后端** | API/数据模型/内部逻辑 | **其他狗狗** | 跨犬讨论达成共识（讨论记录落 feature 文件） |
| **架构级** | 跨模块、新基础设施、触碰不可逆决策 | **狗狗讨论 → operator 拍板** | 先出方案再上报，附决策矩阵 |
| **Trivial** | ≤5 行、纯重构、文档 | 跳过 | 跳过 Design Gate，按 SOP 例外路径判断 |

**前置检查（开 Gate 前先侦查，避免重复造轮子）**：
1. 读 `docs/features/README.md` 找相关 Feature
2. 读相关 Feature 的 Key Decisions / Open Questions
3. 搜历史讨论（evidence 存储 / feature 文件内讨论段）
4. 把发现记录到 Design Gate 讨论里

**User Journey 前置门禁**：涉及用户可感知变化的 Feature，Design Gate 前 spec 的 User Journey 必须已落盘（operator 口述期望回填，讨论落盘 ≠ spec 记录），否则不放行。非用户可感知 → 显式写 `user_journey_exempt: {reason}`。

**不可逆决策对照一问**：每个非 trivial Feature 在 Design Gate 必须能回答：
- 触碰了哪些不可逆决策（AGENTS.md 铁律：不碰 `internal/config/` 推理、不硬编码 DAG、不新建 A2A server、配置不可运行时改）？
- 需要新增/修改决策文档（`docs/plans/` 下 ADR 或 VISION.md）吗？`none | update required | new decision required`
- 答不出来或想绕开已有边界 → Design Gate 不放行。

**OQ 升级规则**：先判断可逆性——回滚成本低 + 不碰愿景/安全/外部契约/显著成本 → 狗狗自决，不升级 operator；需要升级时用决策矩阵格式（现状/方案/取舍/推荐），不能只列模糊问题清单。

**元审美自检**（Design Gate 必问）：这个方案是**坐标变换**（改变问题结构，让复杂度消失）还是**多项式堆项**（在现有结构上叠补丁/脚手架）？后者 → 先找更简的分解方式；审计不过 → 回到讨论或重新设计。

**流程**：
1. 判断功能类型 → 选择确认路径
2. 前端：画 wireframe（Pencil / 文字版 ASCII）→ 发 operator → 等 OK
3. 后端：跨犬讨论 API 契约/数据模型，结论写回 feature 文件
4. 架构：狗狗讨论 → 结论给 operator → 必须附决策矩阵 → operator 拍板
5. 确认产出归档（讨论落 feature 文件 + 更新 BACKLOG 索引）

## 完成

1. **quality-gate** — 自检通过
2. **review** — 非作者独立验证
3. **merge** — merge-gate 通过
4. **验收** — AC 逐条验证
5. **close** — 更新 feature 状态 + BACKLOG + 教训沉淀
