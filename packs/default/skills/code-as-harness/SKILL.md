---
name: code-as-harness
description: >
  证据确认重复摩擦后修 harness。
  Use: 历史重复已确认。
  Not: 未确认重复、首次 bug、review 反馈。
  Output: 未确认不强制 block；需 operator 决策发 interactive，否则行动后发 card。
triggers:
  - "重复摩擦"
  - "harness 修复"
  - "又出现了"
---

# Code as Harness（修 harness 不是修 prompt）

历史重复已确认的摩擦 → 修 harness（代码/配置/工具），不修 prompt。

## 流程

1. **确认重复** — 同一摩擦出现 ≥2 次，有证据
2. **根因分析** — 是 harness 缺陷还是 agent 行为问题
3. **修 harness** — 代码守护：改工具/配置/检查逻辑，不改 prompt
4. **验证** — 修复后同一摩擦不再出现

## 不做的事

- 未确认重复 → 不强制 block，先记录
- 首次 bug → 用 debugging skill
- review 反馈 → 用 receive-review skill
- 修 prompt 绕过 harness 缺陷 → 禁止
