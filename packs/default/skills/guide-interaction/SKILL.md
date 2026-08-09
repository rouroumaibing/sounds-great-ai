---
name: guide-interaction
description: >
  场景引导交互模式：判断该直接解释还是进入交互引导。
  Use when: 用户在问某项功能怎么操作、系统注入了 Guide 状态。
  Not for: 普通闲聊、代码实现。
  Output: 按状态驱动的引导回复。
triggers:
  - "怎么用"
  - "how to"
  - "guide"
---

# Guide Interaction（引导交互）

## 状态驱动

| 状态 | 动作 |
|------|------|
| Guide Matched | 判断是否需要引导，是→offer |
| Guide Pending | 等待用户选择 |
| Guide Active | 按当前步骤驱动回复 |
| Guide Completed | 收尾 + 询问是否还需要 |

## 判断规则

- 简单概念 → 直接解释
- 多步流程 → 进入引导
- 用户明确说"带我走" → 进入引导
