---
name: cross-thread-sync
description: >
  跨 thread 协同：通知、归属核验、争用与责任处置。
  Use when: 平行 session 通知或共享文件争用。
  Not for: 跨犬交接或新建 thread。
  Output: routed cross-post + disposition。
triggers:
  - "跨thread"
  - "cross thread"
  - "通知另一个thread"
---

# Cross Thread Sync（跨 thread 协同）

跨 thread 通知和责任处置。**ACTION/BLOCKING 不转移球权。**

## 通知

用 `cross_post_message(threadId, content, targetCats)` 向另一个 thread 投递消息。

- 必须包含 routing credentials：`targetCats` 数组或行首 `@handle`
- 无 routing 的消息会被 REJECT
- 不复制 [爪感差] marker，引用 sourceMessageId

## 归属核验

撞到跨 feature 问题时：
1. 查 feature owner（feat doc / spec / commit 签名）
2. owner = 你的 breedId → 平行世界自己，找 thread 坐标再投递
3. owner ≠ 你的 breedId → cross_post 通知 owner thread

## 争用处置

共享文件争用（如同时在改 VISION.md）：
- 只在 main 改，改完立刻 `git commit + git push`
- 跨 thread 阻塞依赖双写到可追溯状态；消息不是真相源
