---
name: tdd
description: >
  Red-Green-Refactor for changes with behavior or regression risk.
  Use when: adding observable behavior, fixing a bug, or changing logic.
  Not for: pure docs/research, deterministic generated-artifact refreshes.
  Output: observed RED → minimal GREEN → refactor under protection.
triggers:
  - "写代码"
  - "test first"
  - "TDD"
  - "红绿重构"
---

# TDD（测试驱动开发）

行为变化先看到一个可信的失败信号，再写最少实现把它变绿。**RED 是证据状态，不是必须新建测试文件。**

## 风险入口

| 改动 | 怎么走 |
|---|---|
| 新增用户可见 / runtime 行为 | 先写能失败的行为测试，再实现 |
| 修 bug，现有测试没有复现 | 先补回归测试，确认按正确原因失败 |
| 现有精确检查已经红 | 这就是现成 RED；直接修到该检查绿 |
| 纯文档、调研、无行为的机械改动 | 不触发本 skill |

## Red-Green-Refactor

1. **RED** — 写测试或找到现有检查，确认它以预期原因失败
2. **GREEN** — 写最少实现让测试通过
3. **REFACTOR** — 在测试保护下重构

禁止为了仪式感叠加第二个等价 RED。
