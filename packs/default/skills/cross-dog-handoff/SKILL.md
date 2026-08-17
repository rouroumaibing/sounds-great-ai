---
name: cross-dog-handoff
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

# Cross-Dog Handoff（跨犬交接）

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
- 先找 thread 坐标，再 `cross_post_message(threadId, targetDogs, content)` 投递

## Review 交接的硬约束

把工作交给另一只犬做 review 时，球权转移必须满足跨模型 review 不变量：

- ** reviewer `dog_id` 必须 ≠ author `dog_id`**。同 `dog_id` 自我 review 在交接处被 fail-closed 拒绝，不进入 review 流程。
- **首次跨 dog_id 交接 = review 指派**：系统记录「谁审谁、哪个 thread 发起」。后续该 thread 把工作交回作者，即 review 写回。
- **写回必须回到直连 review carrier**：只有被指派的 reviewer 可写回 verdict，且必须回到发起 review 的那个 thread（direct review carrier），不是祖先 thread 或误投落点。非指派方写回、或写回错 thread，均被 fail-closed 拒绝。
- 这一约束在交接（handoff）与写回（verdict）两端都强制，而不是靠约定。
