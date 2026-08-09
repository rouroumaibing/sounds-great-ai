---
name: cross-cat-handoff
description: >
  跨犬交接与 review 双路由。
  Use when: 交接、exact-HEAD external PR review task 或 PR tracking。
  Not for: 自己任务。
  Output: 五件套 + formal/advisory 分类 + provenance 路由。
triggers:
  - "交接"
  - "handoff"
  - "跨犬"
  - "交给"
---

# Cross-Cat Handoff（跨犬交接）

把球传给下一只犬。交接不是通知，是带着上下文的球权转移。

## 五件套

交接时写全五件套，让接球的犬一眼看清：

| 件 | 内容 | 必填 |
|----|------|7------|
| **done** | 这个 session 做完了什么 | 是 |
| **nextSteps** | 从哪里继续、第一步具体做什么 | 是 |
| **worktreeBranch** | 当前 worktree 路径或分支名 | 否 |
| **commits** | 相关 commit SHA 列表 | 否 |
| **gotchas** | 最容易踩的坑、不可逆点、待验证假设 | 否 |

## Formal vs Advisory 分类

| 类型 | 触发条件 | 路由 |
|------|----------|------|
| **formal** | exact-HEAD external PR review task | PR tracking + review verdict |
| **advisory** | 本地协作、信息共享 | @mention + cross-post |

## Provenance 路由

接球前先做 grounding 三问（见 `receive-handoff-grounding` skill）：
1. claim 来源可信吗？
2. resolver 查证了吗？
3. verdict 判定了吗？

## 交接协议

1. **写五件套** — done + nextSteps 必填
2. **选路由** — @句柄（行首独立一行）或 cross_post_message
3. **声明球权** — 交接后球权归接球方
4. **不等回复** — 交接完成，不 hold 等回复

## 跨 thread 协作

撞 cross-feature 问题且 owner = 你的 `breedId`（平行世界自己）时：
- 不用本 thread `@句柄` 假装路由
- 先找 thread 坐标，再 `cross_post_message(threadId, targetCats, content)` 投递
