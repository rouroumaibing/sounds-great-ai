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
- 不偏离 `docs/governance/decisions/irreversible-decisions.md` 的不可逆决策

## 完成

1. **quality-gate** — 自检通过
2. **review** — 非作者独立验证
3. **merge** — merge-gate 通过
4. **验收** — AC 逐条验证
5. **close** — 更新 feature 状态 + BACKLOG + 教训沉淀
