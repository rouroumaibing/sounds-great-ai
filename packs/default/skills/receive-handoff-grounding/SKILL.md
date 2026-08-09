---
name: receive-handoff-grounding
description: >
  接球前真相核验三问：claim → resolver → verdict。
  Use when: 即将调 hold_ball / register tracking / merge / takeover / 任何 irreversible action 之前。
  Not for: 纯阅读 cross_post；本 thread 日常 @mention 无副作用。
  Output: claim grounding verdict (verified/mismatch/insufficient) + 接球决策。
triggers:
  - "接球"
  - "grounding"
  - "verify claim"
  - "接手"
---

# Receive Handoff Grounding（接球前真相核验）

防止把传球者当无审视真相源。接球前先核验三问。

## 三问流程

### 问 1: Claim 来源可信吗？

| Source Tier | 定义 | 信任度 |
|-------------|------|--------|
| T0 | 一手代码/commit/测试 | 高 |
| T1 | 官方文档/spec/ADR | 高 |
| T2 | 推理/传闻/二手 | 低 |

claim 来自 T2 → 必须查证后再行动。

### 问 2: Resolver 查证了吗？

- claim 引用的文件/commit/PR 存在吗？→ Read 原文
- claim 的摘要与原文一致吗？→ 对比
- claim 的时效性？→ 是否已被 supersede

### 问 3: Verdict 判定了吗？

| Verdict | 含义 | 接球决策 |
|---------|------|----------|
| **verified** | claim 与原文一致，来源可信 | proceed |
| **mismatch** | claim 与原文不一致 | block + push back to source |
| **insufficient** | 无法查证（文件不存在/权限不足） | block + 请求补充证据 |

## 何时必须做 grounding

- 即将调 hold_ball
- 即将 register_pr_tracking / register_issue_tracking
- 即将 merge
- 即将 takeover 别人的任务
- 任何 irreversible action
- 基于 "operator signoff" 或 "你是 owner" 类 claim 行动之前

## 何时可跳过

- 纯阅读 cross_post（无 actionFamily 后续）
- 本 thread 日常 @mention 无副作用
- implementation continuation（自检通过的下一步）
